import { formatScorePp } from "../lib/scorePp";
import { Link } from "react-router-dom";
import { formatOsuAccuracy, formatOsuMods, osuScorePath, type OsuSharedReplay } from "../lib/osuCommunity";

export function OsuReplayRow({ replay }: { replay: OsuSharedReplay }) {
  const pp = replay.performancePoints == null ? (["queued", "calculating"].includes(replay.ppCalculationState ?? "") ? "Calculating..." : "Unavailable") : formatScorePp(replay.performancePoints);
  const stars = replay.calculatedStarRating ?? replay.starRating;
  const baseDifficulty = replay.calculatedStarRating == null && replay.source === "official";
  const judged = replay.count300 + replay.count100 + replay.count50 + replay.countMiss;
  const accuracy = judged === 0 && !replay.passed ? "—" : formatOsuAccuracy(replay.accuracy);
  const unsupported = replay.ppCalculationState === "unsupported";
  const ppReason = unsupported ? "PP calculation is available for osu!standard only." : replay.ppCalculationError;
  const ppLabel = unsupported ? "PP · unsupported mode" : replay.ppSource === "calculated" ? replay.passed ? "calculated PP" : "partial play PP" : "performance";
  return (
    <Link
      to={osuScorePath(replay)}
      className="hub-replay-row"
    >
      {replay.coverUrl ? <img src={replay.coverUrl} className="hub-replay-cover" alt="" loading="lazy" /> : <span className="hub-replay-cover" aria-hidden="true" />}
      <div className="min-w-0">
        <strong className="block break-words text-[13px] font-semibold text-text">{replay.artist} - {replay.title}</strong>
        <span className="mt-1 block truncate text-[11px] text-muted">
          [{replay.difficulty}] by {replay.creator} · {replay.osuUsername || replay.hubHandle}
        </span>
        <span className="mt-1 block text-[10px] text-muted">
          {replay.passed ? "Passed" : "Failed / stopped"} · {replay.maxCombo}x combo · {replay.countMiss} misses
          {!replay.passed ? ` · ${judged} objects judged${replay.mapObjectCount ? ` / ${replay.mapObjectCount}` : ""}` : ""}
        </span>
        <span className="hub-replay-mobile">
          <span className="text-gold">{stars.toFixed(2)}★{baseDifficulty ? " base" : ""}</span>
          <span>{accuracy}</span>
          <span className="text-[#ff78b4]" title={ppReason}>{unsupported ? "PP unavailable for this mode" : pp}</span>
          <span>{formatOsuMods(replay.mods)}</span>
        </span>
      </div>
      <Metric value={`${stars.toFixed(2)}★`} label={baseDifficulty ? "base difficulty" : "difficulty"} className="text-gold" />
      <Metric value={accuracy} label="accuracy" className="text-cyan" />
      <Metric value={pp} label={ppLabel} title={ppReason} className="text-[#ff78b4]" />
      <Metric value={formatOsuMods(replay.mods)} label="mods" />
    </Link>
  );
}

function Metric({ value, label, title, className = "text-text" }: { value: string; label: string; title?: string; className?: string }) {
  return (
    <span className="hub-replay-metric grid min-w-0 gap-0.5 text-right" title={title}>
      <strong className={`text-[12px] tabular-nums ${className}`}>{value}</strong>
      <span className="text-[9px] uppercase text-muted-2">{label}</span>
    </span>
  );
}
