import type {
  Machine,
  MachineStatus,
  LoginResponse,
  DispenseStats,
  Restock,
  Dispense,
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

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

async function fetchJSONAuth<T>(path: string): Promise<T> {
  const token = getToken();
  const res = await fetch(`${BASE}${path}`, {
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  });
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
  const token = getToken();
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error(err.error || `API error: ${res.status}`);
  }
  return res.json();
}

// Public
export function listMachines(): Promise<Machine[]> {
  return fetchJSON("/machines");
}

export function getMachineStatus(id: string): Promise<MachineStatus> {
  return fetchJSON(`/machines/${id}/status`);
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
