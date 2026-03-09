import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchOverview, formatDurationMs } from "../lib/api";

export function CommunityPage() {
  const [overview, setOverview] = useState<GetOverviewResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void fetchOverview()
      .then((next) => {
        if (!cancelled) {
          setOverview(next);
          setError(null);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Could not load community data.");
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <PageStack>
      <PageSection>
        <SectionHeader
          eyebrow="Community"
          title="Where the strongest comparisons already are"
          body="This is where you can find the scenarios, players, and recent runs that are already worth studying."
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
          <StatCard label="Runs" value={overview ? overview.totalRuns.toLocaleString() : "—"} detail="Runs available to compare" />
          <StatCard label="Scenarios" value={overview ? overview.totalScenarios.toLocaleString() : "—"} detail="Scenario pages with saved history" accent="cyan" />
          <StatCard label="Players" value={overview ? overview.totalPlayers.toLocaleString() : "—"} detail="Players with public practice history" accent="gold" />
        </Grid>
      </PageSection>

      <Grid className="grid-cols-[1.15fr_0.85fr] items-start max-[1180px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Scenario watchlist"
            title="Best pages to open next"
            body="These scenarios already have enough history to start showing useful score bands and run patterns."
          />
          {overview?.topScenarios.length ? (
            <ScrollArea className="max-h-[min(64vh,820px)] overflow-auto rounded-[18px] border border-line bg-white/2">
              <table className="min-w-full text-left text-sm">
                <thead className="border-b border-line text-[11px] uppercase tracking-[0.08em] text-muted">
                  <tr>
                    <th className="px-4 py-3">Scenario</th>
                    <th className="px-4 py-3">Type</th>
                    <th className="px-4 py-3">Runs</th>
                    <th className="px-4 py-3">Open</th>
                  </tr>
                </thead>
                <tbody>
                  {overview.topScenarios.map((scenario) => (
                    <tr key={scenario.scenarioSlug} className="border-b border-white/6 last:border-b-0">
                      <td className="px-4 py-3 text-text">{scenario.scenarioName}</td>
                      <td className="px-4 py-3 text-text">{scenario.scenarioType || "Unknown"}</td>
                      <td className="px-4 py-3 text-text">{scenario.runCount.toLocaleString()}</td>
                      <td className="px-4 py-3 text-text">
                        <Link className="text-cyan underline underline-offset-3" to={`/scenarios/${scenario.scenarioSlug}`}>
                          Open
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </ScrollArea>
          ) : (
            <EmptyState title="No scenario pages yet" body={error || "Scenario watchlists will appear here once there is enough history to compare."} />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Player watchlist"
            title="Most active uploaders"
            body="These are the best profiles to inspect first when you want examples and consistent practice history."
          />
          {overview?.activeProfiles.length ? (
            <ScrollArea className="max-h-[min(64vh,820px)] pr-2">
              <div className="grid gap-3">
              {overview.activeProfiles.map((profile) => (
                <Link
                  key={profile.userHandle}
                  to={`/profiles/${profile.userHandle}`}
                  className="rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <strong className="block text-text">{profile.userDisplayName || profile.userHandle}</strong>
                      <p className="mt-1 text-sm text-muted">@{profile.userHandle}</p>
                    </div>
                    <span className="text-sm text-mint">{profile.runCount.toLocaleString()} runs</span>
                  </div>
                  <p className="mt-3 text-sm text-muted">
                    {profile.scenarioCount.toLocaleString()} scenarios • {profile.primaryScenarioType || "Unknown"}
                  </p>
                </Link>
              ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState title="No active profiles yet" body={error || "Active profiles will appear here once there is enough practice history to show."} />
          )}
        </PageSection>
      </Grid>

      <PageSection>
        <SectionHeader
          eyebrow="Recent completed runs"
          title="What players have actually just finished"
          body="The latest runs players have completed."
        />
        {overview?.recentRuns.length ? (
          <ScrollArea className="max-h-[min(62vh,780px)] overflow-auto rounded-[18px] border border-line bg-white/2">
            <table className="min-w-full text-left text-sm">
              <thead className="border-b border-line text-[11px] uppercase tracking-[0.08em] text-muted">
                <tr>
                  <th className="px-4 py-3">Player</th>
                  <th className="px-4 py-3">Scenario</th>
                  <th className="px-4 py-3">Score</th>
                  <th className="px-4 py-3">Acc</th>
                  <th className="px-4 py-3">Duration</th>
                  <th className="px-4 py-3">Run</th>
                </tr>
              </thead>
              <tbody>
                {overview.recentRuns.map((run) => (
                  <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0">
                    <td className="px-4 py-3 text-text">
                      <Link className="text-cyan underline underline-offset-3" to={`/profiles/${run.userHandle || run.userDisplayName}`}>
                        {run.userDisplayName || run.userHandle}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-text">{run.scenarioName}</td>
                    <td className="px-4 py-3 text-text">{Math.round(run.score).toLocaleString()}</td>
                    <td className="px-4 py-3 text-text">{run.accuracy.toFixed(1)}%</td>
                    <td className="px-4 py-3 text-text">{formatDurationMs(run.durationMs)}</td>
                    <td className="px-4 py-3 text-text">
                      <Link className="text-cyan underline underline-offset-3" to={`/runs/${run.runId || run.sessionId}`}>
                        Open
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </ScrollArea>
        ) : (
          <EmptyState title="No recent runs yet" body={error || "Recent runs will appear here once there is practice history to show."} />
        )}
      </PageSection>
    </PageStack>
  );
}
