import assert from "node:assert/strict";
import test from "node:test";
import { createPublicQuery } from "../src/lib/publicQuery";

test("public reads share concurrent work and expire", async () => {
  let now = 100, calls = 0;
  const query = createPublicQuery<number>(10, () => now);
  const load = async () => ++calls;
  assert.deepEqual(await Promise.all([query("a", load), query("a", load)]), [1, 1]);
  assert.equal(await query("a", load), 1);
  now = 111;
  assert.equal(await query("a", load), 2);
  assert.equal(await query("b", load), 3);
});

test("failed reads retry and uncached score reads remain fresh", async () => {
  const query = createPublicQuery<number>();
  await assert.rejects(query("a", async () => { throw Error("unavailable"); }));
  assert.equal(await query("a", async () => 1), 1);
  assert.equal(await query("a", async () => 2), 2);
});

test("public cache retains at most 64 settled entries", async () => {
  const query = createPublicQuery<number>(1000);
  for (let i = 0; i < 65; i++) await query(String(i), async () => i);
  assert.equal(await query("0", async () => 99), 99);
});
