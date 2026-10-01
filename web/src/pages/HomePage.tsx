import { useCallback, useEffect, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link } from "react-router-dom";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { PlayerLookup } from "../components/PlayerLookup";
import { RunTable } from "../components/RunTable";
import { StatCard } from "../components/StatCard";
import { VerificationBadge } from "../components/VerificationBadge";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageHeader } from "../components/ui/PageHeader";
import { Section } from "../components/ui/Section";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useAuth } from "../lib/AuthContext";
import { displayScenarioType, fetchLiveActivityFeed, fetchOverview, slugifyScenarioName, type LiveHubActivity } from "../lib/api";
import { formatPlaytime, formatScore } from "../lib/kovaaksStats";
import { liveView, sortLive } from "../lib/liveActivity";

function LiveStrip({ items }: { items: LiveHubActivity[] }) {
  const playing = sortLive(items).filter((item) => item.scenarioName?.trim()).slice(0, 4);
  if (playing.length === 0) return null;
  const now = Date.now();
  return (
    <Section title={<><span aria-hidden="true" className="mr-2 inline-block h-2 w-2 rounded-full bg-mint align-middle" />Live now</>} aside={<Link to="/live" className="text-cyan hover:underline">{items.length} online</Link>}>
      <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
        {playing.map((item) => {
          const view = liveView(item, now);
          return (
            <li key={item.userHandle} className="min-w-0 rounded-md border border-line bg-panel px-3 py-2.5">
              <Link to={`/profiles/${view.handle}`} className="block truncate text-sm font-medium text-text hover:text-cyan">{view.name}</Link>
              <div className="truncate text-xs text-muted">{view.title}</div>
              <div className="mt-1 text-xs tabular-nums text-muted-2">{formatScore(view.score)} · {view.accuracy != null ? `${view.accuracy.toFixed(1)}%` : "—"}{view.timer ? ` · ${view.timer}` : ""}</div>
            </li>
          );
        })}
      </ul>
    </Section>
  );
}

export function HomePage() {
  const auth = useAuth();
  const [overview, setOverview] = useState<GetOverviewResponse | null>(null);
  const [live, setLive] = useState<LiveHubActivity[]>([]);
  const [error, setError] = useState(false);

  const load = useCallback(() => {
    void fetchOverview()
      .then((next) => { setOverview(next); setError(false); })
      .catch(() => setError(true));
    void fetchLiveActivityFeed(50)
      .then((response) => setLive(response.items.filter((item) => item.active && item.userHandle?.trim())))
      .catch(() => {});
  }, []);
  useEffect(load, [load]);
  useAutoRefresh(load, 30_000);

  const head = (
    <Helmet>
      <title>KovaaK's overview · AimMod Hub</title>
      <meta name="description" content="Recent KovaaK's runs, popular scenarios and active players shared through AimMod." />
    </Helmet>
  );

  if (!overview) {
    return <PageStack>
      {head}
      {error
        ? <><PageHeader title="KovaaK's" /><EmptyState title="Stats could not be loaded."><Button onClick={load}>Try again</Button></EmptyState></>
        : <PageSkeleton label="Loading overview" />}
    </PageStack>;
  }

  const totalRuns = Number(overview.totalRuns);
  const scenarioMax = Math.max(1, ...overview.topScenarios.map((s) => Number(s.runCount)));
  const ownHandle = auth.user?.profileHandle;

  return (
    <PageStack>
      {head}
      <PageHeader
        title="KovaaK's"
        meta={`${overview.totalPlayers.toLocaleString()} players · ${totalRuns.toLocaleString()} runs shared through AimMod`}
        actions={ownHandle ? <Button to={`/profiles/${ownHandle}`}>My profile</Button> : <Button to="/app/kovaaks" variant="primary">Get AimMod</Button>}
      />

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard label="Runs this week" value={overview.runsLast7Days.toLocaleString()} detail={`${overview.playersLast7Days.toLocaleString()} ${overview.playersLast7Days === 1 ? "player" : "players"} active`} />
        <StatCard label="Time played" value={formatPlaytime(overview.totalDurationMs)} detail="All shared runs" />
        <StatCard label="Players" value={overview.totalPlayers.toLocaleString()} />
        <StatCard label="Scenarios" value={overview.totalScenarios.toLocaleString()} />
      </div>

      <LiveStrip items={live} />

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
        <Section title="Recent runs" aside={<Link to="/leaderboard" className="text-cyan hover:underline">Leaderboards</Link>}>
          {overview.recentRuns.length
            ? <RunTable runs={overview.recentRuns.slice(0, 15)} caption="Recent runs" />
            : <EmptyState title="No runs shared yet."><Button to="/app/kovaaks">Get AimMod</Button></EmptyState>}
        </Section>

        <div className="grid gap-6">
          <Section title="Popular scenarios" aside={<Link to="/community" className="text-cyan hover:underline">All</Link>}>
            {overview.topScenarios.length ? (
              <ol className="grid gap-1">
                {overview.topScenarios.slice(0, 8).map((scenario) => (
                  <li key={scenario.scenarioSlug}>
                    <Link to={`/scenarios/${scenario.scenarioSlug || slugifyScenarioName(scenario.scenarioName)}`} className="group block rounded px-2 py-1.5 hover:bg-panel">
                      <div className="flex items-baseline justify-between gap-3 text-sm">
                        <span className="truncate text-text group-hover:text-cyan">{scenario.scenarioName}</span>
                        <span className="shrink-0 tabular-nums text-muted">{scenario.runCount.toLocaleString()} runs</span>
                      </div>
                      <div className="mt-1 h-1 rounded-full bg-bg-2" aria-hidden="true">
                        <div className="h-1 rounded-full bg-mint/60" style={{ width: `${(Number(scenario.runCount) / scenarioMax) * 100}%` }} />
                      </div>
                    </Link>
                  </li>
                ))}
              </ol>
            ) : <EmptyState title="No scenarios yet." />}
          </Section>

          <Section title="Most active players" aside={<Link to="/community#players" className="text-cyan hover:underline">All</Link>}>
            {overview.activeProfiles.length ? (
              <ol className="grid gap-1">
                {overview.activeProfiles.slice(0, 8).map((profile) => (
                  <li key={profile.userHandle}>
                    <Link to={`/profiles/${profile.userHandle}`} className="flex items-center justify-between gap-3 rounded px-2 py-1.5 text-sm hover:bg-panel">
                      <span className="flex min-w-0 items-center gap-1.5">
                        <span className="truncate text-text">{profile.userDisplayName || profile.userHandle}</span>
                        <VerificationBadge verified={Boolean(profile.isVerified)} />
                      </span>
                      <span className="shrink-0 text-xs tabular-nums text-muted">{profile.runCount.toLocaleString()} runs · {displayScenarioType(profile.primaryScenarioType) ?? "Mixed"}</span>
                    </Link>
                  </li>
                ))}
              </ol>
            ) : <EmptyState title="No players yet." />}
          </Section>
        </div>
      </div>

      <Section title="Look up a KovaaK's player">
        <PlayerLookup />
      </Section>
    </PageStack>
  );
}
