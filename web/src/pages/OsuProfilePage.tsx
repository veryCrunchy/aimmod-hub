import { useEffect, useMemo, useState } from "react";
import { PageSeo } from "../components/PageSeo";
import { useScorePp } from "../hooks/useScorePp";
import { ScorePpStatus } from "../components/ScorePpStatus";
import { useParams, useSearchParams } from "react-router-dom";
import { ScoreBrowserControls } from "../components/OsuScoreFilters";
import { filterOsuScores, scoreFilterKeys } from "../lib/osuScoreFilters";
import { OsuReplayRow } from "../components/OsuReplayRow";
import { PlayerIdentity, MetricStrip } from "../components/BrowseHeader";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { Skeleton } from "../components/ui/Skeleton";
import { Button } from "../components/ui/Button";
import { fetchOsuScoreHistory, type OsuScoreHistory, type OsuPublicProfile } from "../lib/osuCommunity";

export function OsuProfilePage() {
  const { handle = "" } = useParams();
  const [profile, setProfile] = useState<OsuPublicProfile | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [history, setHistory] = useState<OsuScoreHistory | null>(null);
  const [params, setParams] = useSearchParams();
  const selectedMode = params.get("mode") ?? "osu";
  const mode = ["osu", "taiko", "fruits", "mania"].includes(selectedMode) ? selectedMode : "osu";
  const setMode = (value: string) => setParams(previous => { const next = new URLSearchParams(previous); next.set("mode", value); return next; }, { replace: true });
  const pp = useScorePp(profile?.recentReplays);
  const visible = useMemo(() => filterOsuScores(pp.items, params), [pp.items, params]);

  useEffect(() => {
    let cancelled = false;
    setProfile(null);
    setHistory(null);
    setError(null);
    void fetchOsuScoreHistory(handle, mode)
      .then((value) => { if (!cancelled) { setHistory(value); setProfile({ ...value.profile, recentReplays: value.items }); } })
      .catch((reason) => { if (!cancelled) setError(reason instanceof Error ? reason.message : "Could not load this osu! profile."); });
    return () => { cancelled = true; };
  }, [handle, attempt, mode]);

  if (error) {
    return <PageStack><PageSeo title="Profile unavailable · AimMod Hub" description="This profile is unavailable." noindex /><PageSection><EmptyState title="Profile unavailable" body="The profile may be private, or the service may be temporarily unavailable."><Button onClick={() => setAttempt(value => value + 1)}>Try again</Button><Button to="/osu/players">Browse players</Button></EmptyState></PageSection></PageStack>;
  }
  if (!profile) {
    return <PageStack><PageSeo title="osu! player · AimMod Hub" description="Loading public player." noindex /><PageSection role="status" aria-label="Loading player profile"><p className="mb-3 text-muted">Loading player profile...</p><Skeleton className="h-28" /></PageSection><PageSection><Skeleton className="h-72" /></PageSection></PageStack>;
  }

  return (
    <PageStack>
      <PageSeo title={`${profile.osuUsername} · osu! · AimMod Hub`} type="profile"
        description={`${profile.osuUsername}'s public osu! replay analysis and performance history on AimMod Hub.`} />
      <section className="profile-heading">
        <PlayerIdentity name={profile.osuUsername} avatar={profile.avatarUrl} detail={[profile.countryCode, profile.hubHandle ? `@${profile.hubHandle}` : ""].filter(Boolean).join(" · ")} heading />
        <label className="text-sm text-muted">Ruleset <select value={mode} onChange={event => setMode(event.target.value)}><option value="osu">osu!</option><option value="taiko">osu!taiko</option><option value="fruits">osu!catch</option><option value="mania">osu!mania</option></select></label>
      </section>
      <MetricStrip metrics={[
        { label: "Performance", value: profile.performancePoints == null ? "—" : `${Math.round(profile.performancePoints).toLocaleString()} pp` },
        { label: "Global rank", value: profile.globalRank == null ? "Unranked" : `#${profile.globalRank.toLocaleString()}` },
        { label: "Play count", value: profile.playCount.toLocaleString() },
        { label: "Shared replays", value: profile.sharedReplayCount.toLocaleString() },
      ]} />
      <PageSection className="p-0 overflow-hidden">
        <ScoreBrowserControls />
        <ScorePpStatus {...pp} />
        <p className="hub-results" role="status">{visible.length} of {profile.recentReplays.length} loaded plays</p>
        {profile.recentReplays.length > 0
          ? visible.length ? visible.map((replay) => <OsuReplayRow key={replay.shareId || replay.officialScoreId} replay={replay} />)
            : <EmptyState title="No matching plays" body="Try a wider range or clear your filters."><Button onClick={() => setParams(previous => { const next = new URLSearchParams(previous); scoreFilterKeys.forEach(key => next.delete(key)); return next; })}>Clear filters</Button></EmptyState>
          : <EmptyState title="No scores to display" body="No plays were returned for this ruleset. Check source availability or choose another ruleset." />}
      </PageSection>
      {history && <div className="profile-coverage"><p>{history.coverage.best.fetched} best scores · {history.coverage.recent.fetched} recent scores · Public uploads. This is not a complete play history.{history.hasMore ? " Showing the first 100 matching plays." : ""}</p>{[history.coverage.best, history.coverage.recent].some(source => !["available", "page_limit"].includes(source.status)) && <p role="status">Some scores could not load. <button className="text-cyan underline" onClick={() => setAttempt(value => value + 1)}>Try again</button></p>}</div>}
    </PageStack>
  );
}
