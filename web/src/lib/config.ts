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
  if (import.meta.env.PROD && typeof window !== "undefined") {
    // Production default points to same-origin API unless overridden.
    return window.location.origin;
  }
  return "http://localhost:8080";
}

export const API_BASE_URL =
  getRuntimeApiBaseUrl() ||
  import.meta.env.VITE_API_BASE_URL?.trim() ||
  getDefaultApiBaseUrl();
