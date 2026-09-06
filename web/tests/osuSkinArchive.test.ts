import { test } from "node:test";
import assert from "node:assert/strict";
import { strToU8, unzipSync, zipSync } from "fflate";
import { prepareSkinArchive } from "../src/lib/osuSkinArchive";

// Synthetic PNG header: tests stop before browser image decoding.
function png(width = 128, height = 128) {
  const bytes = new Uint8Array(24); bytes.set([137, 80, 78, 71]);
  const view = new DataView(bytes.buffer); view.setUint32(16, width); view.setUint32(20, height); return bytes;
}
const archive = (files: Record<string, Uint8Array>) => Uint8Array.from(zipSync(files)).buffer;

test("skin imports preserve custom font paths, HD sprites, animations and hitsounds", async () => {
  const result = unzipSync(new Uint8Array(await prepareSkinArchive(archive({
    "Example/Skin.ini": strToU8("[Fonts]\r\nHitCirclePrefix: Assets/default/default\r\nScorePrefix: digits/score // font\n"),
    "Example/HitCircle@2x.png": png(), "Example/Assets/default/default-1@2x.png": png(),
    "Example/digits/score-2.png": png(), "Example/sliderb0.png": png(),
    "Example/followpoint-0.png": png(), "Example/normal-hitnormal.wav": new Uint8Array([1, 2]),
    "Example/menu-background.png": png(4096, 4096), "Example/private.osr": new Uint8Array([3]),
    "Other/hitcircle.png": png(), "Example/../cursor.png": png(),
  }))));
  assert.deepEqual(Object.keys(result).sort(), ["assets/default/default-1@2x.png", "digits/score-2.png", "followpoint-0.png", "hitcircle@2x.png", "normal-hitnormal.wav", "skin.ini", "sliderb0.png"]);
});

test("skin import refuses ambiguous settings, absent gameplay, duplicate paths and oversized decoded images", async () => {
  await assert.rejects(prepareSkinArchive(archive({ "a/skin.ini": strToU8(""), "b/skin.ini": strToU8("") })), /skin.ini/);
  await assert.rejects(prepareSkinArchive(archive({ "skin.ini": strToU8(""), "menu.png": png() })), /gameplay artwork/);
  await assert.rejects(prepareSkinArchive(archive({ "skin.ini": strToU8(""), "cursor.png": png(9999) })), /size limit/);
  await assert.rejects(prepareSkinArchive(archive({ "skin.ini": strToU8(""), "cursor.png": png(), "CURSOR.png": png() })), /duplicate/);
  await assert.rejects(prepareSkinArchive(archive({ "skin.ini": strToU8(""), "cursor.png": new Uint8Array([0]) })), /invalid image/);
});

test("compressed oversized gameplay assets are rejected before decompression", async () => {
  await assert.rejects(prepareSkinArchive(archive({ "skin.ini": strToU8(""), "cursor.png": new Uint8Array(9 * 1024 * 1024) })), /size limit/);
});
