import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { GetRunResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { TimelineChart } from "../components/charts/TimelineChart";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import type { SessionSummaryValue } from "../gen/aimmod/hub/v1/hub_pb";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { fetchRun, formatDurationMs, formatRelativeTime, slugifyScenarioName, summaryValueToNumber } from "../lib/api";

// ── helpers ─────────────────────────────────────────────────────────────────

function num(map: Record<string, SessionSummaryValue>, key: string): number | null {
  return summaryValueToNumber(map[key]);
}

function fmt(v: number | null, decimals = 0): string {
  if (v === null) return "—";
  return decimals > 0 ? v.toFixed(decimals) : Math.round(v).toLocaleString();
}

// ── small metric row ─────────────────────────────────────────────────────────

function MetricRow({ label, value, dim }: { label: string; value: string; dim?: string }) {
  return (
    <div className="flex items-center justify-between gap-4 py-2 border-b border-white/5 last:border-b-0">
      <span className="text-sm text-muted">{label}</span>
      <span className="text-sm text-text tabular-nums">
        {value}
        {dim && <span className="ml-1 text-muted-2 text-[11px]">{dim}</span>}
      </span>
    </div>
  );
}

// ── section label ────────────────────────────────────────────────────────────

function MetricGroup({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-1 text-[10px] uppercase tracking-[0.1em] text-muted-2">{label}</p>
      {children}
    </div>
  );
}

// ── smoothness badge ─────────────────────────────────────────────────────────

function smoothnessLabel(score: number) {
  if (score >= 80) return { label: "Smooth", color: "text-mint" };
  if (score >= 60) return { label: "Good", color: "text-cyan" };
  if (score >= 40) return { label: "Rough", color: "text-gold" };
  return { label: "Choppy", color: "text-danger" };
}

// ── coaching tag severity ────────────────────────────────────────────────────

function tagColor(tag: string) {
  const lower = tag.toLowerCase();
  if (lower.includes("good") || lower.includes("peak") || lower.includes("great")) return "border-mint/30 text-mint";
  if (lower.includes("warn") || lower.includes("drop") || lower.includes("low") || lower.includes("fade"))
    return "border-danger/30 text-danger";
  return "border-cyan/20 text-cyan";
}

// ── main component ───────────────────────────────────────────────────────────

export function RunPage() {
  const { runId = "" } = useParams();
  const [run, setRun] = useState<GetRunResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setRun(null);
    setError(null);
    void fetchRun(runId)
      .then((next) => { if (!cancelled) setRun(next); })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this run."); });
    return () => { cancelled = true; };
  }, [runId]);

  if (error) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Run" title="Could not load this run" />
          <EmptyState title="Run not found" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!run) {
    return (
      <PageStack>
        <PageSection>
          <div className="mb-3 h-3 w-12 animate-pulse rounded bg-white/5" />
          <div className="mb-3 h-10 w-72 animate-pulse rounded-lg bg-white/5" />
          <div className="mb-4 h-4 w-64 animate-pulse rounded bg-white/5" />
          <Grid className="grid-cols-[repeat(auto-fit,minmax(200px,1fr))]">
            {[0, 1, 2, 3].map((i) => (
              <div key={i} className="h-[100px] animate-pulse rounded-[18px] bg-white/5" />
            ))}
          </Grid>
        </PageSection>
      </PageStack>
    );
  }

  // ── summary metrics ────────────────────────────────────────────────────────
  const s = run.summary;
  const fs = run.featureSet;

  const scoreDerived     = num(s, "scoreTotalDerived");
  const spm              = num(s, "scorePerMinute");
  const peakSpm          = num(s, "peakScorePerMinute");
  const kills            = num(s, "kills");
  const kps              = num(s, "killsPerSecond");
  const peakKps          = num(s, "peakKillsPerSecond");
  const shotsFired       = num(s, "shotsFired");
  const shotsHit         = num(s, "shotsHit");
  const damageDone       = num(s, "damageDone");
  const damagePossible   = num(s, "damagePossible");
  const damageEff        = num(s, "damageEfficiency");
  const avgFireToHit     = num(s, "avgFireToHitMs");
  const p90FireToHit     = num(s, "p90FireToHitMs");
  const avgShotsToHit    = num(s, "avgShotsToHit");
  const correctiveRatio  = num(s, "correctiveShotRatio");
  const avgTtk           = num(s, "panelAvgTtkMs");
  const bestTtk          = num(s, "panelBestTtkMs");

  // smoothness (feature_set)
  const smoothness     = num(fs, "smoothnessComposite");
  const jitter         = num(fs, "smoothnessJitter");
  const overshoot      = num(fs, "smoothnessOvershootRate");
  const pathEff        = num(fs, "smoothnessPathEfficiency");
  const corrRatio      = num(fs, "smoothnessCorrectionRatio");

  const hasTimeline       = run.timelineSeconds.length > 0;
  const hasContextWindows = run.contextWindows.length > 0;
  const hasKills          = kills !== null && kills > 0;
  const hasDamage         = damageEff !== null && damageEff > 0;
  const hasShots          = shotsFired !== null && shotsFired > 0;
  const hasTiming         = avgFireToHit !== null;
  const hasTtk            = avgTtk !== null;
  const hasSmoothness     = smoothness !== null;

  // stat card display values
  const spmDisplay     = spm ? `${Math.round(spm).toLocaleString()} /min` : "—";
  const spmDetail      = peakSpm ? `Peak ${Math.round(peakSpm).toLocaleString()} /min` : "Score per minute";
  const shotsPerHit    = avgShotsToHit ? avgShotsToHit.toFixed(2) : null;
  const accDetail      = shotsPerHit ? `${shotsPerHit} shots/hit` : "Accuracy";
  const damageDisplay  = damageEff !== null ? `${damageEff.toFixed(1)}%` : "—";
  const damageDetail   = avgFireToHit !== null ? `Fire→hit ${Math.round(avgFireToHit)}ms avg` : "Damage efficiency";

  return (
    <PageStack>
      {/* ── header + stat cards ── */}
      <PageSection>
        <SectionHeader
          eyebrow="Run"
          title={<Link className="hover:text-cyan transition-colors" to={`/scenarios/${slugifyScenarioName(run.scenarioName)}`}>{run.scenarioName}</Link>}
          body={`Played ${formatRelativeTime(run.playedAtIso)} by ${run.userDisplayName || run.userHandle}.`}
          aside={
            <div className="flex items-center gap-3">
              <ScenarioTypeBadge type={run.scenarioType} />
              {run.userHandle && (
                <Link className="text-cyan underline underline-offset-3" to={`/profiles/${run.userHandle}`}>
                  Open profile
                </Link>
              )}
            </div>
          }
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(200px,1fr))]">
          <StatCard
            label="Score"
            value={run.score.toLocaleString()}
            detail={scoreDerived ? `Derived ${Math.round(scoreDerived).toLocaleString()}` : "Final result"}
          />
          <StatCard
            label="Accuracy"
            value={`${run.accuracy.toFixed(1)}%`}
            detail={accDetail}
            accent="cyan"
          />
          <StatCard
            label="SPM"
            value={spmDisplay}
            detail={spmDetail}
            accent="gold"
          />
          <StatCard
            label="Damage eff"
            value={damageDisplay}
            detail={damageDetail}
            accent="violet"
          />
        </Grid>
      </PageSection>

      {/* ── timeline chart ── */}
      {hasTimeline && (
        <PageSection>
          <SectionHeader
            eyebrow="Run shape"
            title="Score rate & accuracy over time"
            body={
              hasContextWindows
                ? "SPM (left) and accuracy % (right) per second. Violet markers show saved moments."
                : "SPM (left) and accuracy % (right) plotted second by second."
            }
          />
          <TimelineChart timeline={run.timelineSeconds} contextWindows={run.contextWindows} />
        </PageSection>
      )}

      {/* ── context windows + detailed metrics ── */}
      <Grid className="grid-cols-2 items-start max-[1100px]:grid-cols-1">

        {/* ── context windows ── */}
        <PageSection>
          <SectionHeader
            eyebrow="Saved moments"
            title="Context windows"
            body="Key moments captured during the run."
          />
          {hasContextWindows ? (
            <ScrollArea className="max-h-[min(72vh,900px)] pr-2">
              <div className="grid gap-3">
                {run.contextWindows.map((window, index) => {
                  const startSec = Math.round(Number(window.startMs) / 1000);
                  const endSec   = Math.round(Number(window.endMs) / 1000);

                  // pull structured fields from featureSummary
                  const cwFired    = num(window.featureSummary, "firedCount");
                  const cwHit      = num(window.featureSummary, "hitCount");
                  const cwAcc      = num(window.featureSummary, "accuracyPct");
                  const cwSpm      = num(window.featureSummary, "avgScorePerMinute");
                  const cwKps      = num(window.featureSummary, "avgKillsPerSecond");
                  const cwDmgEff   = num(window.featureSummary, "avgDamageEfficiency");
                  const cwYaw      = num(window.featureSummary, "avgNearestYawErrorDeg");
                  const cwPitch    = num(window.featureSummary, "avgNearestPitchErrorDeg");
                  const cwDist     = num(window.featureSummary, "avgNearestDistance");

                  return (
                    <div
                      key={`${window.startMs}-${index}`}
                      className="rounded-[18px] border border-line bg-white/2 p-[18px]"
                    >
                      {/* header row */}
                      <div className="flex items-start justify-between gap-3 mb-3">
                        <div>
                          <strong className="block text-text">
                            {window.label || window.windowType || "Saved moment"}
                          </strong>
                          <p className="mt-0.5 text-[11px] text-muted-2">
                            {startSec}s – {endSec}s
                            {window.windowType && window.label && window.windowType !== window.label
                              ? ` · ${window.windowType}`
                              : ""}
                          </p>
                        </div>
                        {window.coachingTags.length > 0 && (
                          <div className="flex flex-wrap gap-1.5 justify-end shrink-0">
                            {window.coachingTags.map((tag) => (
                              <span
                                key={tag}
                                className={`rounded-full border px-2.5 py-0.5 text-[10px] uppercase tracking-wider ${tagColor(tag)}`}
                              >
                                {tag}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>

                      {/* structured metrics grid */}
                      <div className="grid grid-cols-3 gap-x-4 gap-y-2 text-[12px]">
                        {cwFired !== null && (
                          <div>
                            <p className="text-muted-2">Fired / Hit</p>
                            <p className="text-text">
                              {Math.round(cwFired)} / {cwHit !== null ? Math.round(cwHit) : "—"}
                            </p>
                          </div>
                        )}
                        {cwAcc !== null && (
                          <div>
                            <p className="text-muted-2">Accuracy</p>
                            <p className="text-text">{cwAcc.toFixed(1)}%</p>
                          </div>
                        )}
                        {cwSpm !== null && (
                          <div>
                            <p className="text-muted-2">SPM</p>
                            <p className="text-text">{Math.round(cwSpm).toLocaleString()}</p>
                          </div>
                        )}
                        {cwKps !== null && (
                          <div>
                            <p className="text-muted-2">KPS</p>
                            <p className="text-text">{cwKps.toFixed(2)}</p>
                          </div>
                        )}
                        {cwDmgEff !== null && (
                          <div>
                            <p className="text-muted-2">Dmg eff</p>
                            <p className="text-text">{cwDmgEff.toFixed(1)}%</p>
                          </div>
                        )}
                        {cwDist !== null && (
                          <div>
                            <p className="text-muted-2">Distance</p>
                            <p className="text-text">{cwDist.toFixed(1)}</p>
                          </div>
                        )}
                        {cwYaw !== null && (
                          <div>
                            <p className="text-muted-2">Yaw err</p>
                            <p className="text-text">{cwYaw.toFixed(1)}°</p>
                          </div>
                        )}
                        {cwPitch !== null && (
                          <div>
                            <p className="text-muted-2">Pitch err</p>
                            <p className="text-text">{cwPitch.toFixed(1)}°</p>
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState title="No saved moments" body="This run did not include any context windows." />
          )}
        </PageSection>

        {/* ── right column: metrics + smoothness ── */}
        <div className="grid gap-[18px]">

          {/* detailed run metrics */}
          <PageSection>
            <SectionHeader eyebrow="Run metrics" title="Session breakdown" />

            <div className="grid gap-5">
              {/* pace */}
              <MetricGroup label="Pace">
                <MetricRow label="Score per minute" value={fmt(spm)} dim="/min" />
                <MetricRow label="Peak SPM" value={fmt(peakSpm)} dim="/min" />
              </MetricGroup>

              {/* shots */}
              {hasShots && (
                <MetricGroup label="Shots">
                  <MetricRow label="Fired" value={fmt(shotsFired)} />
                  <MetricRow label="Hit" value={fmt(shotsHit)} />
                  {avgShotsToHit !== null && (
                    <MetricRow label="Shots per hit" value={avgShotsToHit.toFixed(2)} />
                  )}
                  {correctiveRatio !== null && (
                    <MetricRow label="Corrective shots" value={`${(correctiveRatio * 100).toFixed(1)}%`} />
                  )}
                </MetricGroup>
              )}

              {/* kills */}
              {hasKills && (
                <MetricGroup label="Kills">
                  <MetricRow label="Total kills" value={fmt(kills)} />
                  <MetricRow label="KPS" value={kps !== null ? kps.toFixed(2) : "—"} dim="/s" />
                  {peakKps !== null && <MetricRow label="Peak KPS" value={peakKps.toFixed(2)} dim="/s" />}
                </MetricGroup>
              )}

              {/* damage */}
              {hasDamage && (
                <MetricGroup label="Damage">
                  <MetricRow label="Done" value={fmt(damageDone)} />
                  <MetricRow label="Possible" value={fmt(damagePossible)} />
                  <MetricRow label="Efficiency" value={damageEff !== null ? `${damageEff.toFixed(1)}%` : "—"} />
                </MetricGroup>
              )}

              {/* timing */}
              {hasTiming && (
                <MetricGroup label="Fire → Hit latency">
                  <MetricRow label="Average" value={fmt(avgFireToHit)} dim="ms" />
                  <MetricRow label="p90" value={fmt(p90FireToHit)} dim="ms" />
                </MetricGroup>
              )}

              {/* TTK */}
              {hasTtk && (
                <MetricGroup label="Time to kill">
                  <MetricRow label="Average" value={fmt(avgTtk)} dim="ms" />
                  {bestTtk !== null && <MetricRow label="Best" value={fmt(bestTtk)} dim="ms" />}
                </MetricGroup>
              )}
            </div>
          </PageSection>

          {/* smoothness panel */}
          {hasSmoothness && (
            <PageSection>
              <SectionHeader eyebrow="Mouse quality" title="Smoothness" />

              {(() => {
                const { label, color } = smoothnessLabel(smoothness!);
                return (
                  <div className="mb-5 flex items-center gap-4">
                    <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl border border-line bg-white/2">
                      <span className={`text-2xl font-medium ${color}`}>{Math.round(smoothness!)}</span>
                    </div>
                    <div>
                      <p className={`text-lg font-medium ${color}`}>{label}</p>
                      <p className="text-sm text-muted">Composite smoothness score (0–100)</p>
                    </div>
                  </div>
                );
              })()}

              <div className="grid gap-0">
                {jitter !== null && (
                  <MetricRow label="Jitter" value={`${(jitter * 100).toFixed(1)}%`} />
                )}
                {overshoot !== null && (
                  <MetricRow label="Overshoot rate" value={`${(overshoot * 100).toFixed(1)}%`} />
                )}
                {pathEff !== null && (
                  <MetricRow label="Path efficiency" value={`${(pathEff * 100).toFixed(1)}%`} />
                )}
                {corrRatio !== null && (
                  <MetricRow label="Correction ratio" value={`${(corrRatio * 100).toFixed(1)}%`} />
                )}
              </div>
            </PageSection>
          )}
        </div>
      </Grid>
    </PageStack>
  );
}
