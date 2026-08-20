"use client";

import { useCallback, useEffect, useState } from "react";
import { fireDemoAlert, listIncidents, listServices, virtualMetrics, type Incident, type Service, type VirtualMetrics } from "@/lib/api";

function sevClass(sev: string) {
  switch (sev) {
    case "SEV1":
      return "sev-sev1";
    case "SEV2":
      return "sev-sev2";
    case "SEV3":
      return "sev-sev3";
    default:
      return "sev-default";
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
    <>
      <section className="hero">
        <h1 className="page-title">インシデントボード</h1>
        <p className="page-lead">未解決が上。Webhook は HMAC + 同一 dedup_key で 1 件に集約します。</p>
      </section>
      {err ? <p className="error">{err}</p> : null}
      <div className="row">
        <button
          type="button"
          className="btn"
          onClick={() => fireDemoAlert().then(reload).catch((e) => setErr(String(e)))}
        >
          擬似アラート（inventory 5xx）
        </button>
        <button type="button" className="btn btn-secondary" onClick={reload}>
          再読込
        </button>
      </div>
      <div className="card-grid">
        {incidents.map((inc) => (
          <article key={inc.id} className="card">
            <a href={`/incidents/${inc.id}`} className="incident-link">
              <strong className={sevClass(inc.severity)}>{inc.severity}</strong>{" "}
              <span className="badge">{inc.status}</span>
              <div>{inc.summary}</div>
              <div className="muted">
                {svcName(inc.serviceId)} · {inc.dedupKey} · alerts {inc.alertCount}
              </div>
            </a>
          </article>
        ))}
      </div>
      {incidents.length === 0 ? <p className="muted">インシデントはありません。</p> : null}

      <h2 className="section-title">サービス</h2>
      <div className="card-grid">
        {services.map((s) => (
          <article key={s.id} className="card">
            <strong>{s.name}</strong> <span className="muted">({s.code})</span>
            {s.integrations.map((k) => (
              <div key={k.publicKey} className="muted">
                key {k.publicKey} secret {k.secretMasked}
              </div>
            ))}
          </article>
        ))}
      </div>

      <h2 className="section-title">仮想メトリクス（bad-deploy）</h2>
      {metrics ? (
        <section className="card">
          <p>
            state={metrics.state} · virtualOnly={String(metrics.virtualOnly)}
          </p>
          <p className="muted">{metrics.note}</p>
          <ul>
            {metrics.points.map((p) => (
              <li key={p.name}>
                {p.name}: {p.value} {p.unit}
              </li>
            ))}
          </ul>
          <div className="row">
            <button type="button" className="btn" onClick={() => virtualMetrics("scale").then(setMetrics)}>
              仮想 scale
            </button>
            <button type="button" className="btn btn-secondary" onClick={() => virtualMetrics("rollback").then(setMetrics)}>
              仮想 rollback
            </button>
          </div>
        </section>
      ) : null}
    </>
  );
}
