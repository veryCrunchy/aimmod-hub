import assert from "node:assert/strict";
import test from "node:test";
import { readPpGoal, matchesPpGoal } from "../src/lib/ppGoal";

test("PP bounds handle empty, invalid and reversed links", () => {
  assert.deepEqual(readPpGoal(new URLSearchParams()), { min: undefined, max: undefined });
  assert.deepEqual(readPpGoal(new URLSearchParams("ppMin=300&ppMax=100")), { min: 100, max: 300 });
  assert.deepEqual(readPpGoal(new URLSearchParams("ppMin=-1&ppMax=Infinity")), { min: undefined, max: undefined });
});

test("PP goals use selected accuracy PP, exclude pending and failed calculations, and include bounds", () => {
  const goal = { min: 100, max: 200 };
  assert.equal(matchesPpGoal(undefined, goal), false);
  assert.equal(matchesPpGoal({ pp: 150, maxPp: 300, stars: 5, error: "Unavailable" }, goal), false);
  assert.equal(matchesPpGoal({ pp: 100, maxPp: 300, stars: 5 }, goal), true);
  assert.equal(matchesPpGoal({ pp: 200, maxPp: 300, stars: 5 }, goal), true);
  assert.equal(matchesPpGoal({ pp: 250, maxPp: 300, stars: 5 }, goal), false);
  assert.equal(matchesPpGoal(undefined, { min: undefined, max: undefined }), true);
});
