import { test } from "node:test";
import assert from "node:assert/strict";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { OsuReplayRow } from "../src/components/OsuReplayRow";
import type { OsuSharedReplay } from "../src/lib/osuCommunity";

const sample = { source: "official", shareId: "example-score", officialScoreId: "42", beatmapId: 123, osuUserId: 456, osuUsername: "Example Player", artist: "Example", title: "Synthetic map", difficulty: "Test", creator: "Example Mapper", starRating: 7, accuracy: 1, passed: false, maxCombo: 1, count300: 1, count100: 0, count50: 0, countMiss: 0, mods: ["DT"], performancePoints: .2, ppSource: "calculated" } as OsuSharedReplay;
const render = (patch: Partial<OsuSharedReplay>) => renderToStaticMarkup(createElement(MemoryRouter, null, createElement(OsuReplayRow, { replay: { ...sample, ...patch } })));

test("recent rows explain partial scores and use calculated modded difficulty", () => {
  const html = render({ calculatedStarRating: 9, mapObjectCount: 500 });
  assert.match(html, /9\.00/);
  assert.doesNotMatch(html, /7\.00/);
  assert.match(html, /partial play PP/);
  assert.match(html, /Failed \/ stopped/);
  assert.match(html, /1 objects judged \/ 500/);
  assert.match(html, /&lt;1pp/);
});

test("base stars are identified and empty attempts do not claim 100 percent accuracy", () => {
  const html = render({ count300: 0, maxCombo: 0, performancePoints: 0 });
  assert.match(html, /base difficulty/);
  assert.doesNotMatch(html, /100\.00%/);
  assert.match(html, /0pp/);
});
