import assert from "node:assert/strict";
import test from "node:test";
import { formatOsuAccuracy, formatOsuDuration, formatOsuMods } from "../src/lib/osuCommunity";

test("formats normalized osu score values", () => {
  assert.equal(formatOsuAccuracy(0.98765), "98.77%");
  assert.equal(formatOsuAccuracy(4), "100.00%");
  assert.equal(formatOsuDuration(125_000), "2:05");
  assert.equal(formatOsuMods([]), "NM");
  assert.equal(formatOsuMods(["HD", "HR"]), "HDHR");
});
