import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { createHash } from "node:crypto";

const assetRoot = new URL("../public/brand/aimmod-v9/", import.meta.url);
const hashes: Record<string, string> = {
  "aimmod-256.png": "dd6356b850f6f33349d4595da0d1d95250f2d1e072720fd6aca00d9fd379f81c",
  "aimmod.ico": "74310c2cd8e2727dc84a91ea5b64f8a46eeef205bb69a4e6c1d026363cad106f",
  "app-icon.svg": "d516d465feae6d00eaf9157c5c47e27eeb6102d471e25779300980ab2e881f28",
  "mark-mint.svg": "15ff13b60215e77d9865484f0b2d7a6d12550b13f82c25d9dfc3f404907e0169",
  "share-card-1200x630.png": "be91da8bed27aed83332651ac8b7417c276f7ce591d4250041aef6c0361ff753",
  "wordmark-white.svg": "c1fa343a01ddd7a307227c6050e43e37bb3915b6bd18b1644c940b594e47bcd9",
};

test("the public brand directory contains only the selected v9 production exports", () => {
  assert.deepEqual(readdirSync(assetRoot).sort(), Object.keys(hashes).sort());
  for (const [name, hash] of Object.entries(hashes)) {
    const bytes = readFileSync(new URL(name, assetRoot));
    assert.equal(createHash("sha256").update(bytes).digest("hex"), hash, name);
    if (name.endsWith(".svg")) assert.doesNotMatch(bytes.toString(), /<(?:image|text|foreignObject)\b/);
  }
});

test("favicon and social metadata reference shipped v9 files", () => {
  const html = readFileSync(new URL("../index.html", import.meta.url), "utf8");
  const references = [...html.matchAll(/(?:href|content)="(?:https:\/\/aimmod\.app)?\/brand\/aimmod-v9\/([^"?#]+)"/g)];
  assert.ok(references.length >= 5);
  for (const [, name] of references) assert.ok(readFileSync(new URL(name, assetRoot)).length > 0);
  assert.match(html, /name="twitter:card" content="summary_large_image"/);
  const share = readFileSync(new URL("share-card-1200x630.png", assetRoot));
  assert.equal(share.readUInt32BE(16), 1200);
  assert.equal(share.readUInt32BE(20), 630);
  assert.deepEqual(readFileSync(new URL("../public/favicon.ico", import.meta.url)), readFileSync(new URL("aimmod.ico", assetRoot)));
});
