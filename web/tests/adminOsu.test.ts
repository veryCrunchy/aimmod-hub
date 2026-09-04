import assert from "node:assert/strict";
import { test } from "node:test";
import { adminScoreParams, fetchOsuAdmin } from "../src/pages/adminOsu";

test("score drilldowns preserve exact identity independently of display labels", () => {
  const player = adminScoreParams("same name", "private", "uploaded", 25, { label: "Player", userId: 123 });
  assert.equal(player.get("userId"), "123");
  assert.equal(player.get("q"), "same name");
  assert.equal(player.get("offset"), "25");
  assert.equal(player.has("difficultyKey"), false);
  const map = adminScoreParams("", "", "", 0, { label: "Duplicate map title", difficultyKey: "local:abc&next=123" });
  assert.equal(new URLSearchParams(map.toString()).get("difficultyKey"), "local:abc&next=123");
  assert.equal(map.has("next"), false);
  assert.equal(map.has("userId"), false);
  assert.equal(adminScoreParams("", "", "", 0).has("userId"), false);
});

test("admin reads send session credentials, disable caching and conceal upstream errors", async () => {
  const original = globalThis.fetch;
  const controller = new AbortController();
  try {
    globalThis.fetch = async (_input, options) => {
      assert.equal(options?.credentials, "include");
      assert.equal(options?.cache, "no-store");
      assert.equal(options?.signal, controller.signal);
      return new Response("SECRET DATABASE ERROR", { status: 500 });
    };
    await assert.rejects(fetchOsuAdmin("players", controller.signal), error => error instanceof Error && !error.message.includes("SECRET"));
    globalThis.fetch = async () => new Response("forbidden", { status: 403 });
    await assert.rejects(fetchOsuAdmin("beatmaps", controller.signal), /Admin access is required/);
  } finally { globalThis.fetch = original; }
});
