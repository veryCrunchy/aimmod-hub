import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import type { Plugin } from "vite";

// The upstream npm release supplies Node bindings. Only adapt its loader;
// all difficulty and performance code remains the pinned upstream WASM.
export function rosuBrowser(): Plugin {
  const require = createRequire(import.meta.url);
  const entry = require.resolve("rosu-pp-js");
  const wasm = require.resolve("rosu-pp-js/rosu_pp_js_bg.wasm").replaceAll("\\", "/");
  return {
    name: "rosu-browser-loader",
    resolveId(id) { if (id === "virtual:rosu-browser") return "\0virtual:rosu-browser"; },
    load(id) {
      if (id !== "\0virtual:rosu-browser") return;
      const source = readFileSync(entry, "utf8");
      const util = "const { TextDecoder, TextEncoder, inspect } = require(`util`);";
      const start = source.indexOf("const path = require('path')");
      if (!source.includes(util) || start < 0) throw new Error("rosu-pp-js loader changed; review browser adapter");
      const body = source.slice(0, start).replace(util, "const inspect = { custom: Symbol.for('nodejs.util.inspect.custom') };");
      return `import wasmUrl from ${JSON.stringify(wasm + "?url")};
        let ready;
        export function loadRosu() { return ready ??= initialize().catch(error => { ready = undefined; throw error; }); }
        async function initialize() {
          const module = { exports: {} };
          ${body}
          const response = await fetch(wasmUrl);
          if (!response.ok) throw new Error('PP calculator could not load');
          const bytes = await response.arrayBuffer();
          const result = await WebAssembly.instantiate(bytes, imports);
          wasm = result.instance.exports;
          return module.exports;
        }`;
    },
  };
}
