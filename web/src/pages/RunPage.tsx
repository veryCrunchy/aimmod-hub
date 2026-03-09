import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { GetRunResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchRun, formatDurationMs, summaryValueToNumber, summaryValueToString } from "../lib/api";

export function RunPage() {
  const { runId = "" } = useParams();
  const [run, setRun] = useState<GetRunResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setRun(null);
    setError(null);
    void fetchRun(runId)
      .then((next) => {
        if (!cancelled) setRun(next);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this run.");
      });
    return () => {
      cancelled = true;
    };
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
          <SectionHeader eyebrow="Run" title="Loading run" />
        </PageSection>
      </PageStack>
    );
  }

  const peakSpm = summaryValueToNumber(run.summary.peakScorePerMinute);
  const scoreDerived = summaryValueToNumber(run.summary.scoreTotalDerived);
  const damageEff = summaryValueToNumber(run.summary.damageEfficiency);

  return (
    <PageStack>
      <PageSection>
        <SectionHeader
          eyebrow="Run"
          title={run.scenarioName}
          body={`Played ${new Date(run.playedAtIso).toLocaleString()} by ${run.userDisplayName || run.userHandle}.`}
          aside={
            run.userHandle ? <Link className="text-cyan underline underline-offset-3" to={`/profiles/${run.userHandle}`}>Open profile</Link> : undefined
          }
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
          <StatCard label="Score" value={run.score.toLocaleString()} detail={scoreDerived ? `Derived ${Math.round(scoreDerived).toLocaleString()}` : "Final run result"} />
          <StatCard label="Accuracy" value={`${run.accuracy.toFixed(1)}%`} detail={run.scenarioType || "Unknown"} accent="cyan" />
          <StatCard label="Duration" value={formatDurationMs(run.durationMs)} detail={`${run.timelineSeconds.length} saved timeline points`} accent="gold" />
          <StatCard label="Peak pace" value={peakSpm ? Math.round(peakSpm).toLocaleString() : "—"} detail={damageEff !== null ? `Damage eff ${damageEff.toFixed(1)}%` : "No saved efficiency"} accent="violet" />
        </Grid>
      </PageSection>

      <Grid className="grid-cols-2 items-start max-[1100px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Saved moments"
            title="Context windows"
            body="Key moments pulled out of the run so you can review the parts that mattered most."
          />
          {run.contextWindows.length > 0 ? (
            <div className="grid gap-3">
              {run.contextWindows.map((window, index) => (
                <div key={`${window.startMs}-${index}`} className="rounded-[18px] border border-line bg-white/2 p-[18px]">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <strong className="block text-text">{window.label || window.windowType || "Saved moment"}</strong>
                      <p className="mt-1 text-sm text-muted">
                        {formatDurationMs(window.startMs)} to {formatDurationMs(window.endMs)}
                      </p>
                    </div>
                    {window.coachingTags.length > 0 ? (
                      <span className="rounded-full border border-line px-3 py-1 text-[11px] text-cyan">
                        {window.coachingTags[0]}
                      </span>
                    ) : null}
                  </div>
                  {Object.keys(window.featureSummary).length > 0 ? (
                    <div className="mt-4 grid gap-2 text-sm text-muted">
                      {Object.entries(window.featureSummary).slice(0, 4).map(([key, value]) => (
                        <div key={key} className="flex items-center justify-between gap-4">
                          <span>{key}</span>
                          <span className="text-text">{summaryValueToString(value) ?? "—"}</span>
                        </div>
                      ))}
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title="No saved moments" body="This run did not include any saved context windows yet." />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Run shape"
            title="Timeline by second"
            body="A second-by-second view of how the run changed over time."
          />
          {run.timelineSeconds.length > 0 ? (
            <div className="overflow-hidden rounded-[18px] border border-line bg-white/2">
              <table className="w-full text-left text-sm">
                <thead className="border-b border-line text-[11px] uppercase tracking-[0.08em] text-muted">
                  <tr>
                    <th className="px-4 py-3">Sec</th>
                    <th className="px-4 py-3">Score</th>
                    <th className="px-4 py-3">Acc</th>
                    <th className="px-4 py-3">SPM</th>
                    <th className="px-4 py-3">Shots</th>
                    <th className="px-4 py-3">Paused</th>
                  </tr>
                </thead>
                <tbody>
                  {run.timelineSeconds.slice(-12).map((point) => (
                    <tr key={point.tSec} className="border-b border-white/6 last:border-b-0">
                      <td className="px-4 py-3 text-text">{point.tSec}s</td>
                      <td className="px-4 py-3 text-text">{Math.round(point.score).toLocaleString()}</td>
                      <td className="px-4 py-3 text-text">{point.accuracy.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-text">{Math.round(point.spm).toLocaleString()}</td>
                      <td className="px-4 py-3 text-text">{point.shots}</td>
                      <td className="px-4 py-3 text-text">{point.paused ? "Yes" : "No"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <EmptyState title="No saved timeline" body="This run did not include per-second timeline data yet." />
          )}
        </PageSection>
      </Grid>
    </PageStack>
  );
}
