import { useMemo, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { osuClient } from "../lib/osuCatalog";
import { useOsuDirectory } from "../hooks/useOsuDirectory";
import { useScorePp } from "../hooks/useScorePp";
import { ScorePpStatus } from "../components/ScorePpStatus";
import { groupOsuPlays } from "../lib/osuDirectory";
import { OsuReplayRow } from "../components/OsuReplayRow";
import { SectionHeader } from "../components/SectionHeader";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";

export function OsuDirectoryPage({ view = "overview" }: { view?: "overview" | "beatmaps" | "players" }) {
  const { items, error, retry } = useOsuDirectory();
  const pp = useScorePp(items);
  const navigate = useNavigate();
  const [finding, setFinding] = useState(false);
  const [lookupError, setLookupError] = useState("");
  const [params, setParams] = useSearchParams();
  const query = params.get("q") ?? "";
  const sort = params.get("sort") ?? "recent";
  const title = view === "overview" ? "osu! overview" : view === "players" ? "Players" : "Beatmaps";
  const groups = useMemo(() => {
    const term = query.trim().toLowerCase();
    return groupOsuPlays(pp.items, view === "players" ? "players" : "beatmaps")
      .filter(group => (view === "players"
        ? [group.latest.osuUsername, group.latest.hubHandle, group.latest.hubDisplayName]
        : [group.latest.title, group.latest.artist, group.latest.creator, group.latest.difficulty])
        .some(value => value.toLowerCase().includes(term)))
      .sort((a,b) => sort === "plays" ? b.plays.length - a.plays.length : sort === "pp"
        ? (b.bestPP ?? -1) - (a.bestPP ?? -1)
        : Date.parse(b.latest.playedAt) - Date.parse(a.latest.playedAt));
  }, [pp.items, view, query, sort]);

  function update(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value); else next.delete(key);
    setParams(next, { replace: true });
  }

  return <PageStack>
    <Helmet><title>{title} · AimMod Hub</title></Helmet>
    <PageSection>
      <SectionHeader level={1} eyebrow="osu!" title={title}
        body={view === "overview" ? "Beatmaps, players, and replays from the community." : view === "players" ? "Find players through their public shared plays." : "Explore difficulties played and shared by the community."}
        aside={<Button to="/app/osu">Get AimMod for osu!</Button>} />
      {view !== "overview" && <div className="hub-filters">
        <label>{view === "players" ? "Find a player" : "Find a beatmap"}<input type="search" value={query} placeholder={view === "players" ? "Player name or handle" : "Title, difficulty, artist, or mapper"} onChange={e => update("q",e.target.value)} /></label>
        <label>Sort by<select value={sort} onChange={e => update("sort",e.target.value)}><option value="recent">Latest activity</option><option value="plays">Most shared plays</option><option value="pp">Best shared PP</option></select></label>
        {(query || sort !== "recent") && <Button onClick={() => setParams({})}>Reset filters</Button>}
        {view === "players" && <Button disabled={finding || !query.trim()} onClick={async () => {
          setFinding(true); setLookupError("");
          try {
            const response = await osuClient.getOfficialUserProfile({ identifier: query.trim(), ruleset: 1 }, { signal: AbortSignal.timeout(15000) });
            if (!response.profile?.userId) throw new Error("Player not found or osu! is unavailable.");
            navigate(`/osu/profiles/${response.profile.userId}`);
          } catch { setLookupError("Could not find this osu! player. Check the username or user ID and try again."); }
          finally { setFinding(false); }
        }}>{finding ? "Finding player..." : "Find on osu!"}</Button>}
      </div>}
      {lookupError && <p role="alert" className="py-3 text-muted">{lookupError}</p>}
      {error ? <EmptyState title="Community activity is unavailable" body="Please try again in a moment."><Button onClick={retry}>Try again</Button></EmptyState>
        : !items ? <div role="status"><p className="mb-4 text-muted">Loading osu! activity...</p><Skeleton className="h-64" /></div>
        : <>
          <p className="hub-results">Based on the latest {items.length} public shared plays.</p>
          <ScorePpStatus pending={pp.pending} failed={pp.failed} retry={pp.retry} />
          {view === "overview" ? <>
            <div className="grid grid-cols-3 gap-3 border-y border-line py-5">
              {[[String(items.length),"Shared plays","/osu/community"],[String(groupOsuPlays(items,"beatmaps").length),"Beatmaps","/osu/beatmaps"],[String(groupOsuPlays(items,"players").length),"Players","/osu/players"]].map(([count,label,to])=><Link key={to} to={to} className="min-w-0"><strong className="block text-2xl font-semibold">{count}</strong><span className="text-sm text-muted">{label}</span></Link>)}
            </div>
            <div className="mt-7 mb-3 flex items-center justify-between"><h2 className="text-xl font-semibold">Recent plays</h2><Link to="/osu/community" className="text-sm text-cyan">View all</Link></div>
            {pp.items.slice(0,8).map(replay => <OsuReplayRow key={replay.shareId} replay={replay} />)}
            {items.length === 0 && <EmptyState title="No public plays yet" body="Share a play from AimMod to add it to the community." />}
          </> : groups.length === 0 ? <EmptyState title="No matches" body="Try another search or clear your filters."><Button onClick={() => setParams({})}>Clear filters</Button></EmptyState>
          : <div className="divide-y divide-line border-y border-line">{groups.map(group => {
            const replay=group.latest;
            const src=view === "players" ? replay.avatarUrl : replay.coverUrl;
            return <Link key={group.id} to={view === "players" ? `/osu/profiles/${encodeURIComponent(group.id)}` : `/osu/community?beatmap=${group.id}`} className="grid grid-cols-[52px_minmax(0,1fr)_auto] items-center gap-3 py-4 hover:bg-panel">
              {src ? <img src={src} width="52" height="52" alt="" loading="lazy" className="h-13 w-13 rounded object-cover" /> : <span className="h-13 w-13 rounded bg-panel" />}
              <span className="min-w-0"><strong className="block truncate text-sm">{view === "players" ? replay.osuUsername : replay.title}</strong><span className="block truncate text-xs text-muted">{view === "players" ? `@${replay.hubHandle}` : `${replay.artist} · [${replay.difficulty}] · ${replay.creator}`}</span><span className="mt-1 block text-xs text-muted">{group.plays.length} shared {group.plays.length===1?"play":"plays"}</span></span>
              <span className="text-right"><strong className="block text-sm text-mint">{group.bestPP == null ? "—" : `${Math.round(group.bestPP)}pp`}</strong><span className="block text-[10px] text-muted">Best shared</span></span>
            </Link>;
          })}</div>}
        </>}
    </PageSection>
  </PageStack>;
}
