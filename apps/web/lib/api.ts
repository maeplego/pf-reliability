export const apiBase = process.env.NEXT_PUBLIC_RELIABILITY_API_URL ?? "http://localhost:8012";
export const actor = "sre-demo";

export type Service = {
  id: string;
  code: string;
  name: string;
  description: string;
  integrations: { publicKey: string; secretMasked: string }[];
};

export type Incident = {
  id: string;
  serviceId: string;
  dedupKey: string;
  severity: string;
  status: string;
  summary: string;
  assigneeSub: string;
  alertCount: number;
  createdAt: string;
  updatedAt: string;
  acknowledgedAt: string;
  resolvedAt: string;
  timeline?: { id: string; kind: string; actor: string; message: string; at: string }[];
};

export type VirtualMetrics = {
  scenario: string;
  state: string;
  note: string;
  virtualOnly: boolean;
  points: { name: string; unit: string; value: number }[];
};

async function parse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return res.json() as Promise<T>;
}

function headers(): HeadersInit {
  return { "Content-Type": "application/json", "X-Dev-User-Sub": actor };
}

export async function listServices(): Promise<Service[]> {
  const body = await parse<{ services: Service[] }>(await fetch(`${apiBase}/v1/services`, { cache: "no-store" }));
  return body.services;
}

export async function listIncidents(): Promise<Incident[]> {
  const body = await parse<{ incidents: Incident[] }>(await fetch(`${apiBase}/v1/incidents`, { cache: "no-store" }));
  return body.incidents;
}

export async function getIncident(id: string): Promise<Incident> {
  return parse<Incident>(await fetch(`${apiBase}/v1/incidents/${id}`, { cache: "no-store" }));
}

export async function ackIncident(id: string) {
  return parse<Incident>(await fetch(`${apiBase}/v1/incidents/${id}/ack`, { method: "POST", headers: headers() }));
}

export async function resolveIncident(id: string) {
  return parse<Incident>(await fetch(`${apiBase}/v1/incidents/${id}/resolve`, { method: "POST", headers: headers() }));
}

export async function commentIncident(id: string, body: string) {
  return parse<unknown>(
    await fetch(`${apiBase}/v1/incidents/${id}/comments`, {
      method: "POST",
      headers: headers(),
      body: JSON.stringify({ body }),
    }),
  );
}

export async function fireDemoAlert() {
  return parse<Incident>(await fetch(`${apiBase}/v1/demo/alerts`, { method: "POST", headers: headers() }));
}

export async function scoreTraining(actions: string[], scenario = "bad-deploy"): Promise<TrainingScore> {
  return parse<TrainingScore>(
    await fetch(`${apiBase}/v1/training/score`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ actions, scenario }),
    }),
  );
}

export async function listRunbooks(): Promise<{ id: string; title: string; body: string; serviceCode: string }[]> {
  const body = await parse<{ runbooks: { id: string; title: string; body: string; serviceCode: string }[] }>(
    await fetch(`${apiBase}/v1/runbooks`, { cache: "no-store" }),
  );
  return body.runbooks;
}

export async function getOnCall(): Promise<{ primary: string; secondary: string; note: string; virtualOnly: boolean }> {
  return parse(await fetch(`${apiBase}/v1/oncall`, { cache: "no-store" }));
}

export type TrainingScore = {
  scenario: string;
  actions: string[];
  finalState: string;
  score: number;
  passed: boolean;
  penalties: string[];
  notes: string[];
  virtualOnly: boolean;
};
