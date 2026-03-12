import { useCallback, useEffect, useState } from "react";
import { Helmet } from "react-helmet-async";
import { Link, NavLink } from "react-router-dom";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "../components/SectionHeader";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { StatCard } from "../components/StatCard";
import { VerificationBadge } from "../components/VerificationBadge";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useCountUp } from "../hooks/useCountUp";
import { fetchOverview, formatDurationMs, formatRelativeTime, slugifyScenarioName } from "../lib/api";

const PAGE_SIZE = 15;

function AnimatedStatCard({
  label,
  target,
  detail,
  accent,
  suffix = "",
}: {
  label: string;
  target: number;
  detail: string;
  accent?: "cyan" | "gold" | "violet";
  suffix?: string;
}) {
  const value = useCountUp(target);
  return (
    <StatCard
      label={label}
      value={target ? `${value.toLocaleString()}${suffix}` : "—"}
      detail={detail}
      accent={accent}
    />
  );
}

export function HomePage() {
  const [overview, setOverview] = useState<GetOverviewResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [runsVisible, setRunsVisible] = useState(PAGE_SIZE);

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

  const doRefresh = useCallback(() => {
    void fetchOverview()
      .then((next) => {
        setOverview(next);
        setError(null);
      })
      .catch(() => {});
  }, []);
  useAutoRefresh(doRefresh, 30_000);

  const featuredScenario = overview?.topScenarios[0];
  const featuredProfile = overview?.activeProfiles[0];

  const visibleRuns = overview?.recentRuns.slice(0, runsVisible) ?? [];
  const hasMoreRuns = (overview?.recentRuns.length ?? 0) > runsVisible;

  return (
    <PageStack>
      <Helmet>
        <title>AimMod Hub · Shared practice intelligence</title>
        <meta name="description" content="Shared KovaaK's practice data. View scenario pages, player profiles, and run history." />
      </Helmet>
      <PageSection className="relative overflow-hidden border-mint/18 bg-[radial-gradient(circle_at_top_left,rgba(121,201,151,0.22),transparent_24%),radial-gradient(circle_at_78%_18%,rgba(184,255,225,0.1),transparent_18%),linear-gradient(135deg,rgba(9,25,18,0.98),rgba(6,15,11,0.96)_52%,rgba(3,8,6,0.98))] shadow-[0_24px_80px_rgba(0,0,0,0.42)]">
        <div className="absolute inset-y-0 right-[8%] w-[28%] rounded-full bg-[radial-gradient(circle,rgba(121,201,151,0.14),transparent_68%)] blur-3xl" />
        <div className="relative text-[11px] uppercase tracking-[0.1em] text-cyan">AimMod Hub</div>
        <h1 className="my-2.5 max-w-[14ch] break-words text-[clamp(28px,5.2vw,60px)] leading-[0.94] tracking-[-0.05em]">
          Shared practice data that is finally useful.
        </h1>
        <p className="max-w-[700px] text-[14px] leading-6 text-[#cbe4d7] md:text-[16px] md:leading-7">
          AimMod turns your practice into player profiles, scenario pages, and run detail you can study, share, and
          learn from.
        </p>
        <div className="relative mt-3 flex flex-wrap gap-2">
          <Button to="/community" variant="primary">
            Explore community data
          </Button>
          {featuredScenario ? (
            <Button to={`/scenarios/${featuredScenario.scenarioSlug}`}>Open top scenario</Button>
          ) : null}
          {featuredProfile ? (
            <Button to={`/profiles/${featuredProfile.userHandle}`}>Open active profile</Button>
          ) : null}
        </div>
      </PageSection>

      <Grid className="grid-cols-[repeat(auto-fit,minmax(150px,1fr))]">
        <AnimatedStatCard
          label="Runs"
          target={overview ? Number(overview.totalRuns) : 0}
          detail={error ? "Overview is unavailable right now" : "Sessions available to study"}
        />
        <AnimatedStatCard
          label="Scenarios"
          target={overview ? Number(overview.totalScenarios) : 0}
          detail="Scenarios with practice history"
          accent="cyan"
        />
        <AnimatedStatCard
          label="Players"
          target={overview ? Number(overview.totalPlayers) : 0}
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
            aside={<NavLink to="/community" className="text-cyan transition-colors hover:underline">Browse all →</NavLink>}
          />
          {overview?.topScenarios.length ? (
            <ScrollArea className="max-h-[min(54vh,720px)] pr-2">
              <div className="grid gap-3">
                {overview.topScenarios.slice(0, 6).map((scenario) => (
                  <Link
                    key={scenario.scenarioSlug}
                    to={`/scenarios/${scenario.scenarioSlug}`}
                    className="rounded-[16px] border border-line bg-white/2 p-4 transition-colors hover:border-cyan/30 hover:bg-white/3"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <strong className="block text-text">{scenario.scenarioName}</strong>
                        <div className="mt-1.5"><ScenarioTypeBadge type={scenario.scenarioType} /></div>
                      </div>
                      <div className="text-right shrink-0">
                        <span className="block text-sm text-mint">{scenario.runCount.toLocaleString()}</span>
                        <span className="text-[10px] text-muted-2 uppercase tracking-wider">runs</span>
                      </div>
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
            aside={<NavLink to="/community" className="text-cyan transition-colors hover:underline">Browse all →</NavLink>}
          />
          {overview?.activeProfiles.length ? (
            <ScrollArea className="max-h-[min(54vh,720px)] pr-2">
              <div className="grid gap-3">
                {overview.activeProfiles.slice(0, 6).map((profile) => (
                  <Link
                    key={profile.userHandle}
                    to={`/profiles/${profile.userHandle}`}
                    className="rounded-[16px] border border-line bg-white/2 p-4 transition-colors hover:border-cyan/30 hover:bg-white/3"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <strong className="block text-text truncate">
                            {profile.userDisplayName || profile.userHandle}
                          </strong>
                          <VerificationBadge verified={Boolean(profile.isVerified)} />
                        </div>
                        <p className="mt-1 text-sm text-muted">@{profile.userHandle}</p>
                      </div>
                      <div className="text-right shrink-0">
                        <span className="block text-sm text-cyan">{profile.runCount.toLocaleString()}</span>
                        <span className="text-[10px] text-muted-2 uppercase tracking-wider">runs</span>
                      </div>
                    </div>
                    <div className="mt-3 flex items-center gap-2">
                      <span className="text-sm text-muted">{profile.scenarioCount.toLocaleString()} scenarios</span>
                      <ScenarioTypeBadge type={profile.primaryScenarioType} />
                    </div>
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
          aside={<NavLink to="/leaderboard" className="text-cyan transition-colors hover:underline">View leaderboard →</NavLink>}
        />
        {overview?.recentRuns.length ? (
          <>
            <ScrollArea className="max-h-[min(56vh,740px)] overflow-auto rounded-[16px] border border-line bg-white/2">
              <table className="min-w-[760px] w-full text-left text-sm">
                <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                  <tr>
                    <th className="px-4 py-3">Scenario</th>
                    <th className="px-4 py-3">Player</th>
                    <th className="px-4 py-3">Score</th>
                    <th className="px-4 py-3">Acc</th>
                    <th className="px-4 py-3">Duration</th>
                    <th className="px-4 py-3">When</th>
                    <th className="px-4 py-3">Run</th>
                  </tr>
                </thead>
                <tbody>
                  {visibleRuns.map((run) => (
                    <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0 hover:bg-white/[0.015] transition-colors">
                      <td className="px-3 py-2.5 text-text max-w-[200px] truncate md:px-4 md:py-3">
                        <Link
                          className="hover:text-cyan transition-colors"
                          to={`/scenarios/${slugifyScenarioName(run.scenarioName)}`}
                        >
                          {run.scenarioName}
                        </Link>
                      </td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">
                        <Link
                          className="text-cyan underline underline-offset-3"
                          to={`/profiles/${run.userHandle || run.userDisplayName}`}
                        >
                          {run.userDisplayName || run.userHandle}
                        </Link>
                      </td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">{Math.round(run.score).toLocaleString()}</td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">{run.accuracy.toFixed(1)}%</td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">{formatDurationMs(run.durationMs)}</td>
                      <td className="px-3 py-2.5 text-muted md:px-4 md:py-3">{formatRelativeTime(run.playedAtIso)}</td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">
                        <Link
                          className="text-cyan underline underline-offset-3"
                          to={`/runs/${run.runId || run.sessionId}`}
                        >
                          Open
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </ScrollArea>
            {hasMoreRuns && (
              <button
                onClick={() => setRunsVisible((n) => n + PAGE_SIZE)}
                className="mt-3 w-full rounded-[14px] border border-line py-2.5 text-sm text-muted transition-colors hover:border-cyan/30 hover:text-text"
              >
                Load {Math.min(PAGE_SIZE, (overview?.recentRuns.length ?? 0) - runsVisible)} more runs
              </button>
            )}
          </>
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
