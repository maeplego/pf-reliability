"use client";

import { useState } from "react";
import { scoreTraining, type TrainingScore } from "@/lib/api";

const ACTIONS = ["observe", "scale", "rollback", "escalate", "declare_resolved"] as const;

export default function TrainingPage() {
  const [queued, setQueued] = useState<string[]>([]);
  const [result, setResult] = useState<TrainingScore | null>(null);
  const [err, setErr] = useState("");

  async function apply(next: string[]) {
    setQueued(next);
    if (next.length === 0) {
      setResult(null);
      return;
    }
    try {
      setResult(await scoreTraining(next));
      setErr("");
    } catch (e) {
      setErr(String(e));
    }
  }

  return (
    <main>
      <h1>訓練（bad-deploy）</h1>
      <p>本番クラスタは操作しません。scale では直らず、rollback で仮想状態が recovered になります。</p>
      {err ? <p>{err}</p> : null}
      <p>
        {ACTIONS.map((a) => (
          <button key={a} type="button" style={{ marginRight: 8 }} onClick={() => void apply([...queued, a])}>
            {a}
          </button>
        ))}
      </p>
      <p>操作列: {queued.length ? queued.join(" → ") : "（まだなし）"}</p>
      <p>
        <button type="button" onClick={() => void apply([])}>
          リセット
        </button>
      </p>
      {result ? (
        <section>
          <h2>
            {result.passed ? "合格" : "未回復 / 不合格"} · {result.score} 点 · {result.finalState}
          </h2>
          <p>virtualOnly={String(result.virtualOnly)}</p>
          {result.penalties.length ? (
            <ul>
              {result.penalties.map((p) => (
                <li key={p}>{p}</li>
              ))}
            </ul>
          ) : null}
        </section>
      ) : null}
    </main>
  );
}
