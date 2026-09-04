import { useMemo } from "react";
import { Helmet } from "react-helmet-async";
import { useSearchParams } from "react-router-dom";
import { SectionHeader } from "../components/SectionHeader";
import { OsuReplayRow } from "../components/OsuReplayRow";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { Skeleton } from "../components/ui/Skeleton";
import { useOsuDirectory } from "../hooks/useOsuDirectory";

export function OsuCommunityPage({ replayLibrary = false }: { replayLibrary?: boolean }) {
  const { items: replays, error, retry } = useOsuDirectory();
  const [params, setParams] = useSearchParams();
  const query = params.get("q") ?? "";
  const sort = params.get("sort") ?? "recent";
  const filter = params.get("replay") ?? (replayLibrary ? "file" : "all");
  const beatmap = params.get("beatmap");


  const visible = useMemo(() => {
    const term = query.trim().toLowerCase();
    return (replays ?? []).filter(replay =>
      (!beatmap || String(replay.beatmapId) === beatmap) &&
      (filter !== "file" || replay.hasReplayFile) &&
      [replay.title, replay.artist, replay.difficulty, replay.creator, replay.osuUsername, replay.hubHandle]
        .some(value => value.toLowerCase().includes(term))
    ).sort((a, b) => sort === "pp"
      ? (b.performancePoints ?? -1) - (a.performancePoints ?? -1)
      : sort === "accuracy" ? b.accuracy - a.accuracy
      : Date.parse(b.playedAt) - Date.parse(a.playedAt));
  }, [replays, query, sort, filter, beatmap]);

  function update(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value); else next.delete(key);
    setParams(next, { replace: true });
  }

  return (
    <PageStack>
      <Helmet>
        <title>{replayLibrary ? "osu! replay library" : "osu! community"} · AimMod Hub</title>
        <meta name="description" content="Public osu!standard scores, replay analyses, and player profiles shared through AimMod." />
      </Helmet>
      <PageSection>
        <SectionHeader level={1} eyebrow="osu! community" title={replayLibrary ? "Replay library" : "Shared plays"} body="Explore beatmaps, compare scores, and review players' analysis." aside={<Button to="/app/osu">Get AimMod for osu!</Button>} />
        {beatmap && <p className="mb-3 text-sm text-muted">Beatmap #{beatmap} · {replays?.find(item => String(item.beatmapId) === beatmap)?.difficulty}</p>}
        <div className="hub-filters">
          <label>Find a play<input type="search" value={query} onChange={e => update("q", e.target.value)} placeholder="Beatmap, player, or mapper" /></label>
          <label>Replay<select value={filter} onChange={e => update("replay", e.target.value)}><option value="all">All shared plays</option><option value="file">Download available</option></select></label>
          <label>Sort by<select value={sort} onChange={e => update("sort", e.target.value)}><option value="recent">Newest played</option><option value="pp">Highest PP</option><option value="accuracy">Highest accuracy</option></select></label>
          {(query || beatmap || filter !== (replayLibrary ? "file" : "all") || sort !== "recent") && <Button onClick={() => setParams({})}>Reset filters</Button>}
        </div>
        {replays && <p className="hub-results" role="status">{visible.length} of {replays.length} recent shared plays</p>}
        {error ? (
          <EmptyState title="Community plays are unavailable" body="Please try again in a moment."><Button onClick={retry}>Try again</Button></EmptyState>
        ) : replays === null ? (
          <div role="status" aria-label="Loading shared plays" className="grid gap-3">
            <span className="text-sm text-muted">Loading shared plays...</span>
            {[0, 1, 2, 3, 4].map(index => <Skeleton key={index} className="h-[76px] rounded-md" />)}
          </div>
        ) : replays.length === 0 ? (
          <EmptyState title="No public osu! plays yet" body="Share a play from AimMod to add it to the community." />
        ) : visible.length === 0 ? (
          <EmptyState title="No matching plays" body="Try another beatmap or player, or clear your filters."><Button onClick={() => setParams({})}>Clear filters</Button></EmptyState>
        ) : <div>{visible.map(replay => <OsuReplayRow key={replay.shareId} replay={replay} />)}</div>}
      </PageSection>
    </PageStack>
  );
}
