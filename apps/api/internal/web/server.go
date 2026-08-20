package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/portfolio/pf-reliability/apps/api/internal/auth"
	"github.com/portfolio/pf-reliability/apps/api/internal/incident"
	"github.com/portfolio/pf-reliability/apps/api/internal/runbook"
	"github.com/portfolio/pf-reliability/apps/api/internal/seed"
	"github.com/portfolio/pf-reliability/apps/api/internal/webhook"
	"github.com/portfolio/pf-reliability/packages/scenario"
)

type Server struct {
	incidents *incident.Service
	books     *runbook.Store
	history   *runbook.History
	cors      string
	auth      *auth.Middleware
	ready     func() error
	integKey  string
}

func New(incidents *incident.Service, cors string, mw *auth.Middleware, ready func() error, integKey string) *Server {
	if ready == nil {
		ready = func() error { return nil }
	}
	books := runbook.NewStore()
	books.Seed()
	return &Server{
		incidents: incidents,
		books:     books,
		history:   runbook.NewHistory(),
		cors:      cors,
		auth:      mw,
		ready:     ready,
		integKey:  integKey,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		if err := s.ready(); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /v1/services", s.listServices)
	mux.HandleFunc("GET /v1/services/{id}", s.getService)
	mux.HandleFunc("GET /v1/virtual-metrics", s.virtualMetrics)

	mux.HandleFunc("GET /v1/incidents", s.listIncidents)
	mux.HandleFunc("GET /v1/incidents/{id}", s.getIncident)
	mux.Handle("POST /v1/incidents", s.auth.Handler(http.HandlerFunc(s.createIncident)))
	mux.Handle("POST /v1/incidents/{id}/ack", s.auth.Handler(http.HandlerFunc(s.ackIncident)))
	mux.Handle("POST /v1/incidents/{id}/resolve", s.auth.Handler(http.HandlerFunc(s.resolveIncident)))
	mux.Handle("POST /v1/incidents/{id}/comments", s.auth.Handler(http.HandlerFunc(s.commentIncident)))
	mux.Handle("POST /v1/demo/alerts", s.auth.Handler(http.HandlerFunc(s.demoAlert)))

	mux.HandleFunc("POST /v1/integrations/{key}/events", s.ingestEvent)
	mux.HandleFunc("GET /v1/training/scenarios", s.listScenarios)
	mux.HandleFunc("POST /v1/training/score", s.scoreTraining)
	mux.HandleFunc("GET /v1/training/history", s.trainingHistory)
	mux.HandleFunc("GET /v1/runbooks", s.listRunbooks)
	mux.Handle("POST /v1/runbooks", s.auth.Handler(http.HandlerFunc(s.upsertRunbook)))
	mux.HandleFunc("GET /v1/runbooks/{id}", s.getRunbook)
	mux.Handle("PUT /v1/runbooks/{id}", s.auth.Handler(http.HandlerFunc(s.upsertRunbook)))
	mux.Handle("DELETE /v1/runbooks/{id}", s.auth.Handler(http.HandlerFunc(s.deleteRunbook)))
	mux.HandleFunc("GET /v1/oncall", s.oncall)
	return s.withCORS(mux)
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cors != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.cors)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Dev-User-Sub, X-Signature-256")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func incidentJSON(inc incident.Incident) map[string]any {
	return map[string]any{
		"id":             inc.ID,
		"serviceId":      inc.ServiceID,
		"dedupKey":       inc.DedupKey,
		"severity":       inc.Severity,
		"status":         inc.Status,
		"summary":        inc.Summary,
		"assigneeSub":    inc.AssigneeSub,
		"alertCount":     inc.AlertCount,
		"createdAt":      rfc3339(inc.CreatedAt),
		"updatedAt":      rfc3339(inc.UpdatedAt),
		"acknowledgedAt": rfc3339(inc.AcknowledgedAt),
		"resolvedAt":     rfc3339(inc.ResolvedAt),
	}
}

