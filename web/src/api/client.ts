import type {
  HealthResponse,
  LiveEvent,
  Machine,
  MachineStatus,
  Product,
  Slot,
  LoginResponse,
  DispenseStats,
  Restock,
  Dispense,
  AlertConfig,
  AlertEvent,
  StockPrediction,
  AnalyticsKind,
  AnalyticsByKind,
} from "./types";

const BASE = "/api/v1";

const TOKEN_KEY = "soda_token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

const LOGIN_PATH = "/admin/login";

// authFetch sends the stored token. A 401 means the token has expired (24h)
// or was signed with a different secret, so drop it and go back to login
// instead of leaving every admin page showing "API error: 401".
export async function authFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const token = getToken();
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      ...init.headers,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  });
  if (res.status === 401 && token) {
    clearToken();
    if (window.location.pathname !== LOGIN_PATH) {
      window.location.assign(LOGIN_PATH);
    }
  }
  return res;
}

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

async function fetchJSONAuth<T>(path: string): Promise<T> {
  const res = await authFetch(path);
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error(err.error || `API error: ${res.status}`);
  }
  return res.json();
}

async function postJSONAuth<T>(path: string, body: unknown): Promise<T> {
  const res = await authFetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error(err.error || `API error: ${res.status}`);
  }
  return res.json();
}

/**
 * streamEvents reads the server's event stream until it ends, the request
 * fails, or signal aborts. It uses fetch rather than EventSource because
 * EventSource can't send the Authorization header. onOpen runs once the
 * server accepts the stream.
 */
export async function streamEvents({
  signal,
  onOpen,
  onEvent,
}: {
  signal: AbortSignal;
  onOpen: () => void;
  onEvent: (evt: LiveEvent) => void;
}): Promise<void> {
  const res = await authFetch("/events", {
    signal,
    headers: { Accept: "text/event-stream" },
  });
  if (!res.ok || !res.body) {
    throw new Error(`event stream: HTTP ${res.status}`);
  }
  onOpen();
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) return;
    buf += value;
    let end;
    while ((end = buf.indexOf("\n\n")) >= 0) {
      const block = buf.slice(0, end);
      buf = buf.slice(end + 2);
      // Comment lines (":") are keepalives. Only data matters; the event
      // name is repeated in the JSON as type.
      const data = block
        .split("\n")
        .filter((l) => l.startsWith("data:"))
        .map((l) => l.slice(5).trimStart())
        .join("\n");
      if (data) {
        try {
          onEvent(JSON.parse(data));
        } catch {
          // ignore a malformed message
        }
      }
    }
  }
}

// Public
export function getHealth(): Promise<HealthResponse> {
  return fetchJSON("/health");
}

export function listMachines(): Promise<Machine[]> {
  return fetchJSON("/machines");
}

export function getMachineStatus(id: string): Promise<MachineStatus> {
  return fetchJSON(`/machines/${id}/status`);
}

export function listProducts(): Promise<Product[]> {
  return fetchJSONAuth("/products");
}

export function createProduct(product: {
  id?: string;
  name: string;
  description?: string;
  costPerCan?: number;
  color?: string;
}): Promise<Product> {
  return postJSONAuth("/products", product);
}

export function createMachine(machine: {
  id?: string;
  name: string;
  location?: string;
  slotCount: number;
}): Promise<Machine> {
  return postJSONAuth("/machines", machine);
}

export function setSlots(
  machineId: string,
  slots: {
    position: number;
    productId: string;
    capacity: number;
    lowThreshold: number;
    gpioPin?: number;
  }[]
): Promise<Slot[]> {
  return postJSONAuth(`/machines/${machineId}/slots`, slots);
}

// Auth
export function login(
  username: string,
  password: string
): Promise<LoginResponse> {
  return postJSON("/auth/login", { username, password });
}

// Admin (authenticated)
export function getAdminStats(
  machineId: string,
  since?: string
): Promise<DispenseStats> {
  const params = new URLSearchParams({ machineId });
  if (since) params.set("since", since);
  return fetchJSONAuth(`/admin/stats?${params}`);
}

