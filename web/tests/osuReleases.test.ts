import assert from "node:assert/strict";
import test from "node:test";
import {
  findOsuReleaseAsset,
  findOsuInstaller,
  formatFileSize,
  parseOsuReleaseManifest,
} from "../src/lib/osuReleases";

function manifest(overrides: Record<string, unknown> = {}) {
  return {
    schemaVersion: 1,
    product: "aimmod-osu",
    channel: "stable",
    version: "1.2.3",
    tag: "aimmod-osu-v1.2.3",
    releaseUrl: "https://github.com/veryCrunchy/aimmod/releases/tag/aimmod-osu-v1.2.3",
    installers: [
      {
        operatingSystem: "windows",
        runtimeIdentifier: "win-x64",
        architecture: "x64",
        format: "exe",
        fileName: "AimMod.Osu-win-stable-Setup.exe",
        size: 110100480,
        sha256: "c".repeat(64),
        downloadUrl: "https://github.com/veryCrunchy/aimmod/releases/download/aimmod-osu-v1.2.3/AimMod.Osu-win-stable-Setup.exe",
        supportsInAppUpdates: true,
      },
      {
        operatingSystem: "linux",
        runtimeIdentifier: "linux-x64",
        architecture: "x64",
        format: "AppImage",
        fileName: "AimMod.Osu-linux-stable.AppImage",
        size: 104857600,
        sha256: "d".repeat(64),
        downloadUrl: "https://github.com/veryCrunchy/aimmod/releases/download/aimmod-osu-v1.2.3/AimMod.Osu-linux-stable.AppImage",
        supportsInAppUpdates: true,
      },
    ],
    assets: [
      {
        operatingSystem: "windows",
        runtimeIdentifier: "win-x64",
        architecture: "x64",
        format: "zip",
        fileName: "aimmod-osu-1.2.3-win-x64.zip",
        size: 104857600,
        sha256: "a".repeat(64),
        downloadUrl: "https://github.com/veryCrunchy/aimmod/releases/download/aimmod-osu-v1.2.3/aimmod-osu-1.2.3-win-x64.zip",
      },
      {
        operatingSystem: "linux",
        runtimeIdentifier: "linux-x64",
        architecture: "x64",
        format: "tar.gz",
        fileName: "aimmod-osu-1.2.3-linux-x64.tar.gz",
        size: 94371840,
        sha256: "b".repeat(64),
        downloadUrl: "https://github.com/veryCrunchy/aimmod/releases/download/aimmod-osu-v1.2.3/aimmod-osu-1.2.3-linux-x64.tar.gz",
      },
    ],
    ...overrides,
  };
}

test("parses a complete AimMod osu release manifest", () => {
  const parsed = parseOsuReleaseManifest(manifest(), "stable");

  assert.equal(parsed.version, "1.2.3");
  assert.equal(findOsuReleaseAsset(parsed, "windows")?.format, "zip");
  assert.equal(findOsuReleaseAsset(parsed, "linux")?.format, "tar.gz");
  assert.equal(findOsuInstaller(parsed, "windows")?.format, "exe");
});

test("rejects another product or channel", () => {
  assert.throws(() => parseOsuReleaseManifest(manifest({ product: "aimmod" }), "stable"));
  assert.throws(() => parseOsuReleaseManifest(manifest({ channel: "preview" }), "stable"));
});

test("keeps stable and preview versions in their matching channels", () => {
  assert.throws(() => parseOsuReleaseManifest(manifest({ version: "1.2.3-preview.1", tag: "aimmod-osu-v1.2.3-preview.1" }), "stable"));
  assert.throws(() => parseOsuReleaseManifest(manifest({ channel: "preview" }), "preview"));
});

test("rejects a manifest without both supported platforms", () => {
  const value = manifest();
  value.assets = value.assets.slice(0, 1);
  assert.throws(() => parseOsuReleaseManifest(value, "stable"));
});

test("formats release asset sizes", () => {
  assert.equal(formatFileSize(104857600), "100 MB");
  assert.equal(formatFileSize(1572864), "1.5 MB");
});
