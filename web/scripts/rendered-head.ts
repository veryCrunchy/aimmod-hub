import { parseFragment, serialize, serializeOuter, type DefaultTreeAdapterMap } from "parse5";

// React 19 emits native metadata during SSR; HelmetProvider.context is intentionally empty.
export function extractRenderedHead(markup: string) {
  const fragment = parseFragment(markup);
  const owned = new Map<string, string>();
  function visit(parent: DefaultTreeAdapterMap["parentNode"]) {
    for (const node of [...parent.childNodes]) {
      if (!("tagName" in node)) continue;
      const attrs = Object.fromEntries(node.attrs.map(attr => [attr.name, attr.value]));
      if (node.tagName === "template" && ("data-msg" in attrs || "data-dgst" in attrs)) {
        throw new Error("Prerender failed: React emitted a server-rendering error boundary. Refusing to publish fallback markup.");
      }
      let key: string | undefined;
      if (node.tagName === "title") key = "title";
      if (node.tagName === "meta") key = `meta:${attrs.name ?? attrs.property}`;
      if (node.tagName === "link" && attrs.rel === "canonical") key = "canonical";
      if (node.tagName === "script" && attrs.type === "application/ld+json") key = "schema";
      if (key) {
        owned.set(key, serializeOuter(node));
        parent.childNodes.splice(parent.childNodes.indexOf(node), 1);
      } else visit(node);
    }
  }
  visit(fragment);
  return { body: serialize(fragment), head: [...owned.values()].join("") };
}
