"use client";

import { useCallback, useEffect, useState } from "react";
import { fireDemoAlert, listIncidents, listServices, virtualMetrics, type Incident, type Service, type VirtualMetrics } from "@/lib/api";

function sevColor(sev: string) {
  switch (sev) {
    case "SEV1":
      return "#b91c1c";
    case "SEV2":
      return "#c2410c";
    case "SEV3":
      return "#a16207";
    default:
      return "#4b5563";
  }
}

export default function HomePage() {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [services, setServices] = useState<Service[]>([]);
  const [metrics, setMetrics] = useState<VirtualMetrics | null>(null);
  const [err, setErr] = useState("");

  const reload = useCallback(() => {
    Promise.all([listIncidents(), listServices(), virtualMetrics()])
      .then(([i, s, m]) => {
        setIncidents(i);
        setServices(s);
        setMetrics(m);
        setErr("");
      })
      .catch((e) => setErr(String(e)));
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const svcName = (id: string) => services.find((s) => s.id === id)?.name ?? id;

  return (
    <main>
      <h1>インシデントボード</h1>
      <p style={{ color: "#555" }}>未解決が上。Webhook は HMAC + 同一 dedup_key で 1 件に集約します。</p>
      {err ? <p>{err}</p> : null}
      <p>
        <button type="button" onClick={() => fireDemoAlert().then(reload).catch((e) => setErr(String(e)))}>
          擬似アラート（inventory 5xx）
        </button>{" "}
        <button type="button" onClick={reload}>
          再読込
        </button>
      </p>
      <ul style={{ listStyle: "none", padding: 0, display: "grid", gap: "0.75rem" }}>
        {incidents.map((inc) => (
          <li key={inc.id} style={{ border: "1px solid #ddd", padding: "0.85rem 1rem", borderRadius: 8 }}>
            <a href={`/incidents/${inc.id}`} style={{ color: "inherit", textDecoration: "none" }}>
              <strong style={{ color: sevColor(inc.severity) }}>{inc.severity}</strong>{" "}
              <span>{inc.status}</span>
              <div>{inc.summary}</div>
              <div style={{ color: "#666", fontSize: "0.9rem" }}>
                {svcName(inc.serviceId)} · {inc.dedupKey} · alerts {inc.alertCount}
              </div>
            </a>
          </li>
        ))}
      </ul>
      {incidents.length === 0 ? <p>インシデントはありません。</p> : null}

      <h2>サービス</h2>
      <ul>
        {services.map((s) => (
          <li key={s.id}>
            {s.name} ({s.code})
            {s.integrations.map((k) => (
              <span key={k.publicKey} style={{ marginLeft: "0.5rem", color: "#666" }}>
                key {k.publicKey} secret {k.secretMasked}
              </span>
            ))}
          </li>
        ))}
      </ul>

      <h2>仮想メトリクス（bad-deploy）</h2>
      {metrics ? (
        <div>
          <p>
            state={metrics.state} · virtualOnly={String(metrics.virtualOnly)}
          </p>
          <p style={{ color: "#555" }}>{metrics.note}</p>
          <ul>
            {metrics.points.map((p) => (
              <li key={p.name}>
                {p.name}: {p.value} {p.unit}
              </li>
            ))}
          </ul>
          <p>
            <button type="button" onClick={() => virtualMetrics("scale").then(setMetrics)}>
              仮想 scale
            </button>{" "}
            <button type="button" onClick={() => virtualMetrics("rollback").then(setMetrics)}>
              仮想 rollback
            </button>
          </p>
        </div>
      ) : null}
    </main>
  );
}
