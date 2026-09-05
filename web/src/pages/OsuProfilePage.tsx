import { useEffect, useState } from "react";
import { PageSeo } from "../components/PageSeo";
import { useParams } from "react-router-dom";
import { OsuReplayRow } from "../components/OsuReplayRow";
import { SectionHeader } from "../components/SectionHeader";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { Skeleton } from "../components/ui/Skeleton";
import { Button } from "../components/ui/Button";
import { fetchOsuProfile, type OsuPublicProfile } from "../lib/osuCommunity";

export function OsuProfilePage() {
  const { handle = "" } = useParams();
  const [profile, setProfile] = useState<OsuPublicProfile | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setProfile(null);
    setError(null);
    void fetchOsuProfile(handle)
      .then((value) => { if (!cancelled) setProfile(value); })
      .catch((reason) => { if (!cancelled) setError(reason instanceof Error ? reason.message : "Could not load this osu! profile."); });
    return () => { cancelled = true; };
  }, [handle, attempt]);

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
      <PageSection className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-5 max-[560px]:grid-cols-1">
        {profile.avatarUrl ? <img src={profile.avatarUrl} alt="" className="h-20 w-20 rounded-md object-cover" /> : <div className="h-20 w-20 rounded-md bg-white/5" />}
        <div className="min-w-0">
          <SectionHeader level={1} eyebrow={`@${profile.hubHandle}`} title={profile.osuUsername} body={profile.hubDisplayName} />
          <div className="mt-4 flex flex-wrap gap-x-6 gap-y-2 text-[12px] text-muted">
            <Stat label="Performance" value={profile.performancePoints == null ? "Unavailable" : `${Math.round(profile.performancePoints).toLocaleString()}pp`} />
            <Stat label="Global rank" value={profile.globalRank == null ? "Unranked" : `#${profile.globalRank.toLocaleString()}`} />
            <Stat label="Play count" value={profile.playCount.toLocaleString()} />
            <Stat label="Shared replays" value={profile.sharedReplayCount.toLocaleString()} />
          </div>
        </div>
      </PageSection>
      <PageSection className="p-0 overflow-hidden">
        {profile.recentReplays.length > 0
          ? profile.recentReplays.map((replay) => <OsuReplayRow key={replay.shareId} replay={replay} />)
          : <EmptyState title="No public replay analysis" body="This profile has not made any osu! plays public." />}
      </PageSection>
    </PageStack>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return <span><span className="text-muted-2">{label}</span> <strong className="ml-1 font-semibold text-text tabular-nums">{value}</strong></span>;
}
