import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { HelmetProvider } from "../src/lib/helmet";
import { BrandingPage } from "../src/pages/BrandingPage";
import { brandAssets, brandRoot, iconSizes } from "../src/lib/brandAssets";

test("brand library serves only the selected artwork and all download targets exist", () => {
  const root = new URL(`../public${brandRoot}/`, import.meta.url);
  for (const asset of brandAssets) {
    for (const ext of ["svg", "png"]) {
      const bytes = readFileSync(new URL(`${asset.file}.${ext}`, root));
      assert.ok(bytes.length > 100);
      if (ext === "svg") assert.doesNotMatch(bytes.toString(), /(?:file:\/\/|[A-Z]:[\\/]Users[\\/]|\.codex[\\/]|<script\b|<foreignObject\b)/i);
    }
  }
  for (const size of iconSizes) {
    const png = readFileSync(new URL(`icons/aimmod-${size}.png`, root));
    assert.equal(png.readUInt32BE(16), size);
    assert.equal(png.readUInt32BE(20), size);
  }
  assert.ok(readFileSync(new URL("../public/brand/aimmod-brand-kit.zip", import.meta.url)).length > 1000);
});

test("branding renders assets, usage and real downloadable files", () => {
  const body = renderToString(createElement(HelmetProvider, null, createElement(MemoryRouter, { initialEntries: ["/branding"] }, createElement(BrandingPage))));
  assert.equal((body.match(/<h1\b/g) ?? []).length, 1);
  for (const asset of brandAssets) assert.ok(body.includes(`${brandRoot}/${asset.file}.svg`));
  assert.ok(body.includes("usage.md"));
  assert.ok(body.includes("aimmod-brand-kit.zip"));
  assert.ok(body.includes("Copy Mint #27E4A1"));
  assert.doesNotMatch(body, /design-qa|vector-master|proof\.svg/);
});
