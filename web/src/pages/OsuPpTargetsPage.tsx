import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { PageSeo } from "../components/PageSeo";
import { Button } from "../components/ui/Button";
import { beatmapLinks, mediaUrl, osuClient } from "../lib/osuCatalog";
import { API_BASE_URL } from "../lib/config";
import { BeatmapDifficulty, Provider, Ruleset } from "../gen/aimmod/osu/v1/osu_pb";
import { RangeSlider } from "./OsuCatalogPage";
import { browserPpCache, candidateKey, ppMods, readPpSettings, validChecksum, validPpResult, type PpResult } from "../lib/ppTargetCache";
import "./osuCatalog.css";

type Candidate = { map: BeatmapDifficulty; result?: PpResult };

export function OsuPpTargetsPage() {
  const [params, setParams] = useSearchParams();
  const { query, low, high, accuracy, mods, lazer, sort } = readPpSettings(params);
  const updateParam = (key: string, value: string) => setParams(current => {
    const next = new URLSearchParams(current); next.set(key, value); return next;
  }, { replace: true });
  const [rows, setRows] = useState<Candidate[]>([]);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState(0);
  const [attempt, setAttempt] = useState(0);
  const generation = useRef(0);

  useEffect(() => {
    const revision = ++generation.current;
    const controller = new AbortController();
    const active = () => !controller.signal.aborted && revision === generation.current;
    const cache = browserPpCache();
    const settings = { query, low, high, accuracy, mods, lazer, sort: "pp" };
    const searchKey = candidateKey(settings);
    const restored = cache.getCandidates(searchKey);
    let worker: Worker | undefined;
    setBusy(true); setError(""); setRows([]); setProgress(0); setStatus("Finding difficulties");
    const run = async () => {
      try {
        let maps = restored;
        if (!maps) {
          const response = await osuClient.searchBeatmapItems({ query, providers: [Provider.OSU_OFFICIAL], filters: { ruleset: Ruleset.OSU, status: "ranked", stars: { minimum: low ? Number(low) : undefined, maximum: high ? Number(high) : undefined } }, sort: "plays_desc" }, { signal: controller.signal });
          if (!active()) return;
          if (response.providers.some(provider => !provider.available)) throw new Error("Beatmap search is unavailable. Please try again.");
          maps = [];
          for (const item of response.items.slice(0, 12)) {
            if (!active()) return;
            setStatus(`Loading ${item.title || "map"} difficulties`);
            const detail = await osuClient.getBeatmapItem({ provider: Provider.OSU_OFFICIAL, sourceId: item.sourceId }, { signal: controller.signal });
            if (!active()) return;
            if (!detail.item || detail.provider?.available === false) throw new Error("Map details are unavailable. Please try again.");
            for (const map of detail.item.difficulties) {
              if (map.ruleset === Ruleset.OSU && (!low || map.stars >= Number(low)) && (!high || map.stars <= Number(high)) && !maps.some(existing => existing.beatmapId === map.beatmapId)) maps.push(map);
            }
          }
          if (!active()) return;
          cache.setCandidates(searchKey, maps);
        }
        if (!active()) return;
        const candidates: Candidate[] = maps.map(map => ({ map, result: cache.getResult(map, settings) }));
        setRows([...candidates]);
        setProgress(candidates.filter(candidate => candidate.result).length);
        for (let index = 0; index < candidates.length; index++) {
          if (!active()) return;
          const candidate = candidates[index];
          if (candidate.result) continue;
          setStatus(`Calculating difficulty ${index + 1} of ${candidates.length}`);
          let result: PpResult;
          if (!validChecksum(candidate.map.checksum)) {
            result = { pp: 0, maxPp: 0, stars: candidate.map.stars, error: "Map checksum unavailable. Refresh the search to verify this difficulty." };
          } else {
            worker ??= new Worker(new URL("../lib/ppTargetWorker.ts", import.meta.url), { type: "module" });
            result = await new Promise<PpResult>((resolve, reject) => {
              const cleanup = () => {
                clearTimeout(timeout); controller.signal.removeEventListener("abort", abort);
                worker!.onmessage = null; worker!.onerror = null;
              };
              const abort = () => { cleanup(); reject(new DOMException("Cancelled", "AbortError")); };
              const timeout = setTimeout(() => { cleanup(); reject(new Error("PP calculation timed out. Try a different search.")); }, 25000);
              worker!.onmessage = event => {
                if (event.data?.id !== index) return;
                cleanup();
                if (!validPpResult(event.data) && typeof event.data?.error !== "string") reject(new Error("PP calculator returned an invalid result."));
                else resolve(event.data);
              };
              worker!.onerror = () => { cleanup(); reject(new Error("PP calculator could not start.")); };
              controller.signal.addEventListener("abort", abort, { once: true });
              const checksum = candidate.map.checksum;
              try { worker!.postMessage({ id: index, url: `${API_BASE_URL}/api/osu/v1/playback/beatmaps/${candidate.map.beatmapId}/file?checksum=${encodeURIComponent(checksum)}`, checksum, accuracy, mods, lazer }); }
              catch (failure) { cleanup(); reject(failure); }
            });
          }
          if (!active()) return;
          cache.setResult(candidate.map, settings, result);
          if (result.error) cache.deleteCandidates(searchKey);
          candidate.result = result;
          setRows([...candidates]); setProgress(candidates.filter(candidate => candidate.result).length);
        }
      } catch (failure) { if (active()) setError(failure instanceof Error ? failure.message : "Search unavailable"); }
      finally { worker?.terminate(); if (active()) { setBusy(false); setStatus(""); } }
    };
    const timer = restored === undefined ? setTimeout(() => void run(), 450) : undefined;
    if (restored !== undefined) void run();
    return () => { clearTimeout(timer); controller.abort(); worker?.terminate(); };
  }, [query, low, high, accuracy, mods, lazer, attempt]);

  const ordered = [...rows].sort((a, b) => sort === "stars" ? (b.result?.stars ?? b.map.stars) - (a.result?.stars ?? a.map.stars) : (sort === "max" ? b.result?.maxPp ?? -1 : b.result?.pp ?? -1) - (sort === "max" ? a.result?.maxPp ?? -1 : a.result?.pp ?? -1));
  return <div className="py-5">
    <PageSeo title="osu! PP Targets · AimMod Hub" description="Find ranked osu! difficulties and compare full-combo PP at your chosen accuracy and mods." />
    <h1 className="text-3xl">PP targets</h1>
    <p className="text-muted mt-3">Full-combo PP at your selected accuracy. Personal skill compatibility and recommendations are available in AimMod.</p>
    <div className="hub-filters catalog-filters my-5">
      <label>Search<input type="search" value={query} onChange={event => updateParam("q", event.target.value)} placeholder="Title, artist, mapper" maxLength={256} /></label>
      <RangeSlider name="stars" label="Stars (no mods)" limit={10} step={0.1} minimum={low} maximum={high} onChange={(endpoint, value) => updateParam(endpoint === "Min" ? "min" : "max", value)} />
      <label>Accuracy: {accuracy.toFixed(1)}%<input type="range" min={80} max={100} step={0.1} value={accuracy} onChange={event => updateParam("acc", event.target.value)} /></label>
      <label>Mods<select value={mods} onChange={event => updateParam("mods", event.target.value)}>{ppMods.map(mod => <option key={mod}>{mod}</option>)}</select></label>
      <label>Scoring<select value={String(lazer)} onChange={event => updateParam("scoring", event.target.value === "true" ? "lazer" : "stable")}><option value="true">Lazer</option><option value="false">Stable</option></select></label>
      <label>Sort<select value={sort} onChange={event => updateParam("sort", event.target.value)}><option value="pp">PP at accuracy</option><option value="max">SS PP</option><option value="stars">Modded stars</option></select></label>
    </div>
    {busy && <div role="status" className="py-4"><p>{status}</p>{rows.length > 0 && <progress className="w-full mt-2" value={progress} max={rows.length} />}</div>}
    {error && <div role="alert" className="py-4"><p className="mb-3">{error}</p><Button onClick={() => setAttempt(value => value + 1)}>Try again</Button></div>}
    {!busy && <div className="mb-4"><Button onClick={() => { browserPpCache().deleteCandidates(candidateKey({ query, low, high })); setAttempt(value => value + 1); }}>Refresh results</Button></div>}
    {!busy && !error && !rows.length && <p className="py-8">No matching difficulties. Broaden the star range or change your search.</p>}
    <p className="text-muted text-sm mb-4">{rows.length} difficulties from up to 12 popular matching sets. Values assume no misses and full combo; they are not predicted results or profile PP gains.</p>
    <div className="divide-y divide-line">{ordered.map(({ map, result }) => {
      const links = beatmapLinks(map.beatmapsetId);
      return <article key={map.beatmapId} className="py-4 flex flex-wrap items-center gap-4">
        {mediaUrl(map.coverUrl) && <img src={mediaUrl(map.coverUrl)!} alt="" className="w-24 h-16 object-cover rounded" loading="lazy" />}
        <div className="flex-1 min-w-0 basis-60"><h2 className="font-semibold break-words">{map.title} [{map.name}]</h2><p className="text-sm text-muted">{map.artist} · {map.creator}</p><p className="text-sm text-muted">{(result?.stars ?? map.stars).toFixed(2)} stars · {map.bpm} BPM · {Math.floor(map.lengthSeconds / 60)}:{String(map.lengthSeconds % 60).padStart(2, "0")} · {mods}</p></div>
        <dl className="flex gap-6"><div><dt className="text-xs text-muted">{accuracy.toFixed(1)}% FC</dt><dd className="text-xl text-cyan">{result?.error ? "Unavailable" : result ? `${Math.round(result.pp)}pp` : "Pending"}</dd></div><div><dt className="text-xs text-muted">SS</dt><dd className="text-xl">{result && !result.error ? `${Math.round(result.maxPp)}pp` : "—"}</dd></div></dl>
        {links && <div className="flex flex-wrap gap-2"><Button href={links.aimmod}>Open in AimMod</Button><Button href={links.osu}>Open in osu!</Button></div>}
        {result?.error && <p className="basis-full text-sm text-muted">{result.error}</p>}
      </article>;
    })}</div>
  </div>;
}
