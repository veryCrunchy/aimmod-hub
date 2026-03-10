import { API_BASE_URL } from "./config";

export type AuthUser = {
  userId: number;
  userExternalId: string;
  discordUserId: string;
  username: string;
  displayName: string;
  avatarUrl: string;
};

export type UploadTokenRecord = {
  id: number;
  label: string;
  lastFour: string;
  createdAt: string;
  lastUsedAt?: string | null;
};

export type SessionPayload = {
  authenticated: boolean;
  isAdmin?: boolean;
  adminConfigured?: boolean;
  adminReason?: string;
  user?: AuthUser;
  tokens?: UploadTokenRecord[];
};

const apiBase = API_BASE_URL;

export async function fetchSession(): Promise<SessionPayload> {
  const response = await fetch(`${apiBase}/auth/session`, {
    credentials: "include"
  });
  if (!response.ok) {
    throw new Error(`Session request failed: ${response.status}`);
  }
  return response.json();
}

export async function logout(): Promise<void> {
  const response = await fetch(`${apiBase}/auth/logout`, {
    method: "POST",
    credentials: "include"
  });
  if (!response.ok) {
    throw new Error(`Logout failed: ${response.status}`);
  }
}

export async function createUploadToken(label: string) {
  const response = await fetch(`${apiBase}/auth/upload-tokens`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({ label })
  });
  if (!response.ok) {
    throw new Error(`Create upload token failed: ${response.status}`);
  }
  return response.json() as Promise<{ token: string; record: UploadTokenRecord }>;
}

export async function revokeUploadToken(id: number) {
  const response = await fetch(`${apiBase}/auth/upload-tokens/revoke`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({ id })
  });
  if (!response.ok) {
    throw new Error(await response.text() || `Revoke upload token failed: ${response.status}`);
  }
}

export async function approveDeviceLink(userCode: string) {
  const response = await fetch(`${apiBase}/auth/device/approve`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({ userCode })
  });
  if (!response.ok) {
    throw new Error(await response.text() || `Device approval failed: ${response.status}`);
  }
  return response.json() as Promise<{ status: string }>;
}

export function discordStartUrl(returnTo = "/account") {
  const url = new URL(`${apiBase}/auth/discord/start`);
  url.searchParams.set("return_to", returnTo);
  return url.toString();
}
