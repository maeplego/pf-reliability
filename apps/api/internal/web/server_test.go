package web_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/portfolio/pf-reliability/apps/api/internal/auth"
	"github.com/portfolio/pf-reliability/apps/api/internal/clock"
	"github.com/portfolio/pf-reliability/apps/api/internal/incident"
	"github.com/portfolio/pf-reliability/apps/api/internal/seed"
	"github.com/portfolio/pf-reliability/apps/api/internal/store/memory"
	"github.com/portfolio/pf-reliability/apps/api/internal/web"
	"github.com/portfolio/pf-reliability/apps/api/internal/webhook"
)

const testSecret = "dev-webhook-secret-not-for-prod"
const testKey = "dev-inventory"

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	st := memory.New()
	clk := &clock.Fixed{T: time.Date(2026, 8, 19, 5, 0, 0, 0, time.UTC)}
	svc := incident.NewService(st, clk.Now)
	if err := seed.Ensure(ctx, svc, testKey, testSecret); err != nil {
		t.Fatal(err)
	}
	mw := auth.New(true)
	srv := web.New(svc, "", mw, nil, testKey)
	return httptest.NewServer(srv.Routes())
}

func sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(testSecret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func decode(t *testing.T, res *http.Response, dest any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
		t.Fatal(err)
	}
}

func postJSON(t *testing.T, method, url string, body any, sub string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if sub != "" {
		req.Header.Set("X-Dev-User-Sub", sub)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestHealthReady(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	for _, path := range []string{"/health", "/ready"} {
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 {
			t.Fatalf("%s %d", path, res.StatusCode)
		}
		_ = res.Body.Close()
	}
}

func TestWebhookMissingAndBadSignature(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	body := []byte(`{"dedup_key":"commerce-inventory-5xx","severity":"SEV2","service":"inventory","summary":"5xx ratio > 5%"}`)
	url := ts.URL + "/v1/integrations/" + testKey + "/events"

	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing sig %d", res.StatusCode)
	}
	_ = res.Body.Close()

	req2, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(webhook.HeaderSignature, "sha256="+strings.Repeat("00", 32))
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	if res2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad sig %d", res2.StatusCode)
	}
	_ = res2.Body.Close()
}

func TestWebhookDedupSameIncident(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	body := []byte(`{"dedup_key":"commerce-inventory-5xx","severity":"SEV2","service":"inventory","summary":"5xx ratio > 5%"}`)
	url := ts.URL + "/v1/integrations/" + testKey + "/events"

	post := func() *http.Response {
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(webhook.HeaderSignature, sign(body))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	res1 := post()
	if res1.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res1.Body)
		t.Fatalf("first %d %s", res1.StatusCode, b)
	}
	var a struct {
		ID         string `json:"id"`
		AlertCount int    `json:"alertCount"`
		Status     string `json:"status"`
	}
	decode(t, res1, &a)
	if a.Status != "triggered" || a.AlertCount != 1 {
		t.Fatalf("%+v", a)
	}

	res2 := post()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("second %d", res2.StatusCode)
	}
	var b struct {
		ID         string `json:"id"`
		AlertCount int    `json:"alertCount"`
	}
	decode(t, res2, &b)
	if b.ID != a.ID || b.AlertCount != 2 {
		t.Fatalf("dedup failed %+v vs %+v", a, b)
	}

	list, err := http.Get(ts.URL + "/v1/incidents")
	if err != nil {
		t.Fatal(err)
	}
	var wrap struct {
		Incidents []struct {
			ID string `json:"id"`
		} `json:"incidents"`
	}
	decode(t, list, &wrap)
	if len(wrap.Incidents) != 1 {
		t.Fatalf("want 1 incident, got %d", len(wrap.Incidents))
	}
}

