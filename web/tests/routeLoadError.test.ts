import assert from "node:assert/strict";
import test from "node:test";
import { isRouteLoadError } from "../src/lib/routeLoadError";

test("recognizes browser module and Vite preload failures", () => {
  for (const message of [
    "Failed to fetch dynamically imported module: https://aimmod.app/assets/old.js",
    "error loading dynamically imported module: https://aimmod.app/assets/old.js",
    "Importing a module script failed.",
    "Unable to preload CSS for /assets/old.css",
    "Loading chunk 45 failed.",
  ]) assert.equal(isRouteLoadError(new TypeError(message)), true, message);
});

test("does not mislabel unrelated exceptions as a deployment", () => {
  for (const error of [null, undefined, {}, new TypeError("t.replace is not a function"), new Error("Network error")]) {
    assert.equal(isRouteLoadError(error), false);
  }
});
