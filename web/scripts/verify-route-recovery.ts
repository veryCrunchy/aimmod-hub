import { createRequire } from "node:module";
import assert from "node:assert/strict";

const require = createRequire(import.meta.url);
const { chromium } = require("playwright");
const base = process.env.ROUTE_QA_URL || "http://127.0.0.1:5192";
const browser = await chromium.launch({ channel: "chrome", headless: true });
try {
  const failures = process.env.ROUTE_QA_PRODUCTION === "1" ? ["page", "shared", "css"] : ["page"];
  for (const failure of failures) {
  for (const width of [1440, 390]) {
    const page = await browser.newPage({ viewport: { width, height: 950 } });
    let documents = 0;
    page.on("request", (request: any) => { if (request.isNavigationRequest() && request.frame() === page.mainFrame()) documents++; });
    const pattern = failure === "shared" ? /\/assets\/osuCatalog-[^/]+\.js$/ : failure === "css" ? /\/assets\/OsuCatalogPage-[^/]+\.css$/
      : /\/(?:assets\/OsuCatalogPage-[^/]+\.js|src\/pages\/OsuCatalogPage\.tsx)(?:\?.*)?$/;
    await page.route(pattern, (route: any) => route.fulfill({ status: 404, contentType: "text/plain", body: "404 page not found" }));
    await page.goto(`${base}/osu/learn`);
    // Navigate without a document reload, as an already-open tab would.
    await page.evaluate(() => {
      history.pushState(null, "", "/osu/skins?q=whitecat#results");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    await page.getByRole("heading", { name: "This page needs to reload", exact: true }).waitFor();
    assert.equal(documents, 1, "A failure must not cause an automatic reload loop");
    assert.ok(page.url().endsWith("/osu/skins?q=whitecat#results"));
    assert.ok(await page.getByRole("button", { name: "Reload page", exact: true }).isVisible());
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "Recovery overflow");
    await page.unrouteAll({ behavior: "wait" });
    await page.getByRole("button", { name: "Reload page", exact: true }).click();
    await page.getByRole("heading", { name: "Skins", exact: true }).waitFor({ timeout: 30000 });
    assert.ok(page.url().endsWith("/osu/skins?q=whitecat#results"), "Reload lost the destination");
    assert.equal(documents, 2);
    await page.close();
    console.log(`${width}px ${failure}: removed asset produces recovery UI; reload restores route, query and fragment`);
  }
  }
} finally { await browser.close(); }
