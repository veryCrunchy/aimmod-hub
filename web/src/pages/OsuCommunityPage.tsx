import { useMemo } from "react";
import { Helmet } from "../lib/helmet";
import { useSearchParams } from "react-router-dom";
import { SectionHeader } from "../components/SectionHeader";
import { OsuReplayRow } from "../components/OsuReplayRow";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { Skeleton } from "../components/ui/Skeleton";
import { useOsuDirectory } from "../hooks/useOsuDirectory";
import { OsuScoreFilters } from "../components/OsuScoreFilters";
import { filterOsuScores } from "../lib/osuScoreFilters";
import { useScorePp } from "../hooks/useScorePp";
import { ScorePpStatus } from "../components/ScorePpStatus";

export function OsuCommunityPage({ replayLibrary = false }: { replayLibrary?: boolean }) {
  const { items: replays, error, retry } = useOsuDirectory(replayLibrary);
  const [params, setParams] = useSearchParams();
  const pp = useScorePp(replays);
  const beatmap = params.get("beatmap");


  const visible = useMemo(() => filterOsuScores(pp.items, params, replayLibrary ? "file" : "all"), [pp.items, params, replayLibrary]);

  return (
    <PageStack>
      <Helmet>
        <title>{replayLibrary ? "osu! replay library" : "osu! community"} · AimMod Hub</title>
        <meta name="description" content="Public osu! scores, replays and player profiles." />
      </Helmet>
      <PageSection>
        <SectionHeader level={1} eyebrow="osu! community" title={replayLibrary ? "Replay library" : "Public plays"} body="Explore beatmaps, compare scores, and review players' analysis." aside={<Button to="/app/osu">Get AimMod for osu!</Button>} />
        {beatmap && <p className="mb-3 text-sm text-muted">Beatmap #{beatmap} · {replays?.find(item => String(item.beatmapId) === beatmap)?.difficulty}</p>}
        <OsuScoreFilters defaultReplay={replayLibrary ? "file" : "all"} />
        <ScorePpStatus {...pp} />
        {replays && <p className="hub-results" role="status">{visible.length} of {replays.length} recent public plays</p>}
        {error ? (
          <EmptyState title="Community plays are unavailable" body="Please try again in a moment."><Button onClick={retry}>Try again</Button></EmptyState>
        ) : replays === null ? (
          <div role="status" aria-label="Loading public plays" className="grid gap-3">
            <span className="text-sm text-muted">Loading public plays...</span>
            {[0, 1, 2, 3, 4].map(index => <Skeleton key={index} className="h-[76px] rounded-md" />)}
          </div>
        ) : replays.length === 0 ? (
          <EmptyState title="No public osu! plays yet" body="Public plays will appear here as they become available." />
        ) : visible.length === 0 ? (
          <EmptyState title="No matching plays" body="Try another beatmap or player, or clear your filters."><Button onClick={() => setParams({})}>Clear filters</Button></EmptyState>
        ) : <div>{visible.map(replay => <OsuReplayRow key={replay.shareId || replay.officialScoreId} replay={replay} />)}</div>}
      </PageSection>
    </PageStack>
  );
}
