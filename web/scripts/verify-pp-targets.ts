import { createRequire } from "node:module";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { playbackBeatmap } from "../tests/fixtures/osuPlaybackFixture";
const require = createRequire(import.meta.url);
const { chromium } = require("playwright");
const base = process.env.PP_QA_URL || "http://127.0.0.1:5190";
const api = process.env.PP_QA_API || "http://127.0.0.1:5191";
const checksum = createHash("md5").update(playbackBeatmap).digest("hex");
const browser = await chromium.launch({ channel: process.env.PP_QA_BROWSER || "msedge", headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 950 } });
  await context.route("**/SearchBeatmapItems", (route: any) => route.fulfill({ json: { providers: [{ provider: 1, available: true }], items: [{ provider: 1, sourceId: "12" }] } }));
  await context.route("**/GetBeatmapItem", (route: any) => route.fulfill({ json: { item: { sourceId: "12", difficulties: [{ beatmapId: "42", beatmapsetId: "12", checksum, name: "Synthetic sliders", title: "Example map", artist: "Example", creator: "Example", stars: 4, ruleset: 1, bpm: 120, lengthSeconds: 9 }] } } }));
  await context.route("**/playback/beatmaps/42/file?*", (route: any) => route.fulfill({ body: playbackBeatmap, contentType: "text/plain" }));
  const page = await context.newPage();
  const errors: string[] = [];
  page.on("pageerror", (error: Error) => errors.push(error.message));
  await page.goto(`${base}/osu/pp-targets`, {waitUntil: "domcontentloaded"});
  for (const lazer of [true, false, true]) {
    await page.getByLabel("Scoring", { exact: true }).selectOption(String(lazer));
    const response = await fetch(`${api}/api/osu/v1/pp/calculate`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ map: Buffer.from(playbackBeatmap).toString("base64"), checksum, lazer, accuracy: 98, mods: [] }) });
    assert.equal(response.ok, true);
    const expected = await response.json() as { pp: number; engine: string };
    assert.equal(expected.engine, "aimmod-osu-2026.730.0-v1");
    await page.locator(".pp-card-value strong").filter({ hasText: `${Math.round(expected.pp)} pp` }).first().waitFor({ timeout: 45000 });
    assert.match(await page.locator(".pp-card-value").first().innerText(), lazer ? /Lazer/ : /Stable/);
  }
  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  assert.deepEqual(errors, []);
  console.log("Passed: browser/official-worker parity, stable/lazer switching, mobile layout and runtime checks.");
} finally { await browser.close(); }
