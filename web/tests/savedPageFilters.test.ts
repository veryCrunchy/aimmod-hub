import assert from "node:assert/strict";
import test from "node:test";
import { loadPageFilters, savePageFilters, filterPreferenceQuery, filterChoice, updateFilterQuery, supportsSavedFilters } from "../src/lib/savedPageFilters";
test("all migrated controls and catalog ranges survive without pagination or credentials", () => {
  const names = ["q", "direction", "view", "tab", "scenarioType", "replay", "sort", "starsMin", "starsMax", "bpmMin", "bpmMax", "lengthSecondsMin", "lengthSecondsMax", "approachRateMin", "approachRateMax", "circleSizeMin", "circleSizeMax", "overallDifficultyMin", "overallDifficultyMax"];
  const query = new URLSearchParams(names.map(name => [name, "test"]));
  const expected = query.toString();
  for (const name of ["page", "cursor", "limit", "visible", "token"]) query.set(name, "secret");
  assert.equal(filterPreferenceQuery(query.toString()), expected);
});
test("learning, live and scenario preferences are isolated by exact public path", () => {
  let value: string | null = null;
  const storage = { getItem: () => value, setItem: (_key: string, next: string) => { value = next; } };
  for (const path of ["/learn", "/live", "/scenarios/one", "/scenarios/two"]) {
    assert.equal(supportsSavedFilters(path), true);
    savePageFilters(storage, path, new URLSearchParams({ q: path }).toString());
  }
  for (const path of ["/learn", "/live", "/scenarios/one", "/scenarios/two"]) {
    assert.equal(new URLSearchParams(loadPageFilters(storage, path)).get("q"), path);
  }
  for (const path of ["/admin", "/admin/scenarios/one", "/scenarios/one/private", "/auth/callback"]) assert.equal(supportsSavedFilters(path), false);
});
test("URL choices validate defaults and sort updates preserve unrelated filters atomically", () => {
  assert.equal(filterChoice("top", ["records", "top"] as const, "records"), "top");
  assert.equal(filterChoice("invalid", ["recent", "leaderboard"] as const, "recent"), "recent");
  assert.equal(filterChoice(null, ["all", "video", "mouse"] as const, "all"), "all");
  const original = new URLSearchParams("q=aim&view=replays&tab=top&scenarioType=Tracking");
  const sorted = updateFilterQuery(original, { sort: "name", direction: "asc" });
  assert.equal(sorted.get("sort"), "name");
  assert.equal(sorted.get("direction"), "asc");
  assert.equal(sorted.get("scenarioType"), "Tracking");
  assert.equal(original.has("sort"), false);
  const cleared = updateFilterQuery(sorted, { q: null });
  assert.equal(cleared.has("q"), false);
  assert.equal(cleared.get("view"), "replays");
  assert.equal(cleared.get("tab"), "top");
});
test("bounded preferences never truncate percent-encoded values", () => {
  const query = new URLSearchParams();
  for (const name of ["q", "creator", "player", "topic", "sort", "view"]) query.set(name, "\u65e5".repeat(256));
  const saved = filterPreferenceQuery(query.toString());
  assert.ok(saved.length <= 4096);
  for (const value of new URLSearchParams(saved).values()) assert.equal(value, "\u65e5".repeat(256));
});
test("filter preferences are isolated per page and clear explicitly", () => {
  let value: string | null = null;
  const storage = { getItem: () => value, setItem: (_key: string, next: string) => { value = next; } };
  savePageFilters(storage, "/osu/skins", "q=whitecat&mode=osu");
  savePageFilters(storage, "/osu/pp-targets", "acc=99&mods=HD");
  assert.equal(loadPageFilters(storage, "/osu/skins"), "q=whitecat&mode=osu");
  assert.equal(loadPageFilters(storage, "/osu/pp-targets"), "acc=99&mods=HD");
  savePageFilters(storage, "/osu/skins", "");
  assert.equal(loadPageFilters(storage, "/osu/skins"), "");
});
test("auth tokens and private pages never enter browsing preferences", () => {
  assert.equal(filterPreferenceQuery("q=map&code=secret&token=secret&access_token=secret"), "q=map");
  const storage = { getItem: () => null, setItem: () => { throw new Error("must not write"); } };
  savePageFilters(storage, "/auth/callback", "q=secret");
  assert.equal(loadPageFilters(storage, "/account"), "");
});
test("disabled and malformed storage cannot break filters", () => {
  assert.equal(loadPageFilters({ getItem: () => "bad-json", setItem: () => {} }, "/osu/skins"), "");
  const unavailable = { getItem: () => { throw new Error("denied"); }, setItem: () => { throw new Error("full"); } };
  assert.doesNotThrow(() => savePageFilters(unavailable, "/osu/skins", "q=test"));
});
