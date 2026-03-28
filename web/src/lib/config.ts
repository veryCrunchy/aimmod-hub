type RuntimeConfig = {
  apiBaseUrl?: string;
};

type ImportMetaEnvLike = {
  DEV?: boolean;
  VITE_API_BASE_URL?: string;
};

type RuntimeWindow = Window & {
  __AIMMOD_HUB__?: RuntimeConfig;
};

function getImportMetaEnv(): ImportMetaEnvLike | undefined {
  return (import.meta as ImportMeta & { env?: ImportMetaEnvLike }).env;
}

function getEnvApiBaseUrl(): string | undefined {
  const viteValue = getImportMetaEnv()?.VITE_API_BASE_URL?.trim();
  if (viteValue) return viteValue;

  if (typeof process !== "undefined") {
    const processValue = process.env.VITE_API_BASE_URL?.trim();
    if (processValue) return processValue;
  }

  return undefined;
}

function isDevEnvironment(): boolean {
  if (getImportMetaEnv()?.DEV) return true;
  if (typeof process !== "undefined") {
    return process.env.NODE_ENV !== "production";
  }
  return false;
}

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
  if (isDevEnvironment()) {
    return "http://localhost:8080";
  }
  return "https://aimmod.app";
}

export const API_BASE_URL =
  getRuntimeApiBaseUrl() ||
  getEnvApiBaseUrl() ||
  getDefaultApiBaseUrl();
