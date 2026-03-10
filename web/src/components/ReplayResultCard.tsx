import { Link } from "react-router-dom";
import type { HubSearchRun } from "../lib/api";
import { formatDurationMs, formatRelativeTime } from "../lib/api";
import { ScenarioTypeBadge } from "./ScenarioTypeBadge";

function ReplayCapabilityPill({
  label,
  accent,
}: {
  label: string;
  accent: "mint" | "violet";
}) {
  const classes =
    accent === "mint"
      ? "border-mint/20 bg-mint/10 text-mint"
      : "border-violet/20 bg-violet/10 text-violet";
  return <span className={`rounded-full border px-2 py-1 text-[10px] uppercase tracking-[0.08em] ${classes}`}>{label}</span>;
}

export function ReplayResultCard({ run }: { run: HubSearchRun }) {
  return (
    <div className="rounded-[16px] border border-line bg-white/2 p-4 transition-colors hover:border-mint/25 hover:bg-white/[0.045]">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <Link to={`/runs/${run.publicRunID || run.sessionID}`} className="block truncate text-[15px] font-medium text-text hover:text-cyan">
            {run.scenarioName}
          </Link>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-[12px] text-muted">
            <Link to={`/profiles/${run.userHandle}`} className="truncate text-cyan underline underline-offset-3">
              {run.userDisplayName || run.userHandle}
            </Link>
            <span className="text-muted-2">•</span>
            <span>{formatRelativeTime(run.playedAt)}</span>
          </div>
        </div>
        <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
          <ScenarioTypeBadge type={run.scenarioType} />
          {run.hasVideo ? <ReplayCapabilityPill label="Video" accent="mint" /> : null}
          {run.hasMousePath ? <ReplayCapabilityPill label="Mouse path" accent="violet" /> : null}
        </div>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-muted">
        <span className="text-text">{Math.round(run.score).toLocaleString()} score</span>
        <span>{run.accuracy.toFixed(1)}% acc</span>
        <span>{formatDurationMs(run.durationMS)}</span>
        {run.replayQuality ? <span className="uppercase tracking-[0.08em] text-muted-2">{run.replayQuality}</span> : null}
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-3 text-[12px]">
        <Link className="text-mint underline underline-offset-3" to={`/runs/${run.publicRunID || run.sessionID}`}>
          Open replay
        </Link>
        <span className="text-muted-2">·</span>
        <Link className="text-muted hover:text-text" to={`/scenarios/${run.scenarioSlug}`}>
          Scenario page
        </Link>
      </div>
    </div>
  );
}
