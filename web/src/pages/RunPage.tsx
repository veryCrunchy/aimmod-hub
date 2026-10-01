import { useEffect, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useParams } from "react-router-dom";
import type { GetRunResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageHeader } from "../components/ui/PageHeader";
import { Section } from "../components/ui/Section";
import { Button } from "../components/ui/Button";
import { RunTable } from "../components/RunTable";
import { bestPerPlayer, formatAccuracy, formatScore, rankWithin } from "../lib/kovaaksStats";
import { TimelineChart } from "../components/charts/TimelineChart";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { RunReplayPanel } from "../components/RunReplayPanel";
import { ScenarioBenchmarkRankList } from "../components/BenchmarkCards";
import type { SessionSummaryValue } from "../gen/aimmod/hub/v1/hub_pb";
import { deleteReplayMedia, displayScenarioType, fetchMousePath, fetchReplayMediaMeta, fetchRun, formatDurationMs, formatRelativeTime, slugifyScenarioName, summaryValueToNumber } from "../lib/api";
import { useAuth } from "../lib/AuthContext";

// ── helpers ─────────────────────────────────────────────────────────────────

function num(map: Record<string, SessionSummaryValue>, key: string): number | null {
  return summaryValueToNumber(map[key]);
}

function fmt(v: number | null, decimals = 0): string {
  if (v === null) return "—";
  return decimals > 0 ? v.toFixed(decimals) : Math.round(v).toLocaleString();
}

