import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import type { GetLeaderboardResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Skeleton } from "../components/ui/Skeleton";
import { TypeFilterBar } from "../components/ui/TypeFilterBar";
import { Grid, PageStack } from "../components/ui/Stack";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { hubClient, formatRelativeTime, slugifyScenarioName } from "../lib/api";
import { GetLeaderboardRequest } from "../gen/aimmod/hub/v1/hub_pb";

type Tab = "records" | "top";

function fetchLeaderboard(scenarioType: string) {
  return hubClient.getLeaderboard(new GetLeaderboardRequest({ scenarioType }));
}

export function LeaderboardPage() {
  const [data, setData] = useState<GetLeaderboardResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>("records");
  const [typeFilter, setTypeFilter] = useState<string | null>(null);

  const load = useCallback(() => {
    void fetchLeaderboard("")
      .then((next) => { setData(next); setError(null); })
      .catch((err) => { setError(err instanceof Error ? err.message : "Could not load leaderboard."); });
  }, []);

  useEffect(() => { load(); }, [load]);
  useAutoRefresh(load, 60_000);

  const scenarioTypes = useMemo(() => {
    if (!data) return [];
    const types = new Set<string>();
    for (const r of [...data.records, ...data.topScores]) {
      if (r.scenarioType?.trim() && r.scenarioType !== "Unknown") types.add(r.scenarioType);
    }
    return [...types];
  }, [data]);

  const filteredRecords = useMemo(() => {
    if (!data) return [];
    return typeFilter ? data.records.filter((r) => r.scenarioType === typeFilter) : data.records;
  }, [data, typeFilter]);

  const filteredTop = useMemo(() => {
    if (!data) return [];
    return typeFilter ? data.topScores.filter((r) => r.scenarioType === typeFilter) : data.topScores;
  }, [data, typeFilter]);

  const activeList = tab === "records" ? filteredRecords : filteredTop;

  if (error) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Leaderboard" title="Could not load leaderboard" />
          <EmptyState title="Unavailable" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!data) {
    return (
      <PageStack>
        <PageSection>
          <Skeleton className="mb-3 h-3 w-24" />
          <Skeleton className="mb-3 h-10 w-48" />
          <Skeleton className="h-4 w-80" />
        </PageSection>
        <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
          {[0, 1, 2].map((i) => <Skeleton key={i} className="h-[100px]" />)}
        </Grid>
      </PageStack>
    );
  }

  const uniqueScenarios = new Set(data.records.map((r) => r.scenarioName)).size;
  const uniquePlayers = new Set([...data.records, ...data.topScores].map((r) => r.userHandle)).size;

  return (
    <PageStack>
      <PageSection>
        <SectionHeader
          eyebrow="Leaderboard"
          title="Records and top scores"
          body="The best scores this community has put up — scenario records and the all-time top runs."
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(200px,1fr))]">
          <StatCard label="Scenario records" value={data.records.length.toLocaleString()} detail="Scenarios with a record holder" />
          <StatCard label="Unique scenarios" value={uniqueScenarios.toLocaleString()} detail="Distinct scenario pages" accent="cyan" />
          <StatCard label="Record holders" value={uniquePlayers.toLocaleString()} detail="Players on the board" accent="gold" />
        </Grid>
      </PageSection>

      <PageSection>
        {/* tabs */}
        <div className="mb-[18px] flex items-center gap-1 border-b border-line pb-px">
          {([["records", "Scenario records"], ["top", "Top 100 scores"]] as [Tab, string][]).map(([t, label]) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={[
                "rounded-t-[10px] px-4 py-2 text-[11px] uppercase tracking-[0.08em] transition-colors",
                tab === t ? "border-b-2 border-gold -mb-px text-gold" : "text-muted hover:text-text",
              ].join(" ")}
            >
              {label}
            </button>
          ))}
        </div>

        {scenarioTypes.length > 1 && (
          <TypeFilterBar types={scenarioTypes} active={typeFilter} onChange={setTypeFilter} />
        )}

        {activeList.length > 0 ? (
          <ScrollArea className="max-h-[min(72vh,900px)] overflow-auto rounded-[18px] border border-line bg-white/2">
            <table className="min-w-full text-left text-sm">
              <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                <tr>
                  <th className="px-4 py-3">#</th>
                  <th className="px-4 py-3">Scenario</th>
                  <th className="px-4 py-3">Type</th>
                  <th className="px-4 py-3">Player</th>
                  <th className="px-4 py-3">Score</th>
                  <th className="px-4 py-3">Acc</th>
                  <th className="px-4 py-3">When</th>
                  <th className="px-4 py-3">Run</th>
                </tr>
              </thead>
              <tbody>
                {activeList.map((entry, idx) => {
                  const rank = idx + 1;
                  const rankColor =
                    rank === 1 ? "text-gold" : rank <= 3 ? "text-cyan" : "text-muted-2";
                  return (
                    <tr
                      key={`${entry.runId || entry.sessionId}-${idx}`}
                      className="border-b border-white/6 last:border-b-0 hover:bg-white/[0.015] transition-colors"
                    >
                      <td className={`px-4 py-3 font-medium tabular-nums ${rankColor}`}>{rank}</td>
                      <td className="px-4 py-3 text-text max-w-[200px]">
                        <Link
                          className="hover:text-cyan transition-colors line-clamp-1"
                          to={`/scenarios/${slugifyScenarioName(entry.scenarioName)}`}
                        >
                          {entry.scenarioName}
                        </Link>
                      </td>
                      <td className="px-4 py-3"><ScenarioTypeBadge type={entry.scenarioType} /></td>
                      <td className="px-4 py-3">
                        <Link
                          className="text-cyan underline underline-offset-3"
                          to={`/profiles/${entry.userHandle}`}
                        >
                          {entry.userDisplayName || entry.userHandle}
                        </Link>
                      </td>
                      <td className={`px-4 py-3 font-medium tabular-nums ${rank === 1 ? "text-gold" : "text-text"}`}>
                        {Math.round(entry.score).toLocaleString()}
                      </td>
                      <td className="px-4 py-3 text-text tabular-nums">{entry.accuracy.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-muted" title={new Date(entry.playedAtIso).toLocaleString()}>
                        {formatRelativeTime(entry.playedAtIso)}
                      </td>
                      <td className="px-4 py-3">
                        <Link
                          className="text-cyan underline underline-offset-3"
                          to={`/runs/${entry.runId || entry.sessionId}`}
                        >
                          Open
                        </Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </ScrollArea>
        ) : (
          <EmptyState title="No entries" body="No scores match this filter." />
        )}
      </PageSection>
    </PageStack>
  );
}
