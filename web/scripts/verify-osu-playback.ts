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
const song = Buffer.alloc(44 + 44100 * 10 * 2);
song.write("RIFF"); song.writeUInt32LE(song.length - 8, 4); song.write("WAVEfmt ", 8);
song.writeUInt32LE(16, 16); song.writeUInt16LE(1, 20); song.writeUInt16LE(1, 22);
song.writeUInt32LE(44100, 24); song.writeUInt32LE(88200, 28); song.writeUInt16LE(2, 32);
song.writeUInt16LE(16, 34); song.write("data", 36); song.writeUInt32LE(song.length - 44, 40);
for (let i = 0; i < 441000; i++) song.writeInt16LE(Math.round(Math.sin(i * 2 * Math.PI * 220 / 44100) * 3000), 44 + i * 2);
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
      (window as any).__songStarts = [];
      const start = AudioBufferSourceNode.prototype.start;
      AudioBufferSourceNode.prototype.start = function(when = 0, offset = 0, duration?: number) {
        if (this.buffer && this.buffer.duration >= 10) (window as any).__songStarts.push({ offset, nonzero: this.buffer.getChannelData(0).some(value => Math.abs(value) > 0.01) });
        if (duration === undefined) start.call(this, when, offset); else start.call(this, when, offset, duration);
      };
      window.AudioContext = class extends NativeAudio {
        constructor(options?: AudioContextOptions) { super(options); (window as any).__audioContexts.push(this); }
      };
    });
    await page.route("**/__qa/replay.osr", (route: any) => route.fulfill({ body: replay, contentType: "application/octet-stream" }));
    await page.route("**/__qa/beatmap.osu", (route: any) => route.fulfill({ body: playbackBeatmap, contentType: "text/plain" }));
    await page.route("**/playback/beatmaps/42/audio?*", (route: any) => route.fulfill({ body: song, contentType: "audio/wav" }));
    await page.goto(`${base}/tests/fixtures/playback-qa.html`);
    await page.locator('.osu-replay-player[data-state="ready"]').waitFor({ timeout: 60000 });
    assert.equal(await page.getByRole("button", { name: "Play replay", exact: true }).isVisible(), true);
    const songVolume = page.getByRole("slider", { name: "Song volume", exact: true });
    const hitsoundVolume = page.getByRole("slider", { name: "Hitsound volume", exact: true });
    await songVolume.fill("0.2");
    assert.equal(await hitsoundVolume.inputValue(), "0.35");
    await hitsoundVolume.fill("0.8");
    assert.equal(await songVolume.inputValue(), "0.2");
    await page.getByRole("button", { name: "Replay display settings" }).click();
    await page.getByRole("slider", { name: "Background dim", exact: true }).fill("0.8");
    await page.getByRole("checkbox", { name: "Key presses", exact: true }).uncheck();
    await page.getByRole("checkbox", { name: "Hit timing", exact: true }).uncheck();
    await page.getByRole("checkbox", { name: "Key presses", exact: true }).check();
    await page.getByRole("checkbox", { name: "Hit timing", exact: true }).check();
    await page.getByRole("slider", { name: "Background dim", exact: true }).fill("0.65");
    await page.getByRole("button", { name: "Replay display settings" }).click();
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
    const audioStarts = await page.evaluate(() => (window as any).__songStarts);
    assert.ok(audioStarts.some((item: { offset: number; nonzero: boolean }) => item.nonzero && Math.abs(item.offset - 2.7) < 0.1), "Song buffer was not played at the seek position");
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
    report.push({ width, pixels, song: "non-silent ten-second buffer scheduled at seek position", controls: "play/pause/seek/speed/restart/end passed", hiddenTab: "paused and audio suspended", cleanup: "all audio contexts closed", errors });
    await page.route("**/playback/beatmaps/42/audio?*", (route: any) => route.fulfill({ status: 403, body: "Unavailable" }));
    await page.reload();
    await page.locator('.osu-replay-player[data-state="ready"]').waitFor({ timeout: 60000 });
    assert.equal(await page.getByText("Hitsounds only", { exact: true }).isVisible(), true);
    await page.getByRole("button", { name: "Play replay", exact: true }).click();
    assert.equal(await page.getByText("The matching song is unavailable. Replay hitsounds remain enabled.", { exact: true }).isVisible(), true);
    assert.equal(await page.locator('input[accept="audio/*"]').count(), 0);
    await page.close();
  }
  writeFileSync(resolve(output, "report.json"), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
} finally { await browser.close(); }
