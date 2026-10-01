import { Link } from "react-router-dom";
import type { RunPreview } from "../gen/aimmod/hub/v1/hub_pb";
import { formatDurationMs, formatRelativeTime, slugifyScenarioName } from "../lib/api";
import { formatAccuracy, formatScore } from "../lib/kovaaksStats";
import { cn } from "../lib/cn";

type RunTableProps = {
  runs: readonly RunPreview[];
  /** Columns to show besides score and accuracy. */
  player?: boolean;
  scenario?: boolean;
  duration?: boolean;
  when?: boolean;
  /** Prefix rows with their position. */
  ranked?: boolean;
  /** Mark the row with this score as the best. */
  bestScore?: number;
  /** Where player names link to; defaults to the profile. */
  playerHref?: (run: RunPreview) => string;
  caption?: string;
  className?: string;
};

export function runHref(run: Pick<RunPreview, "runId" | "sessionId">) {
  return `/runs/${run.runId || run.sessionId}`;
}

/** Compact table of runs. The score links to the run. */
export function RunTable({ runs, player = true, scenario = true, duration = false, when = true, ranked = false, bestScore, playerHref, caption, className }: RunTableProps) {
  return (
    <div className={cn("overflow-x-auto rounded-md border border-line", className)}>
      <table className="w-full min-w-[480px] text-left text-sm">
        {caption ? <caption className="sr-only">{caption}</caption> : null}
        <thead className="border-b border-line text-xs text-muted">
          <tr>
            {ranked && <th scope="col" className="w-10 px-3 py-2 font-medium">#</th>}
            {scenario && <th scope="col" className="px-3 py-2 font-medium">Scenario</th>}
            {player && <th scope="col" className="px-3 py-2 font-medium">Player</th>}
            <th scope="col" className="px-3 py-2 text-right font-medium">Score</th>
            <th scope="col" className="px-3 py-2 text-right font-medium">Accuracy</th>
            {duration && <th scope="col" className="px-3 py-2 text-right font-medium">Length</th>}
            {when && <th scope="col" className="px-3 py-2 text-right font-medium">Played</th>}
          </tr>
        </thead>
        <tbody>
          {runs.map((run, index) => {
            const handle = run.userHandle || run.userDisplayName;
            const best = bestScore != null && Math.round(run.score) === Math.round(bestScore);
            return (
              <tr key={run.runId || run.sessionId} className="border-b border-line/60 last:border-b-0 hover:bg-white/[0.02]">
                {ranked && <td className={cn("px-3 py-2 tabular-nums", index === 0 ? "text-gold" : "text-muted-2")}>{index + 1}</td>}
                {scenario && (
                  <td className="max-w-[260px] truncate px-3 py-2">
                    <Link className="text-text hover:text-cyan" to={`/scenarios/${slugifyScenarioName(run.scenarioName)}`}>{run.scenarioName}</Link>
                  </td>
                )}
                {player && (
                  <td className="max-w-[200px] truncate px-3 py-2">
                    <Link className="text-text hover:text-cyan" to={playerHref ? playerHref(run) : `/profiles/${handle}`}>{run.userDisplayName || run.userHandle}</Link>
                  </td>
                )}
                <td className="px-3 py-2 text-right tabular-nums">
                  <Link className={cn("font-medium hover:text-cyan", best ? "text-gold" : "text-text")} to={runHref(run)} title="Open run">
                    {formatScore(run.score)}
                  </Link>
                  {best ? <span className="ml-1.5 text-xs text-gold">best</span> : null}
                </td>
                <td className="px-3 py-2 text-right tabular-nums text-muted">{formatAccuracy(run.accuracy)}</td>
                {duration && <td className="px-3 py-2 text-right tabular-nums text-muted">{formatDurationMs(run.durationMs)}</td>}
                {when && <td className="whitespace-nowrap px-3 py-2 text-right text-muted" title={new Date(run.playedAtIso).toLocaleString()}>{formatRelativeTime(run.playedAtIso)}</td>}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
