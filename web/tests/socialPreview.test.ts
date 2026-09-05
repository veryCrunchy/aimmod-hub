import assert from "node:assert/strict";
import test from "node:test";
import { brandPreviewImage, socialPreviewImage } from "../src/lib/socialPreview";

test("social previews identify the shared page rather than search filters", () => {
  for (const path of ["/osu/replays/example", "/osu/profiles/player", "/osu/learn/performance-points-and-difficulty", "/learn/topic", "/scenarios/aim", "/osu/skins"]) {
    const image = new URL(socialPreviewImage(path));
    assert.equal(image.pathname, "/social-preview.png");
    assert.equal(image.searchParams.get("path"), path);
    assert.equal(image.searchParams.get("v"), "1");
  }
  assert.equal(socialPreviewImage("/osu/beatmaps/?q=anything"), socialPreviewImage("/osu/beatmaps"));
  assert.notEqual(socialPreviewImage("/osu/skins"), socialPreviewImage("/osu/beatmaps"));
});

test("restricted or unknown pages do not request identifying social images", () => {
  for (const path of ["/", "/app", "/admin", "/account", "/search", "/unknown"]) assert.equal(socialPreviewImage(path), brandPreviewImage);
  assert.equal(socialPreviewImage("/osu/replays/unlisted", true), brandPreviewImage);
});