func eventJSON(ev incident.Event) map[string]any {
	return map[string]any{
		"id": ev.ID, "incidentId": ev.IncidentID, "kind": ev.Kind,
		"actor": ev.Actor, "message": ev.Message, "at": rfc3339(ev.At),
	}
}

func (s *Server) listServices(w http.ResponseWriter, r *http.Request) {
	list, err := s.incidents.ListServices(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, ms := range list {
		out = append(out, s.serviceJSON(r, ms))
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": out, "productionOps": false})
}

func (s *Server) getService(w http.ResponseWriter, r *http.Request) {
	ms, err := s.incidents.GetService(r.Context(), r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.serviceJSON(r, ms))
}

func (s *Server) serviceJSON(r *http.Request, ms incident.MonitoredService) map[string]any {
	ints, _ := s.incidents.ListIntegrations(r.Context(), ms.ID)
	keys := make([]map[string]any, 0, len(ints))
	for _, in := range ints {
		keys = append(keys, map[string]any{
			"id": in.ID, "publicKey": in.PublicKey, "secretMasked": incident.MaskSecret(in.Secret),
		})
	}
	return map[string]any{
		"id": ms.ID, "code": ms.Code, "name": ms.Name, "description": ms.Description,
		"createdAt": rfc3339(ms.CreatedAt), "integrations": keys,
		"virtualMetricsOnly": true,
	}
}

func (s *Server) virtualMetrics(w http.ResponseWriter, r *http.Request) {
	name := scenario.NormalizeName(r.URL.Query().Get("scenario"))
	action := strings.TrimSpace(r.URL.Query().Get("after"))
	if action == "" {
		writeJSON(w, http.StatusOK, scenario.MetricsNamed(name, scenario.StateDegraded))
		return
	}
	snap, err := scenario.AfterNamed(name, scenario.Action(action))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) listScenarios(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"scenarios": scenario.Names(), "virtualOnly": true})
}

func (s *Server) scoreTraining(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Scenario string   `json:"scenario"`
		Actions  []string `json:"actions"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	actions := make([]scenario.Action, 0, len(body.Actions))
	for _, raw := range body.Actions {
		actions = append(actions, scenario.Action(strings.TrimSpace(raw)))
	}
	result, err := scenario.ScoreNamed(body.Scenario, actions)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	s.history.Add(runbook.HistoryEntry{
		At: time.Now().UTC(), Scenario: result.Scenario, Score: result.Score, Passed: result.Passed,
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) trainingHistory(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"history": s.history.List(), "virtualOnly": true})
}

func (s *Server) listRunbooks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"runbooks": s.books.List()})
}

func (s *Server) getRunbook(w http.ResponseWriter, r *http.Request) {
	b, err := s.books.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) upsertRunbook(w http.ResponseWriter, r *http.Request) {
	var body runbook.Book
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	if id := strings.TrimSpace(r.PathValue("id")); id != "" {
		body.ID = id
	}
	if err := s.books.Upsert(body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) deleteRunbook(w http.ResponseWriter, r *http.Request) {
	if err := s.books.Delete(r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) oncall(w http.ResponseWriter, r *http.Request) {
	at := time.Now().UTC()
	if raw := strings.TrimSpace(r.URL.Query().Get("at")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", "at must be RFC3339")
			return
		}
		at = parsed
	}
	writeJSON(w, http.StatusOK, runbook.OnCallAt(at))
}

func (s *Server) listIncidents(w http.ResponseWriter, r *http.Request) {
	list, err := s.incidents.List(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, inc := range list {
		out = append(out, incidentJSON(inc))
	}
	writeJSON(w, http.StatusOK, map[string]any{"incidents": out})
}

func (s *Server) getIncident(w http.ResponseWriter, r *http.Request) {
	inc, evs, err := s.incidents.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	timeline := make([]map[string]any, 0, len(evs))
	for _, ev := range evs {
		timeline = append(timeline, eventJSON(ev))
	}
	body := incidentJSON(inc)
	body["timeline"] = timeline
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) createIncident(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	var body struct {
		ServiceID string `json:"serviceId"`
		DedupKey  string `json:"dedupKey"`
		Severity  string `json:"severity"`
		Summary   string `json:"summary"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	inc, created, err := s.incidents.Create(r.Context(), incident.CreateInput{
		ServiceID: body.ServiceID, DedupKey: body.DedupKey,
		Severity: body.Severity, Summary: body.Summary, Actor: u.Sub,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	st := http.StatusCreated
	if !created {
		st = http.StatusOK
	}
	writeJSON(w, st, incidentJSON(inc))
}

func (s *Server) ackIncident(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	inc, err := s.incidents.Ack(r.Context(), r.PathValue("id"), u.Sub)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, incidentJSON(inc))
}

func (s *Server) resolveIncident(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	inc, err := s.incidents.Resolve(r.Context(), r.PathValue("id"), u.Sub)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, incidentJSON(inc))
}

