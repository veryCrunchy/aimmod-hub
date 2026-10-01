import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { GetScenarioLeaderboardResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { fetchScenarioLeaderboard, formatRelativeTime } from "../lib/api";
import { cn } from "../lib/cn";
import { countryFlag } from "../lib/country";
import { CountryTag } from "./CountryTag";
import { formatAccuracy, formatScore } from "../lib/kovaaksStats";
import { intParam } from "../lib/urlState";
import { EmptyState } from "./ui/EmptyState";
import { Pager } from "./ui/Pager";
import { TableSkeleton } from "./ui/Skeleton";

export type LeaderboardState = { range: string; sort: string; page: string; size: string; q: string; linked: string };

export const leaderboardDefaults: LeaderboardState = { range: "all", sort: "score", page: "1", size: "50", q: "", linked: "" };
export const leaderboardChoices = {
  range: ["7d", "30d", "90d", "all"],
  sort: ["score", "accuracy", "recent", "runs"],
  size: ["25", "50", "100"],
} as const;

type Props = {
  source: "aimmod" | "kovaaks";
  scenarioSlug?: string;
  scenarioName: string;
  leaderboardId?: number;
  state: LeaderboardState;
  setState: (changes: Partial<LeaderboardState>) => void;
  /** Signed-in player's handle, for "jump to me" and highlighting. */
  viewerHandle?: string;
  /** A KovaaK's player to jump to and highlight on the global board. */
  viewerSteamId?: string;
};

const select = "min-h-9 rounded-md border border-line bg-panel px-2 text-sm text-text";

/** Paged scenario leaderboard with filters kept in the URL. */
export function ScenarioLeaderboardPanel({ source, scenarioSlug, scenarioName, leaderboardId, state, setState, viewerHandle, viewerSteamId }: Props) {
  const [data, setData] = useState<GetScenarioLeaderboardResponse | null>(null);
  const [error, setError] = useState(false);
  const [jump, setJump] = useState(0);
  const pageSize = intParam(state.size, 50, 10, 100);
  const page = intParam(state.page, 1, 1) - 1;
  const aimmod = source === "aimmod";

  useEffect(() => {
    let cancelled = false;
    setError(false);
    const around = jump > 0;
    void fetchScenarioLeaderboard({
      scenarioSlug, scenarioName, leaderboardId, source,
      range: aimmod ? state.range : "", sort: aimmod ? state.sort : "", query: aimmod ? state.q : "", linkedOnly: aimmod && state.linked === "1",
      page, pageSize,
      aroundHandle: around ? viewerHandle : undefined,
      aroundSteamId: around && !aimmod ? viewerSteamId : undefined,
    }).then((next) => {
      if (cancelled) return;
      setData(next);
      if (around) {
        setJump(0);
        if (next.page !== page) setState({ page: String(next.page + 1) });
      }
    }).catch(() => { if (!cancelled) setError(true); });
    return () => { cancelled = true; };
  }, [scenarioSlug, scenarioName, leaderboardId, source, state.range, state.sort, state.q, state.linked, page, pageSize, jump]); // eslint-disable-line react-hooks/exhaustive-deps

  const highlight = (entry: GetScenarioLeaderboardResponse["entries"][number]) =>
    (viewerHandle && entry.userHandle && entry.userHandle.toLowerCase() === viewerHandle.toLowerCase()) ||
    (viewerSteamId && entry.steamId === viewerSteamId) ||
    (data?.highlightRank ? entry.rank === data.highlightRank : false);
  const canJump = aimmod ? Boolean(viewerHandle) : Boolean(viewerHandle || viewerSteamId);

  return (
    <div className="grid gap-3">
      <div className="flex flex-wrap items-center gap-2">
        {aimmod ? (
          <>
            <label className="flex items-center gap-1.5 text-sm text-muted">
              <span>Period</span>
              <select className={select} value={state.range} onChange={(e) => setState({ range: e.target.value, page: "1" })}>
                <option value="7d">Last 7 days</option>
                <option value="30d">Last 30 days</option>
                <option value="90d">Last 90 days</option>
                <option value="all">All time</option>
              </select>
            </label>
            <label className="flex items-center gap-1.5 text-sm text-muted">
              <span>Sort</span>
              <select className={select} value={state.sort} onChange={(e) => setState({ sort: e.target.value, page: "1" })}>
                <option value="score">Best score</option>
                <option value="accuracy">Accuracy</option>
                <option value="recent">Recently played</option>
                <option value="runs">Most runs</option>
              </select>
            </label>
            <label className="flex min-h-9 items-center gap-2 rounded-md border border-line bg-panel px-2.5 text-sm">
              <input type="checkbox" checked={state.linked === "1"} onChange={(e) => setState({ linked: e.target.checked ? "1" : "", page: "1" })} />
              Linked KovaaK's accounts only
            </label>
            <input type="search" value={state.q} onChange={(e) => setState({ q: e.target.value, page: "1" })} placeholder="Find a player"
              aria-label="Find a player" className="min-h-9 w-full rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted sm:w-52" />
          </>
        ) : (
          <p className="text-sm text-muted">Everyone on KovaaK's, AimMod or not. Updated every few minutes.</p>
        )}
        {canJump ? (
          <button type="button" onClick={() => setJump((n) => n + 1)} className="ml-auto inline-flex min-h-9 items-center rounded-md border border-mint/50 bg-mint/10 px-3 text-sm font-medium hover:bg-mint/20">
            Jump to me
          </button>
        ) : null}
      </div>

      {error ? <EmptyState title={aimmod ? "This leaderboard could not be loaded." : "KovaaK's leaderboard is unavailable right now."} body={aimmod ? undefined : "KovaaK's may be busy or this scenario has no public board."} />
        : !data ? <TableSkeleton rows={8} />
        : data.entries.length === 0 ? <EmptyState title={state.q ? "No player matches that name." : "No scores in this period."} />
        : (
          <div className="overflow-x-auto rounded-md border border-line">
            <table className="w-full text-left text-sm">
              <caption className="sr-only">{aimmod ? "AimMod players" : "KovaaK's global"} leaderboard for {scenarioName}</caption>
              <thead className="border-b border-line text-xs text-muted">
                <tr>
                  <th scope="col" className="w-14 px-3 py-2 font-medium">#</th>
                  <th scope="col" className="px-3 py-2 font-medium">Player</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">Score</th>
                  {aimmod ? <th scope="col" className="px-3 py-2 text-right font-medium max-sm:hidden">Accuracy</th> : <th scope="col" className="px-3 py-2 text-right font-medium max-sm:hidden">cm/360</th>}
                  {aimmod ? <th scope="col" className="px-3 py-2 text-right font-medium max-sm:hidden">Runs</th> : null}
                  <th scope="col" className="px-3 py-2 text-right font-medium max-sm:hidden">Date</th>
                </tr>
              </thead>
              <tbody>
                {data.entries.map((entry) => {
                  const mine = highlight(entry);
                  const playerLink = entry.userHandle ? `/profiles/${entry.userHandle}${scenarioSlug ? `/scenarios/${scenarioSlug}` : ""}` : entry.steamId ? `/u/${entry.steamId}` : undefined;
                  const flag = countryFlag(entry.country);
                  return (
                    <tr key={`${entry.rank}:${entry.userHandle || entry.steamId}`} className={cn("border-b border-line/60 last:border-b-0", mine ? "bg-mint/10" : "hover:bg-white/[0.02]")} aria-current={mine ? "true" : undefined}>
                      <td className={cn("px-3 py-2 tabular-nums", entry.rank <= 3 ? "text-gold" : "text-muted-2")}>{entry.rank.toLocaleString()}</td>
                      <td className="max-w-[200px] px-3 py-2 sm:max-w-[320px]">
                        <div className="flex min-w-0 items-center gap-2">
                          {flag ? <CountryTag code={entry.country} /> : null}
                          {playerLink ? <Link to={playerLink} className="truncate hover:text-cyan">{entry.displayName || entry.userHandle || entry.kovaaksUsername}</Link> : <span className="truncate">{entry.displayName}</span>}
                          {!aimmod && entry.isAimmodPlayer ? <span className="shrink-0 rounded border border-mint/40 px-1 text-[10px] text-mint">AimMod</span> : null}
                          {aimmod && entry.isLinked ? <span className="shrink-0 text-[10px] text-muted-2" title="Linked KovaaK's account">linked</span> : null}
                        </div>
                      </td>
                      <td className="px-3 py-2 text-right font-medium tabular-nums">
                        {entry.runId ? <Link to={`/runs/${entry.runId}`} className="hover:text-cyan">{formatScore(entry.score)}</Link> : formatScore(entry.score)}
                      </td>
                      {aimmod ? <td className="px-3 py-2 text-right tabular-nums text-muted max-sm:hidden">{formatAccuracy(entry.accuracy)}</td>
                        : <td className="px-3 py-2 text-right tabular-nums text-muted max-sm:hidden">{entry.cm360 ? entry.cm360.toFixed(1) : "–"}</td>}
                      {aimmod ? <td className="px-3 py-2 text-right tabular-nums text-muted max-sm:hidden">{entry.runCount.toLocaleString()}</td> : null}
                      <td className="px-3 py-2 text-right text-muted max-sm:hidden">{entry.playedAtIso ? formatRelativeTime(entry.playedAtIso) : "–"}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

      {data && data.total > 0 ? (
        <Pager page={page} pageSize={pageSize} total={data.total} onPage={(p) => setState({ page: String(p + 1) })}
          pageSizes={[25, 50, 100]} onPageSize={(size) => setState({ size: String(size), page: "1" })} label="Leaderboard pages" />
      ) : null}
    </div>
  );
}
