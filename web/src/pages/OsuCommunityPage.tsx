import { useEffect, useState } from "react";
import { Helmet } from "react-helmet-async";
import { SectionHeader } from "../components/SectionHeader";
import { OsuReplayRow } from "../components/OsuReplayRow";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { Skeleton } from "../components/ui/Skeleton";
import { fetchOsuCommunity, type OsuSharedReplay } from "../lib/osuCommunity";

export function OsuCommunityPage() {
  const [replays, setReplays] = useState<OsuSharedReplay[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void fetchOsuCommunity()
      .then((items) => { if (!cancelled) setReplays(items); })
      .catch((reason) => { if (!cancelled) setError(reason instanceof Error ? reason.message : "Could not load osu! activity."); });
    return () => { cancelled = true; };
  }, []);

  return (
    <PageStack>
      <Helmet>
        <title>osu! community · AimMod Hub</title>
        <meta name="description" content="Public osu!standard scores, replay analyses, and player profiles shared through AimMod." />
      </Helmet>
      <PageSection>
        <SectionHeader
          eyebrow="osu! community"
          title="Shared plays and analysis"
          body="Scores only appear here when their owner explicitly makes them public. Unlisted shares remain accessible only by their link."
        />
      </PageSection>
      <PageSection className="p-0 overflow-hidden">
        {error ? (
          <EmptyState title="Could not load community plays" body={error} />
        ) : replays === null ? (
          <div className="grid gap-px bg-line">
            {[0, 1, 2, 3, 4].map((index) => <Skeleton key={index} className="h-[72px] rounded-none" />)}
          </div>
        ) : replays.length === 0 ? (
          <EmptyState title="No public osu! plays yet" body="Public plays shared from the native AimMod client will appear here." />
        ) : (
          <div>{replays.map((replay) => <OsuReplayRow key={replay.shareId} replay={replay} />)}</div>
        )}
      </PageSection>
    </PageStack>
  );
}