func (s *Server) commentIncident(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	ev, err := s.incidents.Comment(r.Context(), r.PathValue("id"), u.Sub, body.Body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, eventJSON(ev))
}

func (s *Server) demoAlert(w http.ResponseWriter, r *http.Request) {
	integ, err := s.incidents.GetIntegration(r.Context(), s.integKey)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	inc, created, err := s.incidents.Ingest(r.Context(), incident.IngestInput{
		Integration: integ,
		DedupKey:    "commerce-inventory-5xx",
		Severity:    "SEV2",
		ServiceCode: seed.InventoryCode,
		Summary:     "5xx ratio > 5%",
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	st := http.StatusCreated
	if !created {
		st = http.StatusOK
	}
	writeJSON(w, st, incidentJSON(inc))
}

func (s *Server) ingestEvent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, webhook.MaxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", "payload exceeds 64KiB")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid", "cannot read body")
		return
	}
	key := strings.TrimSpace(r.PathValue("key"))
	integ, err := s.incidents.GetIntegration(r.Context(), key)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := webhook.Verify(integ.Secret, raw, r.Header.Get(webhook.HeaderSignature)); err != nil {
		writeWebhookAuthError(w, err)
		return
	}
	var body struct {
		DedupKey string `json:"dedup_key"`
		Severity string `json:"severity"`
		Service  string `json:"service"`
		Summary  string `json:"summary"`
		EventID  string `json:"event_id"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	inc, created, err := s.incidents.Ingest(r.Context(), incident.IngestInput{
		Integration: integ,
		EventID:     body.EventID,
		DedupKey:    body.DedupKey,
		Severity:    body.Severity,
		ServiceCode: body.Service,
		Summary:     body.Summary,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	st := http.StatusCreated
	if !created {
		st = http.StatusOK
	}
	writeJSON(w, st, incidentJSON(inc))
}

func decodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dest)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func writeWebhookAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, webhook.ErrMissingSignature):
		writeError(w, http.StatusUnauthorized, "missing_signature", "X-Signature-256 required")
	default:
		writeError(w, http.StatusUnauthorized, "bad_signature", "invalid HMAC signature")
	}
}

func writeDomainError(w http.ResponseWriter, err error) {
	st := statusFor(err)
	code, msg := codeOf(err)
	writeError(w, st, code, msg)
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, incident.ErrNotFound), errors.Is(err, incident.ErrServiceNotFound), errors.Is(err, incident.ErrIntegrationMissing):
		return http.StatusNotFound
	case errors.Is(err, incident.ErrInvalidTransition), errors.Is(err, incident.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, incident.ErrInvalid):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func codeOf(err error) (string, string) {
	switch {
	case errors.Is(err, incident.ErrNotFound), errors.Is(err, incident.ErrServiceNotFound), errors.Is(err, incident.ErrIntegrationMissing):
		return "not_found", err.Error()
	case errors.Is(err, incident.ErrInvalidTransition):
		return "invalid_transition", err.Error()
	case errors.Is(err, incident.ErrConflict):
		return "conflict", err.Error()
	default:
		if err == nil {
			return "error", "error"
		}
		return "invalid", err.Error()
	}
}
