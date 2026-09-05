import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import { getPrerenderLearningEntry, getPrerenderLearningIndex, getPrerenderLearningTopic } from "../src/lib/prerender";

test("prerendered knowledge restores the nested defaults used by live API responses", () => {
  const published = JSON.parse(fs.readFileSync(new URL("../../api/internal/coaching/published/knowledge.v1.json", import.meta.url), "utf8"));
  const runtime = globalThis as typeof globalThis & { __AIMMOD_HUB_PRERENDER__?: unknown };
  const previous = runtime.__AIMMOD_HUB_PRERENDER__;
  try {
    for (const entry of published.entries) {
      runtime.__AIMMOD_HUB_PRERENDER__ = { learningEntries: { [entry.id]: { entry } } };
      const normalized = getPrerenderLearningEntry(entry.id)!;
      assert.ok(normalized.entry, entry.id);
      for (const field of ["actions", "why", "avoid", "drills", "sources", "mechanics", "scenarios", "evidence", "contextTags"] as const) {
        assert.ok(Array.isArray(normalized.entry[field]), `${entry.id}.${field}`);
      }
      if (normalized.entry.flaw) assert.ok(Array.isArray(normalized.entry.flaw.telltales));
      assert.ok(Array.isArray(normalized.relatedEntries));
    }
    runtime.__AIMMOD_HUB_PRERENDER__ = { learningIndex: {}, learningTopics: { aim: {} } };
    assert.deepEqual(getPrerenderLearningIndex()?.entries, []);
    assert.deepEqual(getPrerenderLearningTopic("aim")?.entries, []);
  } finally {
    runtime.__AIMMOD_HUB_PRERENDER__ = previous;
  }
});
