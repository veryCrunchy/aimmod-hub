type RuntimeConfig = {
  apiBaseUrl?: string;
};

type RuntimeWindow = Window & {
  __AIMMOD_HUB__?: RuntimeConfig;
};

function getRuntimeApiBaseUrl(): string | undefined {
  if (typeof window === "undefined") return undefined;
  const runtime = (window as RuntimeWindow).__AIMMOD_HUB__;
  const baseUrl = runtime?.apiBaseUrl?.trim();
  return baseUrl ? baseUrl : undefined;
}

function getDefaultApiBaseUrl(): string {
  return "https://api.aimmod.app";
}

export const API_BASE_URL =
  getRuntimeApiBaseUrl() ||
  import.meta.env.VITE_API_BASE_URL?.trim() ||
  getDefaultApiBaseUrl();
