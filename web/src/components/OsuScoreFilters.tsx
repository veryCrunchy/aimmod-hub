import { useSearchParams } from "react-router-dom";
import { RotateCcw } from "lucide-react";
import { RangeSlider } from "../pages/OsuCatalogPage";
import { scoreFilterKeys } from "../lib/osuScoreFilters";
import "./OsuScoreFilters.css";
import "./browse.css";

export function OsuScoreFilters({ defaultReplay = "all" }: { defaultReplay?: string }) {
  const [params, setParams] = useSearchParams();
  const update = (key: string, value: string) => setParams(previous => {
    const next = new URLSearchParams(previous);
    if (value) next.set(key, value); else next.delete(key);
    return next;
  }, { replace: true });
  const options = [
    ["source", "Source", "all", [["all", "All sources"], ["uploads", "AimMod uploads"], ["official", "osu! scores"], ["merged", "On both"]]],
    ["replay", "Replay", defaultReplay, [["all", "All plays"], ["file", "Has replay"], ["none", "No replay listed"], ["analysis", "Uploaded analysis"]]],
    ["mods", "Mods", "all", [["all", "Any mods"], ["NM", "No Mod"], ["HD", "Hidden"], ["HR", "Hard Rock"], ["DT", "Double Time / Nightcore"], ["HT", "Half Time / Daycore"], ["EZ", "Easy"], ["FL", "Flashlight"], ["NF", "No Fail"], ["CL", "Classic"]]],
    ["result", "Result", "all", [["all", "All results"], ["passed", "Passed"], ["failed", "Failed"]]],
    ["period", "Played", "all", [["all", "Any time"], ["7", "Past week"], ["30", "Past month"], ["90", "Past 3 months"], ["365", "Past year"]]],
    ["sort", "Sort", "recent", [["recent", "Newest played"], ["pp", "Highest PP"], ["accuracy", "Highest accuracy"], ["stars", "Highest stars"], ["score", "Highest score"]]],
  ] as const;
  return <div className="score-filters">
    <div className="hub-filters score-filters__search"><label>Find a play<input type="search" value={params.get("q") ?? ""} onChange={event => update("q", event.target.value)} placeholder="Beatmap, player, or mapper" /></label>
      <button type="button" title="Reset score filters" aria-label="Reset score filters" onClick={() => setParams(previous => { const next = new URLSearchParams(previous); scoreFilterKeys.forEach(key => next.delete(key)); return next; }, { replace: true })}><RotateCcw size={18} /></button></div>
    <div className="hub-filters score-filters__selects">{options.map(([key, label, fallback, values]) => <label key={key}>{label}<select value={params.get(key) ?? fallback} onChange={event => update(key, event.target.value)}>{values.map(([id, text]) => <option value={id} key={id}>{text}</option>)}</select></label>)}</div>
    <div className="score-filters__ranges">{([["stars", "Stars", 15, .1], ["acc", "Accuracy (%)", 100, .1], ["pp", "PP", 2000, 5]] as const).map(([name, label, limit, step]) => <RangeSlider key={name} name={name} label={label} limit={limit} step={step} minimum={params.get(`${name}Min`) ?? ""} maximum={params.get(`${name}Max`) ?? ""} onChange={(endpoint, value) => update(`${name}${endpoint}`, value)} />)}</div>
  </div>;
}

export function ScoreBrowserControls({ title = "Scores & replays", defaultReplay = "all" }: { title?: string; defaultReplay?: string }) {
  const [params, setParams] = useSearchParams();
  return <>
    <div className="profile-plays-heading"><h2>{title}</h2><div className="profile-score-tabs" role="group" aria-label="Score order">{[["recent", "Recent"], ["pp", "Best PP"]].map(([sort, text]) => <button type="button" key={sort} aria-pressed={(params.get("sort") ?? "recent") === sort} onClick={() => setParams(current => { const next = new URLSearchParams(current); next.set("sort", sort); return next; }, { replace: true })}>{text}</button>)}</div></div>
    <details className="profile-filter-toggle"><summary>Search & filter scores{scoreFilterKeys.some(key => key !== "sort" && params.has(key)) ? " · Filters active" : ""}</summary><OsuScoreFilters defaultReplay={defaultReplay} /></details>
  </>;
}
