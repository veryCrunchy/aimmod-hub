import { useState } from "react";
import { ArrowDownToLine } from "lucide-react";
import { DifficultyStrip } from "./DifficultyStrip";
import { beatmapLinks, mediaUrl } from "../lib/osuCatalog";
import type { BeatmapDifficulty } from "../gen/aimmod/osu/v1/osu_pb";
import type { PpResult } from "../lib/ppTargetCache";
import "./BeatmapCard.css";

export type BeatmapCandidate = { map: BeatmapDifficulty; result?: PpResult };
function difficultyColor(stars: number) {
  const colors = ["#69b9ff", "#68ddd0", "#a5df71", "#efdb70", "#f4a06d", "#ee788c", "#cf82d8", "#a194ef", "#c0c7ff"];
  return colors[Math.min(colors.length - 1, Math.max(0, Math.floor(stars)))];
}
export function BeatmapCard({ difficulties, accuracy = 98, mods = "NM", lazer = true, showPp = false, status = "Ranked", onDetails }: {
  difficulties: BeatmapCandidate[]; accuracy?: number; mods?: string; lazer?: boolean; showPp?: boolean; status?: string; onDetails?: () => void;
}) {
  const [selectedId, setSelectedId] = useState("");
  if (!difficulties.length) return null;
  const { map, result } = difficulties.find(row => row.map.beatmapId === selectedId) ?? difficulties[0];
  const links = beatmapLinks(map.beatmapsetId);
  const cover = mediaUrl(map.coverUrl);
  const calculatedPp = difficulties.filter(row => row.result && !row.result.error).map(row => row.result!.pp);
  const ppRange = calculatedPp.length > 1 ? `${Math.round(Math.min(...calculatedPp))}–${Math.round(Math.max(...calculatedPp))} pp` : "";
  const byDifficulty = [...difficulties].sort((a, b) => a.map.stars - b.map.stars);
      return <article className="pp-map">
        <div className="pp-card-body">
          <div className="pp-card-heading">{cover && <img className="pp-card-thumb" src={cover} alt="" loading="lazy" onError={event => { event.currentTarget.style.visibility = "hidden"; }} />}<div><h2 title={map.title}>{links ? <a href={links.source} target="_blank" rel="noreferrer">{map.title}</a> : map.title}</h2><p title={`${map.artist} · mapped by ${map.creator}`}>{map.artist} · {map.creator}</p></div></div>
          <div className="pp-difficulty-picker"><div className="pp-diff-caption"><span>{difficulties.length} {difficulties.length === 1 ? "difficulty" : "difficulties"}</span>{ppRange && <span title="PP range across calculated matching difficulties at your selected accuracy">{ppRange}</span>}</div>
            <DifficultyStrip label={`Difficulty for ${map.title}`} selectedId={map.beatmapId} onSelect={setSelectedId} options={byDifficulty.map(candidate => {
              const stars = candidate.result?.stars ?? candidate.map.stars;
              return { id: candidate.map.beatmapId, color: difficultyColor(stars), label: `${candidate.map.name} · ${stars.toFixed(2)} stars${candidate.result && !candidate.result.error ? ` · ${Math.round(candidate.result.pp)} PP` : ""}` };
            })} />
            <p className="pp-selected-difficulty" title={map.name}><span style={{ color: difficultyColor(result?.stars ?? map.stars) }}>{(result?.stars ?? map.stars).toFixed(2)} ★</span><span>{map.name}</span></p>
          </div>
          <div className="pp-card-stats"><span>{mods === "NM" ? "No mods" : mods}</span><span>{map.bpm} <small>BPM</small></span><span>{Math.floor(map.lengthSeconds / 60)}:{String(map.lengthSeconds % 60).padStart(2, "0")} <small>length</small></span></div>
          <div className="pp-card-bottom">{showPp ? <div className="pp-card-value"><span>{accuracy.toFixed(1)}% FC · {lazer ? "Lazer" : "Stable"}</span><strong>{result?.error ? "Unavailable" : result ? <>{Math.round(result.pp)} <small>pp</small></> : <span className="pp-pending">Calculating…</span>}</strong>{result && !result.error && <span className="pp-ss">SS {Math.round(result.maxPp)} pp</span>}</div> : <div className="beatmap-card-status"><span>{status}</span>{onDetails && <button type="button" onClick={onDetails}>Details & audio</button>}</div>}
            {links && <div className="pp-map-actions"><a href={links.osu} className="pp-play"><ArrowDownToLine size={15} /> Play</a><details className="pp-map-more"><summary aria-label={`More ways to open ${map.title}`}>•••</summary><div><a href={links.aimmod}>Open in AimMod</a><a href={links.source} target="_blank" rel="noreferrer">View on osu!</a></div></details></div>}
          </div>
          {showPp && result?.error && <p className="pp-card-error">{result.error}</p>}
        </div>
      </article>;
}
