import { useEffect, useState } from "react";
import type { GetAimProfileResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { GetAimProfileRequest } from "../gen/aimmod/hub/v1/hub_pb";
import { AimProfileRadar } from "./charts/AimProfileRadar";
import { SectionHeader } from "./SectionHeader";
import { PageSection } from "./ui/PageSection";
import { Skeleton } from "./ui/Skeleton";
import { Grid } from "./ui/Stack";
import { hubClient, displayScenarioType } from "../lib/api";

const TYPE_COLORS: Record<string, string> = {
  Tracking: "text-cyan border-cyan/20 bg-cyan/5",
  MultiHitClicking: "text-gold border-gold/20 bg-gold/5",
  ReactiveClicking: "text-violet border-violet/20 bg-violet/5",
  OneShotClicking: "text-mint border-mint/20 bg-mint/5",
  AccuracyDrill: "text-muted border-line bg-white/2",
};

function typeColor(t: string) {
  return TYPE_COLORS[t] ?? "text-muted border-line bg-white/2";
}

function percentileLabel(pct: number): { label: string; color: string } {
  if (pct >= 90) return { label: "Top 10%", color: "text-mint" };
  if (pct >= 75) return { label: "Top 25%", color: "text-cyan" };
  if (pct >= 50) return { label: "Above avg", color: "text-text" };
  if (pct >= 25) return { label: "Below avg", color: "text-gold" };
  return { label: "Bottom 25%", color: "text-muted" };
}

type Props = { handle: string };

export function AimProfileSection({ handle }: Props) {
  const [profile, setProfile] = useState<GetAimProfileResponse | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void hubClient
      .getAimProfile(new GetAimProfileRequest({ handle }))
      .then((r) => { if (!cancelled) { setProfile(r); setLoading(false); } })
      .catch(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [handle]);

  if (loading) {
    return (
      <PageSection className="h-full">
        <Skeleton className="mb-3 h-3 w-24" />
        <Skeleton className="mb-4 h-6 w-36" />
        <Skeleton className="h-[210px]" />
      </PageSection>
    );
  }

  if (!profile || profile.typeBands.length === 0) return null;

  const overallPct = percentileLabel(profile.overallAccuracyPercentile);

  return (
    <PageSection className="h-full">
      <SectionHeader
        eyebrow="Aim profile"
        title="Performance by scenario type"
        body="Accuracy percentile vs the community across the scenario families this player has practiced."
      />

      <Grid className="grid-cols-[minmax(0,1fr)_220px] items-start gap-4 max-[900px]:grid-cols-1">
        {/* radar — only shown with 2+ types */}
        {profile.typeBands.length >= 2 && <AimProfileRadar bands={profile.typeBands} />}

        {/* summary sidebar */}
        <div className="grid gap-2.5 min-w-[200px] max-[900px]:grid-cols-2 max-[600px]:grid-cols-1">
          <div className="rounded-[14px] border border-line bg-white/2 p-3">
            <p className="text-[10px] uppercase tracking-[0.1em] text-muted-2">Overall</p>
            <p className={`mt-1 text-xl font-medium ${overallPct.color}`}>
              {Math.round(profile.overallAccuracyPercentile)}th
            </p>
            <p className={`mt-0.5 text-[11px] ${overallPct.color}`}>{overallPct.label}</p>
            <p className="mt-1 text-[11px] text-muted-2">{profile.overallAccuracy.toFixed(1)}% avg acc</p>
          </div>

          {profile.strongestType && (
            <div className={`rounded-[14px] border p-3 ${typeColor(profile.strongestType)}`}>
              <p className="text-[10px] uppercase tracking-[0.1em] opacity-60">Strongest</p>
              <p className="mt-1 text-sm font-medium">
                {displayScenarioType(profile.strongestType) ?? profile.strongestType}
              </p>
              <p className="mt-1 text-[11px] opacity-60">Highest accuracy %ile</p>
            </div>
          )}

          {profile.mostPracticedType && profile.mostPracticedType !== profile.strongestType && (
            <div className={`rounded-[14px] border p-3 ${typeColor(profile.mostPracticedType)}`}>
              <p className="text-[10px] uppercase tracking-[0.1em] opacity-60">Most practiced</p>
              <p className="mt-1 text-sm font-medium">
                {displayScenarioType(profile.mostPracticedType) ?? profile.mostPracticedType}
              </p>
              <p className="mt-1 text-[11px] opacity-60">Highest run volume</p>
            </div>
          )}
        </div>
      </Grid>

      {/* per-type breakdown */}
      <div className="mt-4 grid gap-2">
        {profile.typeBands.map((band) => {
          const pct = percentileLabel(band.accuracyPercentile);
          const barWidth = Math.min(100, Math.max(2, band.accuracyPercentile));
          const communityBar = 50;
          return (
            <div key={band.scenarioType} className="rounded-[12px] border border-line bg-white/2 px-3 py-2.5">
              <div className="flex items-center justify-between gap-4">
                <div className="flex items-center gap-3 min-w-0">
                  <span className="shrink-0 text-sm font-medium text-text">
                    {displayScenarioType(band.scenarioType) ?? band.scenarioType}
                  </span>
                  <span className="text-[10px] text-muted-2">{band.runCount.toLocaleString()} runs</span>
                </div>
                <div className="flex items-center gap-3 shrink-0">
                  <span className="text-[11px] text-muted">{band.avgAccuracy.toFixed(1)}% acc</span>
                  <span className={`text-sm font-medium tabular-nums ${pct.color}`}>
                    {Math.round(band.accuracyPercentile)}th
                  </span>
                </div>
              </div>
              {/* progress bar: player (cyan) vs community 50th (gold dashed) */}
              <div className="relative mt-1.5 h-1.5 w-full rounded-full bg-white/8 overflow-hidden">
                <div
                  className="absolute inset-y-0 left-0 h-full rounded-full bg-cyan/50 transition-all"
                  style={{ width: `${barWidth}%` }}
                />
                {/* community 50th marker */}
                <div
                  className="absolute inset-y-0 w-px bg-gold/40"
                  style={{ left: `${communityBar}%` }}
                />
              </div>
              {band.avgSmoothness > 0 && (
                <p className="mt-1 text-[10px] text-muted-2">
                  Smoothness {Math.round(band.avgSmoothness)}/100
                </p>
              )}
            </div>
          );
        })}
      </div>
    </PageSection>
  );
}
