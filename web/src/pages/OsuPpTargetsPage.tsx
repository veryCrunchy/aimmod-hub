import { ArrowUpRight, Search, SlidersHorizontal, RotateCw } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { BeatmapCard } from "../components/BeatmapCard";
import { PageSeo } from "../components/PageSeo";
import { Button } from "../components/ui/Button";
import { osuClient } from "../lib/osuCatalog";
import { API_BASE_URL } from "../lib/config";
import { BeatmapDifficulty, Provider, Ruleset } from "../gen/aimmod/osu/v1/osu_pb";
import { RangeSlider } from "./OsuCatalogPage";
import { browserPpCache, candidateKey, ppMods, readPpSettings, validChecksum, validPpResult, type PpResult } from "../lib/ppTargetCache";
import "./osuCatalog.css";
import "./ppFinder.css";
import { readPpGoal, matchesPpGoal } from "../lib/ppGoal";

type Candidate = { map: BeatmapDifficulty; result?: PpResult };

export function OsuPpTargetsPage() {
  const [params, setParams] = useSearchParams();
  const { query, low, high, accuracy, mods, lazer, sort } = readPpSettings(params);
  const goal = readPpGoal(params);
  const updateParam = (key: string, value: string) => setParams(current => {
    const next = new URLSearchParams(current); next.set(key, value); return next;
  }, { replace: true });
  const [rows, setRows] = useState<Candidate[]>([]);
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
    const workers: (Worker | undefined)[] = [];
    const candidates: Candidate[] = [];
    let detailsFailed = false;
    const publish = () => {
      if (!active()) return;
      setRows([...candidates]);
      setProgress(candidates.filter(candidate => candidate.result).length);
    };
    setBusy(true); setError(""); setRows([]); setProgress(0);

    async function calculate(candidate: Candidate, lane: number) {
      if (!active() || candidate.result) return;
      let result: PpResult;
      if (!validChecksum(candidate.map.checksum)) {
        result = { pp: 0, maxPp: 0, stars: candidate.map.stars, error: "Map details need refreshing." };
      } else {
        try {
          const worker = workers[lane] ??= new Worker(new URL("../lib/ppTargetWorker.ts", import.meta.url), { type: "module" });
          result = await new Promise<PpResult>((resolve, reject) => {
            const cleanup = () => { clearTimeout(timeout); controller.signal.removeEventListener("abort", abort); worker.onmessage = null; worker.onerror = null; };
            const abort = () => { cleanup(); reject(new DOMException("Cancelled", "AbortError")); };
            const timeout = setTimeout(() => { cleanup(); worker.terminate(); workers[lane] = undefined; reject(new Error("PP calculation timed out.")); }, 45000);
            worker.onmessage = event => {
              cleanup();
              if (!validPpResult(event.data) && typeof event.data?.error !== "string") reject(new Error("PP unavailable."));
              else resolve(event.data);
            };
            worker.onerror = () => { cleanup(); worker.terminate(); workers[lane] = undefined; reject(new Error("PP unavailable.")); };
            controller.signal.addEventListener("abort", abort, { once: true });
            const checksum = candidate.map.checksum;
            try { worker.postMessage({ id: 0, url: `${API_BASE_URL}/api/osu/v1/playback/beatmaps/${candidate.map.beatmapId}/file?checksum=${encodeURIComponent(checksum)}`, checksum, accuracy, mods, lazer }); }
            catch (failure) { cleanup(); reject(failure); }
          });
        } catch (failure) {
          if (!active()) return;
          result = { pp: 0, maxPp: 0, stars: candidate.map.stars, error: failure instanceof Error ? failure.message : "PP unavailable." };
        }
      }
      if (!active()) return;
      cache.setResult(candidate.map, settings, result);
      if (result.error) cache.deleteCandidates(searchKey);
      candidate.result = result;
      publish();
    }

    async function runLanes<T>(items: T[], task: (item: T, lane: number) => Promise<void>) {
      let next = 0;
      await Promise.all(Array.from({ length: Math.min(3, items.length) }, async (_, lane) => {
        while (active() && next < items.length) await task(items[next++], lane);
      }));
    }

    const run = async () => {
      try {
        if (restored) {
          candidates.push(...restored.map(map => ({ map, result: cache.getResult(map, settings) })));
          publish();
        } else {
          const response = await osuClient.searchBeatmapItems({ query, providers: [Provider.OSU_OFFICIAL], filters: { ruleset: Ruleset.OSU, status: "ranked", stars: { minimum: low ? Number(low) : undefined, maximum: high ? Number(high) : undefined } }, sort: "plays_desc" }, { signal: controller.signal });
          if (!active()) return;
          if (response.providers.some(provider => !provider.available)) throw new Error("Beatmap search is unavailable. Please try again.");
          // Give each song a usable PP result before working through its remaining difficulties.
          await runLanes(response.items.slice(0, 12), async (item, lane) => {
            let maps: BeatmapDifficulty[];
            try {
              const detail = await osuClient.getBeatmapItem({ provider: Provider.OSU_OFFICIAL, sourceId: item.sourceId }, { signal: controller.signal });
              if (!active()) return;
              if (!detail.item || detail.provider?.available === false) throw new Error("Map unavailable");
              maps = detail.item.difficulties.filter(map => map.ruleset === Ruleset.OSU && (!low || map.stars >= Number(low)) && (!high || map.stars <= Number(high))).sort((a, b) => b.stars - a.stars);
            } catch {
              if (active()) { detailsFailed = true; setError("Some beatmaps could not load. You can browse the available results or retry."); }
              return;
            }
            const added = maps.filter(map => !candidates.some(row => row.map.beatmapId === map.beatmapId)).map(map => ({ map, result: cache.getResult(map, settings) }));
            candidates.push(...added); publish();
            if (added[0]) await calculate(added[0], lane);
          });
          if (active() && !detailsFailed && !candidates.some(row => row.result?.error)) cache.setCandidates(searchKey, candidates.map(row => row.map));
        }
        if (!active()) return;
        const bySet = new Map<string, Candidate[]>();
        for (const candidate of candidates) {
          if (candidate.result) continue;
          const key = candidate.map.beatmapsetId;
          const group = bySet.get(key) ?? []; group.push(candidate); bySet.set(key, group);
        }
        const queue: Candidate[] = [];
        const groups = [...bySet.values()];
        for (let index = 0; groups.some(group => index < group.length); index++) {
          for (const group of groups) if (group[index]) queue.push(group[index]);
        }
        await runLanes(queue, calculate);
      } catch (failure) { if (active()) setError(failure instanceof Error ? failure.message : "Search unavailable"); }
      finally { workers.forEach(worker => worker?.terminate()); if (active()) setBusy(false); }
    };
    const timer = setTimeout(() => void run(), restored ? 0 : 300);
    return () => { clearTimeout(timer); controller.abort(); workers.forEach(worker => worker?.terminate()); };
  }, [query, low, high, accuracy, mods, lazer, attempt]);

  const ordered = [...rows].sort((a, b) => sort === "stars" ? (b.result?.stars ?? b.map.stars) - (a.result?.stars ?? a.map.stars) : (sort === "max" ? b.result?.maxPp ?? -1 : b.result?.pp ?? -1) - (sort === "max" ? a.result?.maxPp ?? -1 : a.result?.pp ?? -1));
  const visible = ordered.filter(row => matchesPpGoal(row.result, goal));
  const sets = new Map<string, Candidate[]>();
  for (const row of visible) {
    const key = row.map.beatmapsetId || row.map.beatmapId;
    const group = sets.get(key) ?? [];
    group.push(row); sets.set(key, group);
  }
  return <div className="pp-finder">
    <PageSeo title="osu! PP Targets · AimMod Hub" description="Find ranked osu! difficulties and compare full-combo PP at your chosen accuracy and mods." />
    <header className="pp-heading"><h1>PP beatmaps</h1><div className="pp-search"><Search size={18} aria-hidden="true" /><input id="beatmap-search" aria-label="Search beatmaps" type="search" value={query} onChange={event => updateParam("q", event.target.value)} placeholder="Song, artist or mapper" maxLength={256} /></div><Link to="/osu/beatmaps">All beatmaps <ArrowUpRight size={16} /></Link></header>
    <div className="pp-toolbar">
      <RangeSlider name="stars" label="Difficulty (no mods)" limit={10} step={0.1} minimum={low} maximum={high} onChange={(endpoint, value) => updateParam(endpoint === "Min" ? "min" : "max", value)} />
      <label>Accuracy<div className="pp-percent"><input aria-label="Target accuracy" type="number" min={80} max={100} step={0.1} key={accuracy} defaultValue={accuracy} onBlur={event => updateParam("acc", event.target.value)} onKeyDown={event => { if (event.key === "Enter") event.currentTarget.blur(); }} /><span>%</span></div></label>
      <label>Mods<select value={mods} onChange={event => updateParam("mods", event.target.value)}>{ppMods.map(mod => <option key={mod} value={mod}>{mod === "NM" ? "No mods" : mod}</option>)}</select></label>
        <label>Scoring<select aria-label="Scoring" value={String(lazer)} onChange={event => updateParam("scoring", event.target.value === "true" ? "lazer" : "stable")}><option value="true">Lazer</option><option value="false">Stable</option></select></label>
      <label>PP goal<input aria-label="Minimum PP" type="number" min="0" max="10000" step="10" value={params.get("ppMin") ?? ""} onChange={event => updateParam("ppMin", event.target.value)} placeholder="Any PP" /></label>
      <details className="pp-more"><summary><SlidersHorizontal size={16} /> More filters{(goal.max !== undefined || !lazer) && <span className="pp-filter-dot" />}</summary><div className="pp-more-panel">
        <label>Maximum PP<input type="number" min="0" max="10000" step="10" value={params.get("ppMax") ?? ""} onChange={event => updateParam("ppMax", event.target.value)} placeholder="Any" /></label>
        <button className="pp-text-button" type="button" onClick={() => setParams({ min: "3", max: "7", acc: "98", mods: "NM", scoring: "lazer", sort: "pp" })}>Reset all filters</button>
      </div></details>
    </div>
    <div className="pp-context"><span>Full-combo estimate · {accuracy.toFixed(1)}% · {lazer ? "Lazer" : "Stable"}{goal.max !== undefined ? ` · Up to ${goal.max} PP` : ""}</span><span>No misses or dropped slider ends.</span></div>
    {error && <div role="alert" className="py-4"><p className="mb-3">{error}</p><Button onClick={() => setAttempt(value => value + 1)}>Try again</Button></div>}
    <div className="pp-results-heading"><div><h2>{sets.size} beatmaps <span>· {visible.length} difficulties</span></h2>{busy && <span role="status">{rows.length ? `Calculating PP · ${progress}/${rows.length}` : "Finding beatmaps…"}</span>}</div><div className="pp-result-tools"><select aria-label="Sort beatmaps" value={sort} onChange={event => updateParam("sort", event.target.value)}><option value="pp">Highest PP</option><option value="max">Highest SS PP</option><option value="stars">Highest difficulty</option></select><button type="button" aria-label="Refresh results" title="Refresh results" disabled={busy} onClick={() => { browserPpCache().deleteCandidates(candidateKey({ query, low, high })); setAttempt(value => value + 1); }}><RotateCw size={16} /></button></div></div>
    {busy && !visible.length && !error && <div className="pp-loading" role="status">{goal.min !== undefined || goal.max !== undefined ? "Checking maps against your PP goal…" : "Looking for ranked beatmaps…"}<div className="pp-loading-line" /></div>}
    {!busy && !error && !visible.length && <div className="pp-empty"><h3>No beatmaps match these filters</h3><p>Try a wider star or PP range, or another song.</p><Button onClick={() => setParams({ min: "3", max: "7", acc: "98", mods: "NM", scoring: "lazer", sort: "pp" })}>Reset filters</Button></div>}
    <div className="pp-results card-grid">{[...sets].map(([setId, difficulties]) => <BeatmapCard key={setId} difficulties={difficulties} accuracy={accuracy} mods={mods} lazer={lazer} showPp />)}</div>
    <p className="pp-footnote">From up to 12 popular matching sets. Change your search to explore more.</p>
  </div>;
}
