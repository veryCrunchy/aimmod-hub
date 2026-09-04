import { Link } from "react-router-dom";
import { formatOsuAccuracy, formatOsuMods, type OsuSharedReplay } from "../lib/osuCommunity";

export function OsuReplayRow({ replay }: { replay: OsuSharedReplay }) {
  const pp = replay.performancePoints == null ? "PP pending" : `${Math.round(replay.performancePoints)}pp`;
  return (
    <Link
      to={`/osu/replays/${replay.shareId}`}
      className="hub-replay-row"
    >
      {replay.coverUrl ? <img src={replay.coverUrl} className="hub-replay-cover" alt="" loading="lazy" /> : <span className="hub-replay-cover" aria-hidden="true" />}
      <div className="min-w-0">
        <strong className="block break-words text-[13px] font-semibold text-text">{replay.artist} - {replay.title}</strong>
        <span className="mt-1 block truncate text-[11px] text-muted">
          [{replay.difficulty}] by {replay.creator} · @{replay.hubHandle}
        </span>
        <span className="hub-replay-mobile">
          <span className="text-gold">{replay.starRating.toFixed(2)}★</span>
          <span>{formatOsuAccuracy(replay.accuracy)}</span>
          <span className="text-[#ff78b4]">{pp}</span>
          <span>{formatOsuMods(replay.mods)}</span>
        </span>
      </div>
      <Metric value={`${replay.starRating.toFixed(2)}★`} label="difficulty" className="text-gold" />
      <Metric value={formatOsuAccuracy(replay.accuracy)} label="accuracy" className="text-cyan" />
      <Metric value={pp} label="performance" className="text-[#ff78b4]" />
      <Metric value={formatOsuMods(replay.mods)} label="mods" />
    </Link>
  );
}

function Metric({ value, label, className = "text-text" }: { value: string; label: string; className?: string }) {
  return (
    <span className="hub-replay-metric grid min-w-0 gap-0.5 text-right">
      <strong className={`text-[12px] tabular-nums ${className}`}>{value}</strong>
      <span className="text-[9px] uppercase text-muted-2">{label}</span>
    </span>
  );
}
