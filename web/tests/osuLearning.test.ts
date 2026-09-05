import assert from "node:assert/strict";
import test from "node:test";
import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import type { FilledContext } from "react-helmet-async";
import { Helmet, HelmetProvider } from "../src/lib/helmet";
import { PageSeo } from "../src/components/PageSeo";
import { OsuLearningPage } from "../src/pages/OsuLearningPage";
import content from "../../api/internal/seo/content.json";
import { extractRenderedHead } from "../scripts/rendered-head";

function render(route: string, page = createElement(OsuLearningPage)) {
  const context: { helmet?: FilledContext["helmet"] } = {};
  const body = renderToString(createElement(HelmetProvider, { context },
    createElement(MemoryRouter, { initialEntries: [route] },
      createElement(Routes, null, createElement(Route, { path: "/osu/learn/:slug?", element: page })))));
  const rendered = extractRenderedHead(body);
  return { body: rendered.body, head: rendered.head, scripts: rendered.head.match(/<script\b[^>]*>[\s\S]*?<\/script>/)?.[0] ?? "" };
}

test("sourced osu guides render real content, one H1 and one article schema", () => {
  assert.ok(content.guides.length >= 15);
  assert.equal(new Set(content.guides.map(g => g.slug)).size, content.guides.length);
  for (const guide of content.guides) {
    assert.ok(guide.sections.reduce((length, section) => length + section.body.split(/\s+/).length, 0) >= 120);
    assert.ok(guide.sources.length >= 2);
    for (const source of guide.sources) {
      const url = new URL(source.url);
      assert.equal(url.protocol, "https:");
      assert.ok(["osu.ppy.sh", "www.youtube.com"].includes(url.hostname));
      if (url.hostname === "www.youtube.com") assert.match(url.searchParams.get("v") ?? "", /^[\w-]{11}$/);
    }
    const page = render(`/osu/learn/${guide.slug}`);
    assert.equal((page.body.match(/<h1\b/g) ?? []).length, 1);
    assert.ok(page.body.includes(guide.sections[0].body));
    assert.ok(page.head.includes(`https://aimmod.app/osu/learn/${guide.slug}`));
    assert.ok(page.head.includes('content="article"'));
    assert.equal((page.scripts.match(/application\/ld\+json/g) ?? []).length, 1);
    const schema = JSON.parse(page.scripts.replace(/^.*?>/, "").replace(/<\/script>$/, ""));
    assert.equal(schema.headline, guide.title);
    assert.equal(schema["@type"], "Article");
    assert.deepEqual(schema.citation, guide.sources.map(s => s.url));
  }
});

test("knowledge index links every guide and missing guide is noindex without Article markup", () => {
  const index = render("/osu/learn");
  for (const guide of content.guides) assert.ok(index.body.includes(`/osu/learn/${guide.slug}`));
  assert.equal(index.scripts, "");
  const missing = render("/osu/learn/missing");
  assert.ok(missing.head.includes("noindex, nofollow"));
  assert.equal(missing.scripts, "");
});

test("knowledge offers search, video filtering and section navigation", () => {
  const index = render("/osu/learn");
  assert.ok(index.body.includes('type="search"'));
  assert.ok(index.body.includes("With video resources"));
  const article = render("/osu/learn/aim-misses-and-cursor-control");
  assert.ok(article.body.includes('href="#section-1"'));
  assert.ok(article.body.includes('id="section-1"'));
  assert.ok(article.body.includes("Community coaching"));
});

test("SEO canonical drops queries and trailing slash and private pages emit no schema", () => {
  const page = render("/osu/learn/?q=private", createElement(PageSeo, { title: "Private", description: "Private", noindex: true,
    schema: { "@type": "Article", headline: "Must not appear" } }));
  assert.ok(page.head.includes('href="https://aimmod.app/osu/learn"'));
  assert.ok(!page.head.includes("?q="));
  assert.ok(page.head.includes("noindex, nofollow"));
  assert.equal(page.scripts, "");
});

test("React 19 SSR metadata moves to the head without touching article text", () => {
  const interpolated = renderToString(createElement(Helmet, null, createElement("title", null, "Player", " - ", "Map")));
  assert.ok(interpolated.includes("<title>Player - Map</title>"));
  assert.throws(() => extractRenderedHead('<template data-msg="private stack path"></template>'), /Refusing to publish/);
  const result = extractRenderedHead('<title>Default</title><meta name="robots" content="noindex"><article><h1>Guide</h1><title>Specific</title><meta name="robots" content="index, follow"><script type="application/ld+json">{"@type":"Article"}</script><p>Body stays.</p></article>');
  assert.equal((result.head.match(/<title>/g) ?? []).length, 1);
  assert.ok(result.head.includes("Specific"));
  assert.ok(!result.head.includes("noindex"));
  assert.equal((result.head.match(/application\/ld\+json/g) ?? []).length, 1);
  assert.ok(result.body.includes("<h1>Guide</h1>"));
  assert.ok(!result.body.includes("<meta"));
});
