import { createRequire } from "node:module";
import assert from "node:assert/strict";
import { parseOsuReleaseManifest } from "../src/lib/osuReleases";
const require = createRequire(import.meta.url);
const { chromium } = require("playwright");
const response = await fetch("https://github.com/veryCrunchy/aimmod/releases/download/aimmod-osu-stable/aimmod-osu-stable.json");
assert.ok(response.ok);
const manifest = parseOsuReleaseManifest(await response.json(), "stable");
const browser = await chromium.launch({ channel: "chrome", headless: true });
try {
  const page = await browser.newPage();
  await page.route("**/api/osu/v1/releases/stable", (route: any) => route.fulfill({ json: manifest }));
  await page.goto("http://127.0.0.1:5192/app/osu");
  for (const platform of ["Windows", "Linux"]) {
    const button = page.getByRole("link", { name: `Download for ${platform}`, exact: true });
    await button.waitFor();
    assert.ok((await button.getAttribute("href"))?.includes(`/aimmod-osu-v${manifest.version}/`));
  }
  assert.equal(await page.getByText("No build is available in this channel.", { exact: true }).count(), 0);
  console.log(`Live channel v${manifest.version}: Windows and Linux download links verified`);
} finally { await browser.close(); }