function normalizePercentLike(value: number | null): number | null {
  if (value == null || !Number.isFinite(value)) return null;
  if (value >= 0 && value <= 1) return value * 100;
  return value;
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
  const auth = useAuth();
  const { runId = "" } = useParams();
  const [run, setRun] = useState<GetRunResponse | null>(null);
  const [replayMediaUrl, setReplayMediaUrl] = useState<string | null>(null);
  const [mousePath, setMousePath] = useState<import("../lib/api").MousePathPoint[]>([]);
  const [hitTimestampsMs, setHitTimestampsMs] = useState<number[]>([]);
  const [playbackOffsetMs, setPlaybackOffsetMs] = useState(0);
  const [videoOffsetMs, setVideoOffsetMs] = useState(0);
  const [mousePathLoaded, setMousePathLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deletingReplay, setDeletingReplay] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setRun(null);
    setReplayMediaUrl(null);
    setMousePath([]);
    setHitTimestampsMs([]);
    setPlaybackOffsetMs(0);
    setVideoOffsetMs(0);
    setMousePathLoaded(false);
    setError(null);
    void fetchRun(runId)
      .then((next) => {
        if (!cancelled) {
          setRun(next);
          void fetchReplayMediaMeta(next.runId || runId)
            .then((media) => {
              if (!cancelled && media.available && media.mediaUrl) {
                setReplayMediaUrl(media.mediaUrl);
              }
            })
            .catch(() => {});
          void fetchMousePath(next.runId || runId)
            .then((payload) => {
              if (!cancelled && payload.available) {
                setMousePath(payload.points);
                setHitTimestampsMs(payload.hitTimestampsMs);
                setPlaybackOffsetMs(payload.playbackOffsetMs ?? 0);
                setVideoOffsetMs(payload.videoOffsetMs ?? 0);
              }
              if (!cancelled) {
                setMousePathLoaded(true);
              }
            })
            .catch(() => {
              if (!cancelled) {
                setMousePathLoaded(true);
              }
            });
        }
      })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this run."); });
    return () => { cancelled = true; };
  }, [runId]);

  const runName = run?.userDisplayName || run?.userHandle || "";
  const metaTitle = run
    ? `${run.scenarioName} by ${runName} · AimMod Hub`
    : "Run · AimMod Hub";
  const metaDesc = run
    ? `Score: ${Math.round(run.score).toLocaleString()} · Accuracy: ${run.accuracy.toFixed(1)}%`
    : "Run detail on AimMod Hub.";

  if (error) {
    return (
      <PageStack>
        <Helmet><title>Run · AimMod Hub</title></Helmet>
        <PageHeader title="Run not found" />
        <EmptyState title="This run could not be loaded. It may have been removed.">
          <Button to="/kovaaks">Recent runs</Button>
        </EmptyState>
      </PageStack>
    );
  }

  if (!run) {
    return <PageStack><PageSkeleton label="Loading run" /></PageStack>;
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
  const damageEff        = normalizePercentLike(num(s, "damageEfficiency"));
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
  const canDeleteReplayMedia =
    Boolean(replayMediaUrl) &&
    auth.authenticated &&
    !!auth.user &&
    (auth.user.profileHandle || auth.user.username).toLowerCase() === run.userHandle.toLowerCase();

  const damageDisplay  = damageEff !== null ? `${damageEff.toFixed(1)}%` : "—";
  const scenarioSlug   = slugifyScenarioName(run.scenarioName);
  const scenarioBoard  = [...run.scenarioRuns].sort((a, b) => b.score - a.score);
  const scoreRank      = rankWithin(run.score, scenarioBoard);
  const scoreRankLabel = scoreRank != null ? `#${scoreRank} of the top ${scenarioBoard.length} runs` : scenarioBoard.length ? `Outside the top ${scenarioBoard.length}` : undefined;
  const playerBoard    = bestPerPlayer(scenarioBoard);

  return (
    <PageStack>
      <Helmet>
        <title>{metaTitle}</title>
        <meta name="description" content={metaDesc} />
        <meta property="og:title" content={metaTitle} />
        <meta property="og:description" content={metaDesc} />
      </Helmet>
      <PageHeader
        title={<Link className="hover:text-cyan" to={`/scenarios/${scenarioSlug}`}>{run.scenarioName}</Link>}
        meta={<>
          <Link className="text-text hover:text-cyan" to={`/profiles/${run.userHandle}`}>{runName}</Link>
          {" · "}<span title={new Date(run.playedAtIso).toLocaleString()}>{formatRelativeTime(run.playedAtIso)}</span>
          {" · "}{formatDurationMs(run.durationMs)}
          {displayScenarioType(run.scenarioType) ? ` · ${displayScenarioType(run.scenarioType)}` : ""}
        </>}
        actions={run.userHandle ? <>
          <Button to={`/profiles/${run.userHandle}/scenarios/${scenarioSlug}`}>Player's progress</Button>
          <Button to={`/scenarios/${scenarioSlug}?tab=leaderboard`}>Scenario leaderboard</Button>
        </> : null}
      />
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard label="Score" value={formatScore(run.score)} detail={scoreRankLabel} accent={scoreRank === 1 ? "gold" : "text"} />
        <StatCard label="Accuracy" value={formatAccuracy(run.accuracy)} detail={hasShots ? `${fmt(shotsHit)} of ${fmt(shotsFired)} shots hit` : undefined} />
        <StatCard label="Score per minute" value={fmt(spm)} detail={peakSpm ? `Peak ${fmt(peakSpm)}` : undefined} />
        {hasDamage
          ? <StatCard label="Damage efficiency" value={damageDisplay} detail={damageDone != null && damagePossible != null ? `${fmt(damageDone)} of ${fmt(damagePossible)}` : undefined} />
          : hasTtk ? <StatCard label="Time to kill" value={`${fmt(avgTtk)} ms`} detail={bestTtk != null ? `Best ${fmt(bestTtk)} ms` : "Average"} />
          : hasKills ? <StatCard label="Kills" value={fmt(kills)} detail={kps != null ? `${kps.toFixed(2)} per second` : undefined} />
          : null}
      </div>

      {run.benchmarkRanks.length > 0 && (
        <PageSection>
          <SectionHeader title="Benchmark rank for this score" />
          <ScenarioBenchmarkRankList title="Ranks" ranks={run.benchmarkRanks} handle={run.userHandle} />
        </PageSection>
      )}

      {/* ── timeline chart ── */}
      {hasTimeline && (
        <PageSection>
          <SectionHeader
            title="Score per minute and accuracy, second by second"
            body={hasContextWindows ? "Markers show the moments listed below." : undefined}
          />
          <TimelineChart timeline={run.timelineSeconds} contextWindows={run.contextWindows} />
        </PageSection>
      )}

      {(replayMediaUrl || mousePath.length > 1) && (
        <RunReplayPanel
          run={run}
          runId={runId}
          replayMediaUrl={replayMediaUrl}
          mousePath={mousePath}
          hitTimestampsMs={hitTimestampsMs}
          playbackOffsetMs={playbackOffsetMs}
          videoOffsetMs={videoOffsetMs}
          mousePathLoaded={mousePathLoaded}
          canDeleteReplayMedia={canDeleteReplayMedia}
          deletingReplay={deletingReplay}
          onDeleteReplay={() => {
            setDeletingReplay(true);
            void deleteReplayMedia(run.runId || runId)
              .then(() => setReplayMediaUrl(null))
              .finally(() => setDeletingReplay(false));
          }}
        />
      )}

      {/* ── context windows + detailed metrics ── */}
      <Grid className={hasContextWindows ? "grid-cols-2 items-start max-[1100px]:grid-cols-1" : "items-start"}>

        {hasContextWindows && <PageSection>
          <SectionHeader title="Moments" body="Stretches of the run worth a closer look." />
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
                  const cwDmgEff   = normalizePercentLike(num(window.featureSummary, "avgDamageEfficiency"));
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
            null
          )}
        </PageSection>}

        {/* ── right column: metrics + smoothness ── */}
        <div className={hasContextWindows ? "grid gap-[18px]" : "grid items-start gap-[18px] lg:grid-cols-2"}>

          {/* detailed run metrics */}
          <PageSection>
            <SectionHeader title="Details" />

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
              <SectionHeader title="Smoothness" />

              {(() => {
                const { label, color } = smoothnessLabel(smoothness!);
                return (
                  <div className="mb-5 flex items-center gap-4">
                    <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl border border-line bg-white/2">
                      <span className={`text-2xl font-medium ${color}`}>{Math.round(smoothness!)}</span>
                    </div>
                    <div>
                      <p className={`text-lg font-medium ${color}`}>{label}</p>
                      <p className="text-sm text-muted">Out of 100</p>
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

      {playerBoard.length > 0 && (
        <Section title="Best players on this scenario" aside={<Link to={`/scenarios/${scenarioSlug}?tab=leaderboard`} className="text-cyan hover:underline">Full leaderboard</Link>}>
          <RunTable runs={playerBoard.map((entry) => entry.best)} scenario={false} ranked caption="Best player scores on this scenario"
            playerHref={(r) => `/profiles/${r.userHandle || r.userDisplayName}/scenarios/${scenarioSlug}`} />
        </Section>
      )}
    </PageStack>
  );
}
