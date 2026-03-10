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
  if (!baseUrl || baseUrl === "__AIMMOD_HUB_API_BASE_URL__") return undefined;
  return baseUrl;
}

function getDefaultApiBaseUrl(): string {
  if (typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin;
  }
  if (import.meta.env.DEV) {
    return "http://localhost:8080";
  }
  return "https://aimmod.app";
}

export const API_BASE_URL =
  getRuntimeApiBaseUrl() ||
  import.meta.env.VITE_API_BASE_URL?.trim() ||
  getDefaultApiBaseUrl();