func TestWebhookEventIDIdempotent(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	body := []byte(`{"dedup_key":"k1","severity":"SEV3","service":"inventory","summary":"once","event_id":"evt-1"}`)
	url := ts.URL + "/v1/integrations/" + testKey + "/events"
	post := func() *http.Response {
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set(webhook.HeaderSignature, sign(body))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res1 := post()
	var a struct {
		ID         string `json:"id"`
		AlertCount int    `json:"alertCount"`
	}
	decode(t, res1, &a)
	res2 := post()
	var b struct {
		ID         string `json:"id"`
		AlertCount int    `json:"alertCount"`
	}
	decode(t, res2, &b)
	if a.ID != b.ID || b.AlertCount != 1 {
		t.Fatalf("event_id replay should not bump count: %+v %+v", a, b)
	}
}

func TestAckResolveAndTimeline(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	svcs, err := http.Get(ts.URL + "/v1/services")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Services []struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"services"`
	}
	decode(t, svcs, &catalog)
	var checkoutID string
	for _, ms := range catalog.Services {
		if ms.Code == "checkout" {
			checkoutID = ms.ID
		}
	}
	if checkoutID == "" {
		t.Fatal("checkout missing")
	}
	res := postJSON(t, http.MethodPost, ts.URL+"/v1/incidents", map[string]any{
		"serviceId": checkoutID, "severity": "SEV1", "summary": "manual fire", "dedupKey": "manual-1",
	}, "sre-demo")
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var inc struct {
		ID string `json:"id"`
	}
	decode(t, res, &inc)

	ack := postJSON(t, http.MethodPost, ts.URL+"/v1/incidents/"+inc.ID+"/ack", nil, "sre-demo")
	if ack.StatusCode != 200 {
		t.Fatalf("ack %d", ack.StatusCode)
	}
	var afterAck struct {
		Status      string `json:"status"`
		AssigneeSub string `json:"assigneeSub"`
	}
	decode(t, ack, &afterAck)
	if afterAck.Status != "acknowledged" || afterAck.AssigneeSub != "sre-demo" {
		t.Fatalf("%+v", afterAck)
	}

	cmt := postJSON(t, http.MethodPost, ts.URL+"/v1/incidents/"+inc.ID+"/comments", map[string]any{"body": "checking virtual metrics"}, "sre-demo")
	if cmt.StatusCode != http.StatusCreated {
		t.Fatalf("comment %d", cmt.StatusCode)
	}
	_ = cmt.Body.Close()

	resv := postJSON(t, http.MethodPost, ts.URL+"/v1/incidents/"+inc.ID+"/resolve", nil, "sre-demo")
	if resv.StatusCode != 200 {
		t.Fatalf("resolve %d", resv.StatusCode)
	}
	_ = resv.Body.Close()

	got, err := http.Get(ts.URL + "/v1/incidents/" + inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	var detail struct {
		Status   string `json:"status"`
		Timeline []struct {
			Kind string `json:"kind"`
		} `json:"timeline"`
	}
	decode(t, got, &detail)
	if detail.Status != "resolved" {
		t.Fatalf("status %s", detail.Status)
	}
	kinds := map[string]int{}
	for _, ev := range detail.Timeline {
		kinds[ev.Kind]++
	}
	if kinds["opened"] != 1 || kinds["acknowledged"] != 1 || kinds["comment"] != 1 || kinds["resolved"] != 1 {
		t.Fatalf("timeline %+v", kinds)
	}

	ack2 := postJSON(t, http.MethodPost, ts.URL+"/v1/incidents/"+inc.ID+"/ack", nil, "sre-demo")
	defer ack2.Body.Close()
	if ack2.StatusCode != http.StatusConflict {
		t.Fatalf("ack resolved %d", ack2.StatusCode)
	}
}

func TestMutationsRequireAuth(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	res := postJSON(t, http.MethodPost, ts.URL+"/v1/incidents", map[string]any{
		"serviceId": "x", "severity": "SEV2", "summary": "nope",
	}, "")
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("%d", res.StatusCode)
	}
}

func TestVirtualMetricsStaySynthetic(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	res, err := http.Get(ts.URL + "/v1/virtual-metrics?after=scale")
	if err != nil {
		t.Fatal(err)
	}
	var snap struct {
		VirtualOnly bool   `json:"virtualOnly"`
		State       string `json:"state"`
		Points      []struct {
			Value float64 `json:"value"`
		} `json:"points"`
	}
	decode(t, res, &snap)
	if !snap.VirtualOnly || snap.State != "scaled" || snap.Points[0].Value < 0.1 {
		t.Fatalf("%+v", snap)
	}
}

func TestUnknownIntegration(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	body := []byte(`{"dedup_key":"x","severity":"SEV2","service":"inventory","summary":"x"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/integrations/no-such/events", bytes.NewReader(body))
	req.Header.Set(webhook.HeaderSignature, sign(body))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("%d", res.StatusCode)
	}
}

func TestResolvedDedupOpensNewIncident(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	fire := postJSON(t, http.MethodPost, ts.URL+"/v1/demo/alerts", nil, "sre-demo")
	var a struct {
		ID string `json:"id"`
	}
	decode(t, fire, &a)
	resv := postJSON(t, http.MethodPost, ts.URL+"/v1/incidents/"+a.ID+"/resolve", nil, "sre-demo")
	_ = resv.Body.Close()
	fire2 := postJSON(t, http.MethodPost, ts.URL+"/v1/demo/alerts", nil, "sre-demo")
	var b struct {
		ID string `json:"id"`
	}
	decode(t, fire2, &b)
	if a.ID == b.ID {
		t.Fatal("resolved incident should not absorb a new alert")
	}
}
