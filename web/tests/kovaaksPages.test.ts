import { test } from "node:test";
import assert from "node:assert/strict";
import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { HelmetProvider } from "../src/lib/helmet";
import { AuthProvider } from "../src/lib/AuthContext";
import { LivePage } from "../src/pages/LivePage";
import { EmptyState } from "../src/components/ui/EmptyState";
import { StatCard } from "../src/components/StatCard";

function render(route: string, element: ReturnType<typeof createElement>) {
  return renderToString(createElement(HelmetProvider, { context: {} },
    createElement(AuthProvider, null, createElement(MemoryRouter, { initialEntries: [route] }, element))));
}

test("live page has a plain heading, a loading state and no companion jargon", () => {
  const html = render("/live", createElement(LivePage));
  assert.match(html, /<h1[^>]*>Live now<\/h1>/);
  assert.match(html, /aria-label="Loading live players"/);
  assert.doesNotMatch(html, /bridge|grinding/i);
});

test("empty states are one line plus an optional action", () => {
  const html = renderToString(createElement(MemoryRouter, null, createElement(EmptyState, { title: "Nobody is playing right now." })));
  assert.match(html, /Nobody is playing right now\./);
  assert.doesNotMatch(html, /<p/);
});

test("stat cards only render context when given", () => {
  assert.doesNotMatch(renderToString(createElement(StatCard, { label: "Runs", value: "12" })), /text-muted-2/);
  assert.match(renderToString(createElement(StatCard, { label: "Runs", value: "12", detail: "3 this week" })), /3 this week/);
});
