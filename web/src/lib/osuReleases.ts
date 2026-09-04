export type OsuReleaseChannel = "stable" | "preview";
export type OsuReleasePlatform = "windows" | "linux";

export interface OsuReleaseAsset {
  operatingSystem: OsuReleasePlatform;
  runtimeIdentifier: "win-x64" | "linux-x64";
  architecture: "x64";
  format: "zip" | "tar.gz";
  fileName: string;
  size: number;
  sha256: string;
  downloadUrl: string;
}

export interface OsuInstallerAsset {
  operatingSystem: OsuReleasePlatform;
  runtimeIdentifier: "win-x64" | "linux-x64";
  architecture: "x64";
  format: "exe" | "AppImage";
  fileName: string;
  size: number;
  sha256: string;
  downloadUrl: string;
  supportsInAppUpdates: true;
}

export interface OsuReleaseManifest {
  schemaVersion: 1;
  product: "aimmod-osu";
  channel: OsuReleaseChannel;
  version: string;
  tag: string;
  releaseUrl: string;
  installers: OsuInstallerAsset[];
  assets: OsuReleaseAsset[];
}

const RELEASE_ROOT = "https://github.com/veryCrunchy/aimmod/releases/download";

export const OSU_RELEASE_MANIFEST_URLS: Record<OsuReleaseChannel, string> = {
  stable: `${RELEASE_ROOT}/aimmod-osu-stable/aimmod-osu-stable.json`,
  preview: `${RELEASE_ROOT}/aimmod-osu-preview/aimmod-osu-preview.json`,
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isHttpsUrl(value: unknown): value is string {
  if (typeof value !== "string") return false;
  try {
    return new URL(value).protocol === "https:";
  } catch {
    return false;
  }
}

function parseAsset(value: unknown): OsuReleaseAsset | null {
  if (!isRecord(value)) return null;

  const operatingSystem = value.operatingSystem;
  const runtimeIdentifier = value.runtimeIdentifier;
  const format = value.format;
  const expectedRuntime = operatingSystem === "windows" ? "win-x64" : operatingSystem === "linux" ? "linux-x64" : null;
  const expectedFormat = operatingSystem === "windows" ? "zip" : operatingSystem === "linux" ? "tar.gz" : null;

  if (
    expectedRuntime === null
    || runtimeIdentifier !== expectedRuntime
    || format !== expectedFormat
    || value.architecture !== "x64"
    || typeof value.fileName !== "string"
    || !value.fileName
    || typeof value.size !== "number"
    || !Number.isSafeInteger(value.size)
    || value.size <= 0
    || typeof value.sha256 !== "string"
    || !/^[a-f\d]{64}$/i.test(value.sha256)
    || !isHttpsUrl(value.downloadUrl)
  ) {
    return null;
  }

  return value as unknown as OsuReleaseAsset;
}

function parseInstaller(value: unknown): OsuInstallerAsset | null {
  if (!isRecord(value)) return null;

  const operatingSystem = value.operatingSystem;
  const runtimeIdentifier = value.runtimeIdentifier;
  const expectedRuntime = operatingSystem === "windows" ? "win-x64" : operatingSystem === "linux" ? "linux-x64" : null;
  const expectedFormat = operatingSystem === "windows" ? "exe" : operatingSystem === "linux" ? "AppImage" : null;
  if (
    expectedRuntime === null
    || runtimeIdentifier !== expectedRuntime
    || value.format !== expectedFormat
    || value.architecture !== "x64"
    || typeof value.fileName !== "string"
    || !value.fileName
    || typeof value.size !== "number"
    || !Number.isSafeInteger(value.size)
    || value.size <= 0
    || typeof value.sha256 !== "string"
    || !/^[a-f\d]{64}$/i.test(value.sha256)
    || !isHttpsUrl(value.downloadUrl)
    || value.supportsInAppUpdates !== true
  ) {
    return null;
  }

  return value as unknown as OsuInstallerAsset;
}

export function parseOsuReleaseManifest(value: unknown, expectedChannel: OsuReleaseChannel): OsuReleaseManifest {
  if (!isRecord(value) || !Array.isArray(value.assets) || !Array.isArray(value.installers)) {
    throw new Error("The release channel returned an invalid manifest.");
  }

  const assets = value.assets.map(parseAsset);
  const parsedAssets = assets.filter((asset): asset is OsuReleaseAsset => asset !== null);
  const runtimeIds = new Set(parsedAssets.map((asset) => asset.runtimeIdentifier));
  const installers = value.installers.map(parseInstaller);
  const parsedInstallers = installers.filter((asset): asset is OsuInstallerAsset => asset !== null);
  const installerRuntimeIds = new Set(parsedInstallers.map((asset) => asset.runtimeIdentifier));
  const version = typeof value.version === "string" ? value.version : "";
  const isPrerelease = version.includes("-");

  if (
    value.schemaVersion !== 1
    || value.product !== "aimmod-osu"
    || value.channel !== expectedChannel
    || !/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version)
    || (expectedChannel === "stable" ? isPrerelease : !isPrerelease)
    || value.tag !== `aimmod-osu-v${version}`
    || !isHttpsUrl(value.releaseUrl)
    || parsedAssets.length !== value.assets.length
    || runtimeIds.size !== parsedAssets.length
    || !runtimeIds.has("win-x64")
    || !runtimeIds.has("linux-x64")
    || parsedInstallers.length !== value.installers.length
    || installerRuntimeIds.size !== parsedInstallers.length
    || !installerRuntimeIds.has("win-x64")
    || !installerRuntimeIds.has("linux-x64")
  ) {
    throw new Error("The release channel returned an invalid manifest.");
  }

  const expectedDownloadRoot = `${RELEASE_ROOT}/${value.tag}/`;
  if ([...parsedAssets, ...parsedInstallers].some((asset) => !asset.downloadUrl.startsWith(expectedDownloadRoot))) {
    throw new Error("The release channel returned an invalid manifest.");
  }

  return { ...value, assets: parsedAssets, installers: parsedInstallers } as unknown as OsuReleaseManifest;
}

