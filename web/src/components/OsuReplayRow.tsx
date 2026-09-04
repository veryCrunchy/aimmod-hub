import { Link } from "react-router-dom";
import { formatOsuAccuracy, formatOsuMods, type OsuSharedReplay } from "../lib/osuCommunity";

export function OsuReplayRow({ replay }: { replay: OsuSharedReplay }) {
  const pp = replay.performancePoints == null ? "PP pending" : `${Math.round(replay.performancePoints)}pp`;
  return (
    <Link
      to={`/osu/replays/${replay.shareId}`}
      className="grid min-w-0 grid-cols-[minmax(0,1fr)_repeat(4,minmax(72px,auto))] items-center gap-4 border-b border-line px-4 py-3 transition-colors last:border-b-0 hover:bg-white/3 max-[760px]:grid-cols-[minmax(0,1fr)_auto]"
    >
      <div className="min-w-0">
        <strong className="block truncate text-[13px] font-semibold text-text">{replay.artist} - {replay.title}</strong>
        <span className="mt-1 block truncate text-[11px] text-muted">
          [{replay.difficulty}] by {replay.creator} · @{replay.hubHandle}
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
    <span className="grid min-w-[72px] gap-0.5 text-right max-[760px]:hidden">
      <strong className={`text-[12px] tabular-nums ${className}`}>{value}</strong>
      <span className="text-[9px] uppercase text-muted-2">{label}</span>
    </span>
  );
}
