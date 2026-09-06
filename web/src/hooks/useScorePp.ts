import { useEffect, useMemo, useState } from "react";
import type { OsuSharedReplay } from "../lib/osuCommunity";
import { API_BASE_URL } from "../lib/config";
import { getCachedScorePpResult, setCachedScorePpResult, type ScorePpResult, scorePpCacheKey, scorePpValidationReason } from "../lib/scorePp";

const empty: OsuSharedReplay[] = [];
export function useScorePp(source: OsuSharedReplay[] | null | undefined) {
  const original = source ?? empty;
  const [metadata, setMetadata] = useState<Record<string, Partial<OsuSharedReplay>>>({});
  const [results, setResults] = useState<Record<string, Partial<ScorePpResult> & { error?: string }>>({});
  const [activeKey, setActiveKey] = useState("");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setMetadata({});
    void (async () => {
      const resolved: Record<string, Partial<OsuSharedReplay>> = {};
      for (const score of original) {
        if (controller.signal.aborted) return;
        if (score.performancePoints != null || score.ppCalculation || score.ppCalculationStatus !== "pending") continue;
        try {
          const response = await fetch(`${API_BASE_URL}/api/osu/v1/replays/${encodeURIComponent(score.shareId)}`, {
            signal: AbortSignal.any([controller.signal, AbortSignal.timeout(10000)]),
            headers: { Accept: "application/json" },
          });
          if (!response.ok) throw new Error("Score data could not be loaded. Please retry.");
          const detail = await response.json() as OsuSharedReplay;
          if (detail.shareId !== score.shareId || detail.osuUserId !== score.osuUserId || detail.beatmapId !== score.beatmapId || detail.ruleset !== score.ruleset)
            throw new Error("Score data did not match this play.");
          if (detail.performancePoints == null && !detail.ppCalculation)
            throw new Error("Exact score data is not available yet. Please retry.");
          resolved[score.shareId] = { performancePoints: detail.performancePoints, ppCalculation: detail.ppCalculation, ppSource: detail.ppSource };
        } catch (error) {
          resolved[score.shareId] = { ppCalculationError: error instanceof Error ? error.message : "Score data could not be loaded. Please retry." };
        }
      }
      if (!controller.signal.aborted) setMetadata(resolved);
    })();
    return () => controller.abort();
  }, [original, attempt]);
  const input = useMemo(() => original.map(score => ({ ...score, ...metadata[score.shareId] })), [original, metadata]);
  useEffect(() => {
    let active = true;
    let worker: Worker | undefined;
    let cancelPending: (() => void) | undefined;
    const run = async () => {
      const seen = new Set<string>();
      for (const score of input) {
        if (!active) break;
        if (score.performancePoints != null || !score.ppCalculation) continue;
        const calculation = score.ppCalculation;
        const key = scorePpCacheKey(calculation);
        if (!key) continue;
        if (seen.has(key)) continue;
        seen.add(key);
        let result = getCachedScorePpResult(calculation);
        if (result !== undefined) {
          setResults(previous => ({ ...previous, [key]: result! }));
          continue;
        }
        setActiveKey(key);
        try {
          worker ??= new Worker(new URL("../lib/scorePpWorker.ts", import.meta.url), { type: "module" });
          result = await new Promise<ScorePpResult>((resolve, reject) => {
            const finish = (error?: Error, value?: ScorePpResult) => {
              clearTimeout(timer); cancelPending = undefined;
              if (worker) { worker.onmessage = null; worker.onerror = null; }
              if (error) reject(error); else resolve(value!);
            };
            const timer = setTimeout(() => { worker?.terminate(); worker = undefined; finish(new Error("PP calculation timed out. Please retry.")); }, 25000);
            cancelPending = () => finish(new Error("Cancelled"));
            worker!.onerror = () => { worker?.terminate(); worker = undefined; finish(new Error("PP calculation could not start. Please retry.")); };
            worker!.onmessage = event => {
              if (event.data?.id !== key) return;
              if (typeof event.data.pp === "number" && Number.isFinite(event.data.pp) && event.data.pp >= 0) finish(undefined, { pp: event.data.pp, stars: event.data.stars, objectCount: event.data.objectCount });
              else finish(new Error(event.data.error || "PP calculation unavailable."));
            };
            try { worker!.postMessage({ id: key, input: calculation, url: `${API_BASE_URL}/api/osu/v1/playback/beatmaps/${calculation.beatmapId}/file?checksum=${encodeURIComponent(calculation.beatmapChecksum)}` }); }
            catch { finish(new Error("PP calculation could not start. Please retry.")); }
          });
          if (!active) break;
          setCachedScorePpResult(calculation, result);
          setResults(previous => ({ ...previous, [key]: result! }));
        } catch (error) {
          if (active) setResults(previous => ({ ...previous, [key]: { error: error instanceof Error ? error.message : "PP calculation unavailable." } }));
        }
      }
      worker?.terminate();
      if (active) setActiveKey("");
    };
    setResults({});
    void run();
    return () => { active = false; cancelPending?.(); worker?.terminate(); };
  }, [input, attempt]);
  const items = useMemo(() => input.map(score => {
    if (score.performancePoints != null) return score;
    if (!score.ppCalculation) return { ...score, ppCalculationState: score.ppCalculationStatus === "pending" && !score.ppCalculationError ? "queued" as const : "unavailable" as const };
    const invalid = scorePpValidationReason(score.ppCalculation);
    if (invalid) return { ...score, ppCalculationState: "unavailable" as const, ppCalculationError: invalid };
    const key = scorePpCacheKey(score.ppCalculation);
    const result = results[key];
    if (result?.pp !== undefined) return { ...score, performancePoints: result.pp, calculatedStarRating: result.stars, mapObjectCount: result.objectCount, ppSource: "calculated" };
    return { ...score, ppCalculationState: result?.error ? "unavailable" as const : key === activeKey ? "calculating" as const : "queued" as const, ppCalculationError: result?.error };
  }), [input, results, activeKey]);
  const pending = items.filter(score => ["queued", "calculating"].includes(score.ppCalculationState ?? "")).length;
  const failed = items.filter(score => score.ppCalculationError).length;
  return { items, pending, failed, retry: () => setAttempt(value => value + 1) };
}
