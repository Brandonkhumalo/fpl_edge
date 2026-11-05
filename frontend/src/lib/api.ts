import { useAuth } from "./auth";

const baseURL = import.meta.env.VITE_API_URL ?? "http://localhost:8080/api/v1";

type APIErrorBody = { error?: { message?: string } };

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const { accessToken, setSession, clear, user } = useAuth.getState();
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
  let response = await fetch(`${baseURL}${path}`, { ...init, headers, credentials: "include" });
  if (response.status === 401 && user && path !== "/auth/refresh") {
    const refreshResponse = await fetch(`${baseURL}/auth/refresh`, { method: "POST", headers: { "Content-Type": "application/json" }, credentials: "include" });
    if (refreshResponse.ok && user) {
      const refreshed = await refreshResponse.json() as { accessToken: string; refreshToken: string };
      setSession({ user, ...refreshed });
      headers.set("Authorization", `Bearer ${refreshed.accessToken}`);
      response = await fetch(`${baseURL}${path}`, { ...init, headers, credentials: "include" });
    } else {
      clear();
    }
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as APIErrorBody;
    throw new Error(body.error?.message ?? `Request failed (${response.status})`);
  }
  return (response.status === 204 ? undefined : response.json()) as Promise<T>;
}

export async function downloadAPI(path: string, filename: string) {
  const token = useAuth.getState().accessToken;
  const response = await fetch(`${baseURL}${path}`, { headers: token ? { Authorization: `Bearer ${token}` } : {}, credentials: "include" });
  if (!response.ok) throw new Error(`Download failed (${response.status})`);
  const url = URL.createObjectURL(await response.blob());
  const link = document.createElement("a"); link.href = url; link.download = filename; link.click(); URL.revokeObjectURL(url);
}

