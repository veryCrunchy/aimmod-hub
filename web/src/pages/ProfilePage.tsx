import { useCallback, useEffect, useMemo, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useParams } from "react-router-dom";
import type { ActivityDay, GetProfileResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { ActivityStrip, CompareLauncher, PlayerScenarioStats } from "../components/PlayerScenarioStats";
import { ReplayResultCard } from "../components/ReplayResultCard";
import { BenchmarkSummaryGrid, hasRank } from "../components/BenchmarkCards";
import { RunTrendChart } from "../components/charts/RunTrendChart";
import { ScenarioTypeChart } from "../components/charts/ScenarioTypeChart";
import { RunTable } from "../components/RunTable";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { StatCard } from "../components/StatCard";
import { VerificationBadge } from "../components/VerificationBadge";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageHeader } from "../components/ui/PageHeader";
import { Section } from "../components/ui/Section";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { AimProfileSection } from "../components/AimProfileSection";
import { AimFingerprintSection } from "../components/AimFingerprintSection";
import { useAnimatedNumber } from "../hooks/useAnimatedNumber";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useNow } from "../hooks/useNow";
import { useAuth } from "../lib/AuthContext";
import { displayScenarioType, fetchBenchmarkList, fetchLiveActivity, fetchPlayerScenarioStats, fetchProfile, fetchReplayHub, formatRelativeTime, subscribeLiveActivityFeed, type HubSearchRun, type LiveHubActivity } from "../lib/api";
import { accuracyTrend, formatAccuracy, formatPlaytime, formatPoints, formatScore, newestFirst } from "../lib/kovaaksStats";
import { liveView, ownSessionNote } from "../lib/liveActivity";

const RUNS_SHOWN = 15;

function LiveNumber({ value, format }: { value: number | null; format: (value: number) => string }) {
  const animated = useAnimatedNumber(value, 650);
  return <>{animated != null ? format(animated) : "—"}</>;
}

function LiveBanner({ activity, own, nowMs }: { activity: LiveHubActivity; own: boolean; nowMs: number }) {
  const view = liveView(activity, nowMs);
  const note = own ? ownSessionNote(activity) : null;
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 rounded-md border border-mint/30 bg-mint/[0.06] px-4 py-3 text-sm" role="status">
      <span className="flex items-center gap-2 font-medium text-mint"><span aria-hidden="true" className="h-2 w-2 rounded-full bg-mint" />{view.status}</span>
      {view.phase !== "idle" && <>
        <span className="min-w-0 truncate text-text">{view.title}</span>
        <span className="tabular-nums text-muted">Score <LiveNumber value={view.score} format={(v) => Math.round(v).toLocaleString()} /></span>
        <span className="tabular-nums text-muted">Accuracy <LiveNumber value={view.accuracy} format={(v) => `${v.toFixed(1)}%`} /></span>
        {view.timer ? <span className="tabular-nums text-muted">{view.timer}</span> : null}
      </>}
      {view.session ? <span className="text-muted">{view.session}</span> : null}
      {note ? <span className="w-full text-xs text-gold">{note}</span> : null}
    </div>
  );
}

