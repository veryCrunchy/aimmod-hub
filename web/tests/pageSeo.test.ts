import assert from "node:assert/strict";
import test from "node:test";
import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { HelmetProvider } from "../src/lib/helmet";
import { PageSeo, RouteSeo } from "../src/components/PageSeo";
import { extractRenderedHead } from "../scripts/rendered-head";

function head(route: string, ownPage = false, noindex = false) {
  const component = ownPage
    ? createElement(PageSeo, { title: "Page", description: "Description", noindex, schema: { "@type": "Article" } })
    : createElement(RouteSeo);
  return extractRenderedHead(renderToString(createElement(HelmetProvider, { context: {} },
    createElement(MemoryRouter, { initialEntries: [route] }, component)))).head;
}

test("private routes cannot accidentally opt in to indexing or schema", () => {
  for (const route of ["/account", "/admin/coaching", "/auth/callback", "/link-device", "/search"]) {
    for (const ownPage of [false, true]) {
      const result = head(route, ownPage);
      assert.ok(result.includes('content="noindex, nofollow"'), route);
      assert.ok(!result.includes("application/ld+json"), route);
    }
  }
});

test("PP targets page owns a query-free canonical and social metadata", () => {
  assert.equal(head("/osu/pp-targets"), "");
  const result = head("/osu/pp-targets/?accuracy=99#results", true);
  assert.ok(result.includes('href="https://aimmod.app/osu/pp-targets"'));
  assert.ok(result.includes('content="index, follow"'));
  assert.ok(result.includes('property="og:image"'));
  assert.ok(result.includes('name="twitter:description"'));
  assert.ok(!result.includes("accuracy=99"));
});

test("public catalog supplements complete social metadata", () => {
  const result = head("/osu/beatmaps");
  for (const key of ["og:type", "og:image:width", "og:image:height", "og:image:alt", "twitter:title", "twitter:description", "twitter:image:alt"]) {
    assert.ok(result.includes(`="${key}"`), key);
  }
});

test("replay visibility stays page-owned and unknown routes are noindex", () => {
  assert.equal(head("/osu/replays/shared"), "");
  assert.ok(head("/osu/replays/shared", true, true).includes('content="noindex, nofollow"'));
  assert.ok(head("/osu/replays/shared", true).includes('content="index, follow"'));
  assert.ok(head("/not-a-page").includes('content="noindex, nofollow"'));
  assert.ok(head("/profiles/example/invalid").includes('content="noindex, nofollow"'));
  assert.ok(head("/profiles/example/benchmarks/123").includes('content="index, follow"'));
});
