// Client API REST. Le refresh token est conservé en localStorage pour ce
// squelette v1 ; un durcissement ultérieur (cookie httpOnly + SameSite)
// est identifié dans docs/SECURITY.md comme prochaine étape.
const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "/api/v1";

let accessToken: string | null = null;

export function getAccessToken() {
  return accessToken;
}

export function setTokens(access: string, refresh: string) {
  accessToken = access;
  localStorage.setItem("sharedesk_refresh_token", refresh);
}

export function clearTokens() {
  accessToken = null;
  localStorage.removeItem("sharedesk_refresh_token");
}

export function getRefreshToken() {
  return localStorage.getItem("sharedesk_refresh_token");
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function parseError(res: Response): Promise<never> {
  let message = res.statusText;
  try {
    const body = await res.json();
    if (body?.error) message = body.error;
  } catch {
    // réponse non-JSON, on garde le statusText
  }
  throw new ApiError(res.status, message);
}

async function refreshAccessToken(): Promise<boolean> {
  const refresh = getRefreshToken();
  if (!refresh) return false;
  const res = await fetch(`${API_BASE}/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refresh }),
  });
  if (!res.ok) {
    clearTokens();
    return false;
  }
  const data = await res.json();
  setTokens(data.access_token, data.refresh_token);
  return true;
}

export async function apiFetch(path: string, options: RequestInit = {}, retry = true): Promise<Response> {
  const headers = new Headers(options.headers);
  if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
  if (options.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });

  if (res.status === 401 && retry) {
    const refreshed = await refreshAccessToken();
    if (refreshed) return apiFetch(path, options, false);
  }
  return res;
}

export async function apiJSON<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await apiFetch(path, options);
  if (!res.ok) return parseError(res);
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export function wsBaseURL(): string {
  return import.meta.env.VITE_WS_BASE_URL ?? `${location.origin.replace(/^http/, "ws")}/api/v1`;
}