export function getMachineCost(
  machineId: string,
  since?: string
): Promise<{ cost: number }> {
  const params = new URLSearchParams({ machineId });
  if (since) params.set("since", since);
  return fetchJSONAuth(`/admin/cost?${params}`);
}

export function listDispenses(
  machineId: string,
  limit?: number,
  since?: string
): Promise<Dispense[]> {
  const params = new URLSearchParams();
  if (limit) params.set("limit", String(limit));
  if (since) params.set("since", since);
  return fetchJSONAuth(`/machines/${machineId}/dispenses?${params}`);
}

export function listRestocks(
  machineId: string,
  limit?: number
): Promise<Restock[]> {
  const params = new URLSearchParams();
  if (limit) params.set("limit", String(limit));
  return fetchJSONAuth(`/machines/${machineId}/restocks?${params}`);
}

export function createRestock(
  machineId: string,
  slots: { position: number; quantity: number }[]
): Promise<Restock> {
  return postJSONAuth(`/machines/${machineId}/restock`, { slots });
}

// Helpers for PUT/DELETE

async function putJSONAuth<T>(path: string, body: unknown): Promise<T> {
  const res = await authFetch(path, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error(err.error || `API error: ${res.status}`);
  }
  return res.json();
}

async function deleteJSONAuth<T>(path: string): Promise<T> {
  const res = await authFetch(path, { method: "DELETE" });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error(err.error || `API error: ${res.status}`);
  }
  return res.json();
}

// Delete operations
export function deleteProduct(id: string): Promise<{ deleted: string }> {
  return deleteJSONAuth(`/products/${id}`);
}

export function deleteMachine(id: string): Promise<{ deleted: string }> {
  return deleteJSONAuth(`/machines/${id}`);
}

// Alert Configs
export function listAlertConfigs(machineId: string): Promise<AlertConfig[]> {
  return fetchJSONAuth(`/machines/${machineId}/alerts`);
}

export function createAlertConfig(
  machineId: string,
  config: { slotPosition: number; lowThreshold: number; enabled: boolean }
): Promise<AlertConfig> {
  return postJSONAuth(`/machines/${machineId}/alerts`, config);
}

export function updateAlertConfig(
  id: string,
  config: { lowThreshold: number; enabled: boolean }
): Promise<AlertConfig> {
  return putJSONAuth(`/alerts/${id}`, config);
}

export function deleteAlertConfig(
  id: string
): Promise<{ deleted: string }> {
  return deleteJSONAuth(`/alerts/${id}`);
}

// Alert Events
export function listAlertEvents(
  machineId: string,
  activeOnly?: boolean,
  limit?: number
): Promise<AlertEvent[]> {
  const params = new URLSearchParams();
  if (activeOnly) params.set("active", "true");
  if (limit) params.set("limit", String(limit));
  return fetchJSONAuth(`/machines/${machineId}/alert-events?${params}`);
}

// Admin
export function resetAllData(): Promise<{ status: string }> {
  return deleteJSONAuth("/admin/reset");
}

// User Preferences
// Stock Predictions
export function getStockPredictions(
  machineId: string,
  lookbackDays?: number
): Promise<StockPrediction[]> {
  const params = new URLSearchParams();
  if (lookbackDays) params.set("lookbackDays", String(lookbackDays));
  return fetchJSONAuth(`/machines/${machineId}/predictions?${params}`);
}

// Analytics
export function getAnalytics<K extends AnalyticsKind>(
  kind: K,
  params: {
    machineId: string;
    from?: string;
    to?: string;
    tz?: string;
    bucket?: string;
  }
): Promise<AnalyticsByKind[K]> {
  const q = new URLSearchParams({ machineId: params.machineId });
  if (params.from) q.set("from", params.from);
  if (params.to) q.set("to", params.to);
  if (params.tz) q.set("tz", params.tz);
  if (params.bucket) q.set("bucket", params.bucket);
  return fetchJSONAuth(`/analytics/${kind}?${q}`);
}
