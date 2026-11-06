import test from "node:test";
import assert from "node:assert/strict";
import { snapshotStatus } from "../src/lib/snapshotStatus";
test("snapshot freshness keeps pending and missing timestamps distinct", () => {
 assert.equal(snapshotStatus("2026-01-01T00:00:00Z", false, Date.parse("2026-01-01T00:03:00Z")), "Updated 3 min ago");
 assert.match(snapshotStatus("", true), /Refreshing/);
 assert.match(snapshotStatus("invalid", false), /Waiting/);
});
