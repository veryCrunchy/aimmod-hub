import { useEffect, useState } from "react";
import {
  PolarAngleAxis,
  PolarGrid,
  PolarRadiusAxis,
  Radar,
  RadarChart,
  ResponsiveContainer,
} from "recharts";
import type { AimFingerprint } from "../gen/aimmod/hub/v1/hub_pb";
import { GetAimFingerprintRequest } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "./SectionHeader";
import { PageSection } from "./ui/PageSection";
import { Skeleton } from "./ui/Skeleton";
import { hubClient } from "../lib/api";

const AXIS_DEFS: Record<string, { what: string; how: (tracking: boolean) => string }> = {
  precision: {
    what: "How cleanly and directly you move onto the target.",
    how: () => "Higher scores come from straighter mouse paths and less small shake near the target.",
  },
  speed: {
    what: "How quickly you move when a run is live.",
    how: (t) =>
      t
        ? "This reflects your typical movement speed across recent tracking runs."
        : "This reflects your typical movement speed across recent clicking runs.",
  },
  control: {
    what: "How well you stop cleanly and stay on line without wasting motion.",
    how: () => "Higher scores come from fewer overshoots, fewer extra corrections, and cleaner paths into the target.",
  },
  consistency: {
    what: "How steady your movement pace stays from moment to moment.",
    how: () => "Higher scores mean your movement speed stays more even instead of speeding up and braking all the time.",
  },
  decisiveness: {
    what: "How quickly you commit once you are on target.",
    how: () => "Higher scores mean you spend less time in small follow-up corrections before finishing the shot or track.",
  },
  rhythm: {
    what: "How stable your timing feels during a run.",
    how: (t) =>
      t
        ? "For tracking, this becomes Flow and rewards smooth, even speed while staying connected to the target."
        : "For clicking, this rewards even shot timing instead of rushed bursts and hesitant pauses.",
  },
};

type Props = { handle: string };

export function AimFingerprintSection({ handle }: Props) {
  const [fp, setFp] = useState<AimFingerprint | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void hubClient
      .getAimFingerprint(new GetAimFingerprintRequest({ handle }))
      .then((r) => {
        if (!cancelled) {
          setFp(r.overall ?? null);
          setLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [handle]);

  if (loading) {
    return (
      <PageSection>
        <Skeleton className="mb-3 h-3 w-28" />
        <Skeleton className="mb-5 h-6 w-44" />
        <Skeleton className="h-[300px]" />
      </PageSection>
    );
  }

  if (!fp) return null;

  const isTracking =
    fp.dominantScenarioType === "PureTracking" ||
    fp.dominantScenarioType.includes("Tracking");

  const radarData = fp.axes.map((a) => ({ metric: a.label, value: a.value }));

  const stableAxes = [...fp.axes].sort((a, b) => a.volatility - b.volatility).slice(0, 2);
  const swingAxes = [...fp.axes].sort((a, b) => b.volatility - a.volatility).slice(0, 2);

  const styleColor = fp.styleColor || "#00f5a0";

  return (
    <PageSection>
      <SectionHeader
        eyebrow="Aim fingerprint"
        title="Movement profile"
        body={`Built from ${fp.sessionCount} recent sessions with smoothness data — precision, speed, control, consistency, decisiveness, and ${isTracking ? "flow" : "rhythm"}.`}
      />

      <div className="grid gap-4 grid-cols-[1fr_auto] max-[900px]:grid-cols-1">
        {/* radar */}
        <div className="flex flex-col gap-4">
          <ResponsiveContainer width="100%" height={280}>
            <RadarChart data={radarData} cx="50%" cy="50%">
              <PolarGrid stroke="rgba(255,255,255,0.08)" />
              <PolarAngleAxis
                dataKey="metric"
                tick={{ fill: "rgba(255,255,255,0.55)", fontSize: 12 }}
              />
              <PolarRadiusAxis
                angle={90}
                domain={[0, 100]}
                tick={false}
                axisLine={false}
              />
              <Radar
                dataKey="value"
                stroke={styleColor}
                fill={styleColor}
                fillOpacity={0.18}
                strokeWidth={2}
              />
            </RadarChart>
          </ResponsiveContainer>

          {/* stable / swingy badges */}
          <div className="flex flex-wrap gap-2">
            {stableAxes.map((a) => (
              <span
                key={`stable-${a.key}`}
                className="rounded-full border border-mint/20 bg-mint/8 px-3 py-1 text-[11px] text-mint/80"
              >
                Stable: {a.label}
              </span>
            ))}
            {swingAxes.map((a) => (
              <span
                key={`swingy-${a.key}`}
                className="rounded-full border border-gold/20 bg-gold/8 px-3 py-1 text-[11px] text-gold/80"
              >
                Swingy: {a.label}
              </span>
            ))}
          </div>
        </div>

        {/* sidebar: style card + axis values */}
        <div className="flex flex-col gap-3 min-w-[220px] max-w-[280px] max-[900px]:max-w-full">
          {/* aim style */}
          <div
            className="rounded-[14px] border p-4"
            style={{
              borderColor: `${styleColor}30`,
              backgroundColor: `${styleColor}08`,
            }}
          >
            <p className="text-[10px] uppercase tracking-[0.1em] opacity-50">Aim style</p>
            <p className="mt-1.5 text-lg font-semibold" style={{ color: styleColor }}>
              {fp.styleName}
            </p>
            <p className="mt-0.5 text-[11px] text-muted">{fp.styleTagline}</p>
            <p className="mt-3 text-[11px] text-muted-2 leading-relaxed">{fp.styleDescription}</p>
            {fp.styleFocus && (
              <div className="mt-3 rounded-[8px] border border-white/6 bg-white/3 px-3 py-2">
                <p className="text-[10px] uppercase tracking-[0.08em] text-muted-2">Focus areas</p>
                <p className="mt-1 text-[11px] text-muted">{fp.styleFocus}</p>
              </div>
            )}
          </div>

          {/* axis bars */}
          <div className="grid gap-2">
            {fp.axes.map((axis) => {
              const def = AXIS_DEFS[axis.key];
              return (
                <div
                  key={axis.key}
                  className="group rounded-[12px] border border-line bg-white/2 px-3 py-2.5"
                  title={def ? `${def.what} ${def.how(isTracking)}` : ""}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[11px] text-muted">{axis.label}</span>
                    <span className="text-[13px] font-medium tabular-nums text-text">
                      {axis.value}
                    </span>
                  </div>
                  <div className="relative mt-1.5 h-1 w-full rounded-full bg-white/8">
                    <div
                      className="absolute inset-y-0 left-0 rounded-full transition-all"
                      style={{
                        width: `${axis.value}%`,
                        backgroundColor: styleColor,
                        opacity: 0.65,
                      }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      {/* axis definitions */}
      <div className="mt-4 grid gap-2 grid-cols-[repeat(auto-fit,minmax(200px,1fr))]">
        {fp.axes.map((axis) => {
          const def = AXIS_DEFS[axis.key];
          if (!def) return null;
          return (
            <div key={axis.key} className="rounded-[12px] border border-line bg-white/2 px-3 py-2.5">
              <p className="text-[11px] font-medium text-text">{axis.label}</p>
              <p className="mt-0.5 text-[10px] text-muted-2 leading-relaxed">{def.what}</p>
              <p className="mt-1 text-[10px] text-muted-2/70 leading-relaxed">{def.how(isTracking)}</p>
            </div>
          );
        })}
      </div>
    </PageSection>
  );
}
