import { createRequire } from "node:module";
import { mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import assert from "node:assert/strict";
import { createPlaybackReplay, playbackBeatmap } from "../tests/fixtures/osuPlaybackFixture";

const require = createRequire(import.meta.url);
const { chromium } = require("playwright");
const base = process.env.PLAYBACK_QA_URL || "http://127.0.0.1:5192";
const output = resolve(process.env.PLAYBACK_QA_OUTPUT || "../.qa/replay-player");
mkdirSync(output, { recursive: true });
const replay = await createPlaybackReplay();
const browser = await chromium.launch({ channel: "chrome", headless: true });
const report: unknown[] = [];
try {
  for (const width of [1440, 390]) {
    const page = await browser.newPage({ viewport: { width, height: 950 }, deviceScaleFactor: 1 });
    const errors: string[] = [];
    page.on("pageerror", (error: Error) => errors.push(error.message));
    await page.addInitScript(() => {
      const NativeAudio = window.AudioContext;
      (window as any).__audioContexts = [];
      window.AudioContext = class extends NativeAudio {
        constructor(options?: AudioContextOptions) { super(options); (window as any).__audioContexts.push(this); }
      };
    });
    await page.route("**/__qa/replay.osr", (route: any) => route.fulfill({ body: replay, contentType: "application/octet-stream" }));
    await page.route("**/__qa/beatmap.osu", (route: any) => route.fulfill({ body: playbackBeatmap, contentType: "text/plain" }));
    await page.goto(`${base}/tests/fixtures/playback-qa.html`);
    await page.locator('.osu-replay-player[data-state="ready"]').waitFor({ timeout: 60000 });
    assert.equal(await page.getByRole("button", { name: "Play replay", exact: true }).isVisible(), true);
    const timeline = page.getByRole("slider", { name: "Replay timeline" });
    await timeline.fill("2700");
    await page.waitForTimeout(200);
    const pixels = await page.locator("canvas").evaluate((canvas: HTMLCanvasElement) => {
      const context = canvas.getContext("2d")!;
      const { data } = context.getImageData(0, 0, canvas.width, canvas.height);
      let bright = 0, coloured = 0;
      for (let i = 0; i < data.length; i += 4) {
        if (Math.max(data[i], data[i + 1], data[i + 2]) > 100) bright++;
        if (Math.max(data[i], data[i + 1], data[i + 2]) - Math.min(data[i], data[i + 1], data[i + 2]) > 40) coloured++;
      }
      return { bright, coloured, width: canvas.width, height: canvas.height };
    });
    assert.ok(pixels.bright > 2000 && pixels.coloured > 1000, `Blank canvas: ${JSON.stringify(pixels)}`);
    await page.screenshot({ path: resolve(output, `playback-${width}.png`), fullPage: true });
    await page.getByRole("button", { name: "Play replay", exact: true }).click();
    await page.waitForTimeout(550);
    const advanced = Number(await timeline.inputValue());
    assert.ok(advanced > 3000, "Replay clock did not advance");
    await page.getByRole("button", { name: "Pause replay", exact: true }).click();
    const paused = Number(await timeline.inputValue());
    await page.waitForTimeout(350);
    assert.ok(Math.abs(Number(await timeline.inputValue()) - paused) < 100, "Paused clock kept advancing");
    await page.locator("select").selectOption("2");
    await page.getByRole("button", { name: "Play replay", exact: true }).click();
    await page.waitForTimeout(500);
    assert.ok(Number(await timeline.inputValue()) - paused > 650, "Speed control did not change playback clock");
    await page.getByRole("button", { name: "Restart replay" }).click();
    await page.waitForTimeout(150);
    assert.ok(Number(await timeline.inputValue()) < 50, "Restart did not reset clock");
    const layout = await page.evaluate(() => ({ content: document.documentElement.scrollWidth, viewport: innerWidth }));
    assert.ok(layout.content <= layout.viewport, `Horizontal overflow at ${width}`);
    await page.getByRole("button", { name: "Play replay", exact: true }).click();
    await page.evaluate(() => {
      Object.defineProperty(document, "hidden", { configurable: true, value: true });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await page.getByRole("button", { name: "Play replay", exact: true }).waitFor();
    assert.equal(await page.evaluate(() => (window as any).__audioContexts.every((context: AudioContext) => context.state === "suspended")), true, "Hidden tab did not suspend audio");
    await page.evaluate(() => {
      Object.defineProperty(document, "hidden", { configurable: true, value: false });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    const end = Number(await timeline.getAttribute("max"));
    await timeline.fill(String(Math.floor((end - 200) / 10) * 10));
    await page.getByRole("button", { name: "Play replay", exact: true }).click();
    await page.waitForTimeout(500);
    assert.equal(await page.getByRole("button", { name: "Play replay", exact: true }).isVisible(), true, "Replay did not stop at end");
    await page.getByRole("button", { name: "Play replay", exact: true }).click();
    await page.evaluate(() => (window as any).unmountReplay());
    await page.waitForTimeout(200);
    assert.equal(await page.evaluate(() => (window as any).__audioContexts.every((context: AudioContext) => context.state === "closed")), true, "Audio survives route unmount");
    assert.deepEqual(errors, []);
    report.push({ width, pixels, controls: "play/pause/seek/speed/restart/end passed", hiddenTab: "paused and audio suspended", cleanup: "all audio contexts closed", errors });
    await page.close();
  }
  writeFileSync(resolve(output, "report.json"), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
} finally { await browser.close(); }
