import { test } from "node:test";
import assert from "node:assert/strict";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { OsuService } from "../src/gen/aimmod/osu/v1/osu_connect";
import { Provider, SearchBeatmapItemsRequest, SkinProvider } from "../src/gen/aimmod/osu/v1/osu_pb";
import { beatmapLinks, CatalogCache, catalogRequest, mediaUrl, numberRange, skinLinks } from "../src/lib/osuCatalog";

test("native handoffs accept supported IDs and reject injected paths or oversized IDs", () => {
  assert.equal(beatmapLinks("2147483647")?.osu, "osu://dl/2147483647");
  assert.equal(beatmapLinks("123")?.aimmod, "aimmod-osu://beatmapsets/123");
  for (const id of ["0", "-1", "01", " 12", "1/2", "1?x=y", "1#x", "2147483648", "999999999999999999999"]) assert.equal(beatmapLinks(id), null);
  assert.equal(skinLinks(SkinProvider.OSU_SKINS, "3sXe0RR")?.aimmod, "aimmod-osu://skins/osuskins/3sXe0RR");
  assert.equal(skinLinks(SkinProvider.OSU_SKINS, "3sXe0RR")?.source, "https://osuskins.net/skin/3sXe0RR");
  assert.equal(skinLinks(SkinProvider.OSUCK, "123-example")?.aimmod, "aimmod-osu://skins/osuck/123-example");
  for (const id of ["../x", "a/b", "abc?d", "a%2fb", "a#b", "", "a".repeat(129)]) assert.equal(skinLinks(SkinProvider.OSUCK, id), null);
  assert.equal(skinLinks(SkinProvider.UNSPECIFIED, "123"), null);
});

test("media accepts HTTPS and provider protocol-relative URLs only", () => {
  assert.equal(mediaUrl("//b.ppy.sh/preview/1.mp3"), "https://b.ppy.sh/preview/1.mp3");
  for (const url of ["javascript:alert(1)", "data:image/png;base64,a", "http://example.org/x", "https://a:b@example.org/x", "/relative", ""]) assert.equal(mediaUrl(url), undefined);
});

test("range filters preserve zero and optional endpoints and reject invalid ranges", () => {
  assert.equal(numberRange("", ""), undefined);
  assert.equal(numberRange("0", "5")?.minimum, 0);
  assert.equal(numberRange("", "5")?.minimum, undefined);
  assert.equal(numberRange("4.5", "")?.maximum, undefined);
  for (const [min, max] of [["6", "5"], ["-1", ""], ["NaN", "2"], ["", "Infinity"]]) assert.throws(() => numberRange(min, max));
});

test("cache expires and evicts entries without leaking one query into another", () => {
  let now = 0;
  const cache = new CatalogCache(2, 10, () => now);
  cache.set("a", 1); cache.set("b", 2); cache.set("c", 3);
  assert.equal(cache.get("a"), undefined);
  assert.equal(cache.get("b"), 2);
  now = 10;
  assert.equal(cache.get("b"), undefined);
  cache.delete("c");
  assert.equal(cache.get("c"), undefined);
});

test("aborted requests cannot populate cache even when the transport ignores cancellation", async () => {
  const controller = new AbortController();
  const cache = new CatalogCache();
  let finish!: (value: string) => void;
  const promise = catalogRequest("old", controller.signal, () => new Promise<string>(resolve => { finish = resolve; }), cache);
  controller.abort(); finish("stale");
  await assert.rejects(promise, { name: "AbortError" });
  assert.equal(cache.get("old"), undefined);
});

test("successful results are reused; partial provider failures and errors remain retryable", async () => {
  const cache = new CatalogCache();
  const signal = new AbortController().signal;
  let calls = 0;
  const success = async () => { calls++; return { providers: [{ available: true }], items: ["result"] }; };
  await catalogRequest("ok", signal, success, cache);
  await catalogRequest("ok", signal, success, cache);
  assert.equal(calls, 1);
  const partial = async () => ({ providers: [{ available: true }, { available: false }], items: ["available provider result"] });
  assert.equal((await catalogRequest("partial", signal, partial, cache)).items.length, 1);
  assert.equal(cache.get("partial"), undefined);
  await assert.rejects(catalogRequest("error", signal, async () => { throw new Error("offline"); }, cache));
  assert.equal(cache.get("error"), undefined);
});

test("generated Connect boundary carries opaque cursors and optional numeric filters unchanged", async () => {
  const bodies: Record<string, unknown>[] = [];
  const client = createClient(OsuService, createConnectTransport({ baseUrl: "https://catalog.test", useBinaryFormat: false, fetch: async (_input, init) => {
    bodies.push(JSON.parse(await new Response(init?.body).text()) as Record<string, unknown>);
    return new Response(JSON.stringify({ items: [], nextPageTokens: [{ provider: "PROVIDER_OSU_OFFICIAL", pageToken: "opaque+/=cursor" }], providers: [{ provider: "PROVIDER_OSU_OFFICIAL", available: true }] }), { headers: { "content-type": "application/json" } });
  } }));
  const request = new SearchBeatmapItemsRequest({ providers: [Provider.OSU_OFFICIAL], filters: { stars: numberRange("0", "5") } });
  const first = await client.searchBeatmapItems(request);
  await client.searchBeatmapItems({ ...request, pageTokens: first.nextPageTokens });
  assert.deepEqual(bodies[1].pageTokens, [{ provider: "PROVIDER_OSU_OFFICIAL", pageToken: "opaque+/=cursor" }]);
  assert.deepEqual((bodies[0].filters as Record<string, unknown>).stars, { minimum: 0, maximum: 5 });
});