export function ProfilePage() {
  const { handle = "" } = useParams();
  const auth = useAuth();
  const [profile, setProfile] = useState<GetProfileResponse | null>(null);
  const [replays, setReplays] = useState<HubSearchRun[]>([]);
  const [liveActivity, setLiveActivity] = useState<LiveHubActivity | null>(null);
  const [error, setError] = useState(false);
  const [runsShown, setRunsShown] = useState(RUNS_SHOWN);
  const [activity, setActivity] = useState<ActivityDay[]>([]);
  const [hiddenBenchmarks, setHiddenBenchmarks] = useState<Set<number>>(new Set());

  useEffect(() => {
    let cancelled = false;
    // Test, empty and barely played benchmarks stay off the profile summary.
    void fetchBenchmarkList().then((r) => { if (!cancelled) setHiddenBenchmarks(new Set(r.benchmarks.filter((b) => b.hidden).map((b) => b.benchmarkId))); }).catch(() => {});
    return () => { cancelled = true; };
  }, []);
  const nowMs = useNow(1000);

  useEffect(() => {
    let cancelled = false;
    setProfile(null);
    setReplays([]);
    setLiveActivity(null);
    setError(false);
    setRunsShown(RUNS_SHOWN);
    void fetchProfile(handle)
      .then((next) => {
        if (cancelled) return;
        setProfile(next);
        const resolved = next.userHandle || handle;
        void fetchLiveActivity(resolved).then((activity) => { if (!cancelled) setLiveActivity(activity); }).catch(() => {});
        void fetchReplayHub({ handle: resolved, limit: 6 }).then((response) => { if (!cancelled) setReplays(response.items); }).catch(() => {});
        void fetchPlayerScenarioStats(resolved, "").then((stats) => { if (!cancelled) setActivity(stats.activity); }).catch(() => {});
      })
      .catch(() => { if (!cancelled) setError(true); });
    return () => { cancelled = true; };
  }, [handle]);

  const doRefresh = useCallback(() => {
    void fetchProfile(handle).then(setProfile).catch(() => {});
  }, [handle]);
  useAutoRefresh(doRefresh, 60_000);

  const refreshLive = useCallback(() => {
    if (!handle.trim()) return;
    void fetchLiveActivity(handle).then(setLiveActivity).catch(() => {});
  }, [handle]);
  useEffect(() => (handle.trim() ? subscribeLiveActivityFeed({ onUpdate: refreshLive }) : undefined), [refreshLive, handle]);
  useAutoRefresh(refreshLive, 30_000);

  const recentRuns = useMemo(() => newestFirst(profile?.recentRuns ?? []), [profile]);

  if (error) {
    return (
      <PageStack>
        <Helmet><title>{handle} · AimMod Hub</title></Helmet>
        <PageHeader title={`@${handle}`} />
        <EmptyState title="This player could not be found.">
          <Button to={`/search?q=${encodeURIComponent(handle)}`}>Search players</Button>
        </EmptyState>
      </PageStack>
    );
  }

  if (!profile) return <PageStack><PageSkeleton stats={5} label="Loading profile" /></PageStack>;

  const name = profile.userDisplayName || profile.userHandle;
  const own = Boolean(auth.user?.profileHandle) && auth.user?.profileHandle?.toLowerCase() === profile.userHandle.toLowerCase();
  const metaTitle = `${name} (@${profile.userHandle}) · AimMod Hub`;
  const metaDesc = `${profile.runCount.toLocaleString()} KovaaK's runs across ${profile.scenarioCount.toLocaleString()} scenarios.`;
  const mainType = displayScenarioType(profile.primaryScenarioType);
  const trend = accuracyTrend(profile.recentRuns);
  const lastPlayed = profile.lastPlayedAtIso || recentRuns[0]?.playedAtIso;
  const rankedBenchmarks = profile.benchmarks.filter((b) => hasRank(b.overallRank) && !hiddenBenchmarks.has(b.benchmarkId));
  const hasScenarioTypes = profile.topScenarios.some((s) => s.scenarioType?.trim() && s.scenarioType !== "Unknown");

  return (
    <PageStack>
      <Helmet>
        <title>{metaTitle}</title>
        <meta name="description" content={metaDesc} />
        <meta property="og:title" content={metaTitle} />
        <meta property="og:description" content={metaDesc} />
        <meta property="og:type" content="profile" />
      </Helmet>

      <PageHeader
        title={<span className="inline-flex items-center gap-2">{name}<VerificationBadge verified={Boolean(profile.isVerified)} /></span>}
        meta={[`@${profile.userHandle}`, lastPlayed ? `last played ${formatRelativeTime(lastPlayed)}` : null, mainType ? `plays mostly ${mainType.toLowerCase()}` : null].filter(Boolean).join(" · ")}
        actions={<>
          {rankedBenchmarks.length > 0 && <Button to={`/profiles/${profile.userHandle}/benchmarks`}>Benchmarks</Button>}
          {own && <Button to="/account">Account</Button>}
          <CompareLauncher handle={profile.userHandle} />
        </>}
      />

      {liveActivity?.active && <LiveBanner activity={liveActivity} own={own} nowMs={nowMs} />}

      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5">
        <StatCard label="Runs" value={profile.runCount.toLocaleString()} detail={`${profile.runsLast7Days.toLocaleString()} this week`} />
        <StatCard label="Time played" value={formatPlaytime(profile.totalDurationMs)} />
        <StatCard label="Scenarios" value={profile.scenarioCount.toLocaleString()} />
        <StatCard label="Average accuracy" value={formatAccuracy(profile.averageAccuracy)} detail={trend ? `${formatPoints(trend.changePts)} lately` : undefined} />
        <StatCard label="Personal bests" value={profile.personalBests.length.toLocaleString()} detail={rankedBenchmarks.length ? `${rankedBenchmarks.length} benchmark ranks` : undefined} />
      </div>

      <PlayerScenarioStats handle={profile.userHandle} />

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <Section title="Activity">
          {activity.length ? <ActivityStrip days={activity} /> : <EmptyState title="No runs in the last six months." />}
        </Section>
        <div className="grid gap-6">
          {profile.recentRuns.length >= 2 && (
            <Section title="Accuracy, recent runs" aside={trend ? `${formatAccuracy(trend.recent)} last ${Math.min(10, Math.floor(profile.recentRuns.length / 2))}` : undefined}>
              <RunTrendChart runs={profile.recentRuns} />
            </Section>
          )}
          {hasScenarioTypes && (
            <Section title="Runs by scenario type">
              <ScenarioTypeChart topScenarios={profile.topScenarios} />
            </Section>
          )}
        </div>
      </div>

      {rankedBenchmarks.length > 0 && (
        <Section title="Benchmark ranks" aside={<Link to={`/profiles/${profile.userHandle}/benchmarks`} className="text-cyan hover:underline">All benchmarks</Link>}>
          <BenchmarkSummaryGrid benchmarks={rankedBenchmarks} handle={profile.userHandle} />
        </Section>
      )}

      <Section title="Recent runs" aside={`${recentRuns.length} most recent`}>
        {recentRuns.length ? (
          <>
            <RunTable runs={recentRuns.slice(0, runsShown)} player={false} duration caption="Recent runs" />
            {recentRuns.length > runsShown && (
              <Button className="mt-3 w-full" onClick={() => setRunsShown((n) => n + RUNS_SHOWN)}>Show more runs</Button>
            )}
          </>
        ) : <EmptyState title="No runs shared yet." />}
      </Section>

      <div className="grid items-start gap-6 xl:grid-cols-2">
        <AimProfileSection handle={profile.userHandle} />
        <AimFingerprintSection handle={profile.userHandle} />
      </div>

      {profile.topScenarios.length > 0 && (
        <Section title="Scenarios played">
          <ul className="flex flex-wrap gap-2">
            {profile.topScenarios.map((scenario) => (
              <li key={scenario.scenarioSlug}>
                <Link to={`/profiles/${profile.userHandle}/scenarios/${scenario.scenarioSlug}`} className="inline-flex items-center gap-2 rounded-md border border-line bg-panel px-3 py-1.5 text-sm hover:border-line-strong">
                  <span className="text-text">{scenario.scenarioName}</span>
                  <span className="tabular-nums text-muted-2">{scenario.runCount.toLocaleString()}</span>
                  <ScenarioTypeBadge type={scenario.scenarioType} className="max-sm:hidden" />
                </Link>
              </li>
            ))}
          </ul>
        </Section>
      )}

      {replays.length > 0 && (
        <Section title="Replays" aside={<Link to={`/replays?q=${encodeURIComponent(profile.userHandle)}`} className="text-cyan hover:underline">All replays</Link>}>
          <div className="grid gap-3 lg:grid-cols-2">
            {replays.map((run) => <ReplayResultCard key={`profile-replay:${run.publicRunID || run.sessionID}`} run={run} />)}
          </div>
        </Section>
      )}
    </PageStack>
  );
}
