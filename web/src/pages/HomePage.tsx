import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchOverview, formatDurationMs } from "../lib/api";

export function HomePage() {
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
          setError(err instanceof Error ? err.message : "Could not load hub overview.");
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const featuredScenario = overview?.topScenarios[0];
  const featuredProfile = overview?.activeProfiles[0];

  return (
    <PageStack>
      <PageSection className="bg-[linear-gradient(135deg,rgba(57,208,255,0.12),rgba(50,240,170,0.07)_45%,rgba(255,217,86,0.06)),linear-gradient(180deg,rgba(3,9,23,0.9),rgba(6,11,24,0.96))] p-[30px]">
        <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">AimMod Hub</div>
        <h1 className="my-4 text-[clamp(42px,6vw,88px)] leading-[0.96] tracking-[-0.04em]">
          Shared practice data that is finally useful.
        </h1>
        <p className="max-w-[760px] text-lg leading-8 text-muted">
          AimMod turns your practice into player profiles, scenario pages, and run detail you can study, share, and learn from.
        </p>
        <div className="mt-[22px] flex flex-wrap gap-3">
          <Button to="/community" variant="primary">
            Explore community data
          </Button>
          {featuredScenario ? (
            <Button to={`/scenarios/${featuredScenario.scenarioSlug}`}>Open top scenario</Button>
          ) : null}
          {featuredProfile ? <Button to={`/profiles/${featuredProfile.userHandle}`}>Open active profile</Button> : null}
        </div>
      </PageSection>

      <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
        <StatCard
          label="Runs"
          value={overview ? overview.totalRuns.toLocaleString() : "—"}
          detail={error ? "Overview is unavailable right now" : "Sessions available to study"}
        />
        <StatCard
          label="Scenarios"
          value={overview ? overview.totalScenarios.toLocaleString() : "—"}
          detail="Scenarios with practice history"
          accent="cyan"
        />
        <StatCard
          label="Players"
          value={overview ? overview.totalPlayers.toLocaleString() : "—"}
          detail="Profiles with saved history"
          accent="gold"
        />
      </Grid>

      <Grid className="grid-cols-2 max-[1100px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Top scenarios"
            title="Where the current volume is"
            body="These scenarios already have enough history to be useful comparison pages."
          />
          {overview?.topScenarios.length ? (
            <ScrollArea className="max-h-[min(58vh,760px)] pr-2">
              <div className="grid gap-3">
              {overview.topScenarios.slice(0, 6).map((scenario) => (
                <Link
                  key={scenario.scenarioSlug}
                  to={`/scenarios/${scenario.scenarioSlug}`}
                  className="rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <strong className="block text-text">{scenario.scenarioName}</strong>
                      <p className="mt-1 text-sm text-muted">{scenario.scenarioType || "Unknown"}</p>
                    </div>
                    <span className="text-sm text-mint">{scenario.runCount.toLocaleString()} runs</span>
                  </div>
                </Link>
              ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState
              title="No scenario history yet"
              body={error || "Scenario pages will appear here once there is enough history to compare."}
            />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Active players"
            title="Profiles with real history"
            body="These players have the most practice history right now, which makes them the best starting point for study and comparison."
          />
          {overview?.activeProfiles.length ? (
            <ScrollArea className="max-h-[min(58vh,760px)] pr-2">
              <div className="grid gap-3">
              {overview.activeProfiles.slice(0, 6).map((profile) => (
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
                    <span className="text-sm text-cyan">{profile.runCount.toLocaleString()} runs</span>
                  </div>
                  <p className="mt-3 text-sm text-muted">
                    {profile.scenarioCount.toLocaleString()} scenarios • {profile.primaryScenarioType || "Unknown"}
                  </p>
                </Link>
              ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState
              title="No player history yet"
              body={error || "Player profiles will appear here once there is saved practice history to show."}
            />
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
                  <th className="px-4 py-3">Scenario</th>
                  <th className="px-4 py-3">Player</th>
                  <th className="px-4 py-3">Score</th>
                  <th className="px-4 py-3">Acc</th>
                  <th className="px-4 py-3">Duration</th>
                  <th className="px-4 py-3">Run</th>
                </tr>
              </thead>
              <tbody>
                {overview.recentRuns.slice(0, 12).map((run) => (
                  <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0">
                    <td className="px-4 py-3 text-text">{run.scenarioName}</td>
                    <td className="px-4 py-3 text-text">
                      <Link className="text-cyan underline underline-offset-3" to={`/profiles/${run.userHandle || run.userDisplayName}`}>
                        {run.userDisplayName || run.userHandle}
                      </Link>
                    </td>
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
          <EmptyState
            title="No runs yet"
            body={error || "Recent runs will appear here once there is practice history to show."}
          />
        )}
      </PageSection>
    </PageStack>
  );
}
