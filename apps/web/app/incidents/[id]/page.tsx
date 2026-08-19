"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { ackIncident, commentIncident, getIncident, resolveIncident, type Incident } from "@/lib/api";

export default function IncidentPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const [inc, setInc] = useState<Incident | null>(null);
  const [comment, setComment] = useState("");
  const [err, setErr] = useState("");

  const reload = useCallback(() => {
    getIncident(id)
      .then(setInc)
      .catch((e) => setErr(String(e)));
  }, [id]);

  useEffect(() => {
    reload();
  }, [reload]);

  if (!inc) {
    return (
      <main>
        <p>{err || "読み込み中…"}</p>
        <p>
          <a href="/">ボードへ</a>
        </p>
      </main>
    );
  }

  return (
    <main>
      <p>
        <a href="/">ボードへ</a>
      </p>
      <h1>{inc.summary}</h1>
      <p>
        {inc.severity} · {inc.status} · {inc.dedupKey} · assignee {inc.assigneeSub || "—"}
      </p>
      {err ? <p>{err}</p> : null}
      <p>
        <button type="button" onClick={() => ackIncident(inc.id).then(reload).catch((e) => setErr(String(e)))}>
          Ack
        </button>{" "}
        <button type="button" onClick={() => resolveIncident(inc.id).then(reload).catch((e) => setErr(String(e)))}>
          Resolve
        </button>
      </p>
      <h2>タイムライン</h2>
      <ol>
        {(inc.timeline ?? []).map((ev) => (
          <li key={ev.id}>
            <strong>{ev.kind}</strong> {ev.message} <span style={{ color: "#666" }}>({ev.actor} · {ev.at})</span>
          </li>
        ))}
      </ol>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          commentIncident(inc.id, comment)
            .then(() => {
              setComment("");
              reload();
            })
            .catch((err) => setErr(String(err)));
        }}
      >
        <label>
          コメント
          <br />
          <input value={comment} onChange={(e) => setComment(e.target.value)} style={{ width: "24rem" }} />
        </label>{" "}
        <button type="submit">追加</button>
      </form>
    </main>
  );
}