export async function fetchOsuReleaseManifest(
  channel: OsuReleaseChannel,
  signal?: AbortSignal,
): Promise<OsuReleaseManifest> {
  let response: Response;
  try {
    response = await fetch(OSU_RELEASE_MANIFEST_URLS[channel], {
      headers: { Accept: "application/json" },
      cache: "no-cache",
      signal,
    });
  } catch (reason) {
    if (reason instanceof DOMException && reason.name === "AbortError") throw reason;
    throw new Error("Release information is temporarily unavailable.");
  }

  if (!response.ok) {
    if (response.status === 404) {
      throw new Error(`${channel === "stable" ? "Stable" : "Preview"} builds are not published yet.`);
    }
    throw new Error("Release information is temporarily unavailable.");
  }

  return parseOsuReleaseManifest(await response.json(), channel);
}

export function findOsuReleaseAsset(manifest: OsuReleaseManifest, platform: OsuReleasePlatform) {
  const runtimeIdentifier = platform === "windows" ? "win-x64" : "linux-x64";
  return manifest.assets.find((asset) => asset.runtimeIdentifier === runtimeIdentifier) ?? null;
}

export function findOsuInstaller(manifest: OsuReleaseManifest, platform: OsuReleasePlatform) {
  const runtimeIdentifier = platform === "windows" ? "win-x64" : "linux-x64";
  return manifest.installers.find((asset) => asset.runtimeIdentifier === runtimeIdentifier) ?? null;
}

export function formatFileSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "";
  const units = ["B", "KB", "MB", "GB"];
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** exponent;
  return `${value >= 100 || exponent === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[exponent]}`;
}
