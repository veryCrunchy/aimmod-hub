import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { GetScenarioPageResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { ScoreDistributionChart } from "../components/charts/ScoreDistributionChart";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchScenarioPage, formatDurationMs } from "../lib/api";

export function ScenarioPage() {
  const { slug = "" } = useParams();
  const [page, setPage] = useState<GetScenarioPageResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setPage(null);
    setError(null);
    void fetchScenarioPage(slug)
      .then((next) => {
        if (!cancelled) setPage(next);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this scenario.");
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  if (error) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Scenario" title="Could not load this scenario" />
          <EmptyState title="Scenario not found" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!page) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Scenario" title="Loading scenario" />
        </PageSection>
      </PageStack>
    );
  }

  const hasDistribution = page.recentRuns.length >= 3;
  const scoreRange =
    page.bestScore > 0
      ? `${Math.round(page.averageScore).toLocaleString()} avg · ${Math.round(page.bestScore).toLocaleString()} best`
      : null;

  return (
    <PageStack>
      <PageSection>
        <SectionHeader
          eyebrow="Scenario"
          title={page.scenarioName}
          body="A shared view of how players are performing on this scenario."
          aside={<ScenarioTypeBadge type={page.scenarioType} />}
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
          <StatCard label="Runs" value={page.runCount.toLocaleString()} detail="Saved runs for this scenario" />
          <StatCard
            label="Best score"
            value={Math.round(page.bestScore).toLocaleString()}
            detail="Best recorded result so far"
            accent="cyan"
          />
          <StatCard
            label="Average score"
            value={Math.round(page.averageScore).toLocaleString()}
            detail="Average result across all runs"
            accent="gold"
          />
          <StatCard
            label="Average accuracy"
            value={`${page.averageAccuracy.toFixed(1)}%`}
            detail={`Typical duration ${formatDurationMs(page.averageDurationMs)}`}
            accent="violet"
          />
        </Grid>
      </PageSection>

      {hasDistribution && (
        <PageSection>
          <SectionHeader
            eyebrow="Score distribution"
            title="How scores are spread"
            body={
              scoreRange
                ? `${scoreRange}. Highlighted bar shows where the average falls.`
                : "Score spread across all recorded runs. Highlighted bar shows where the average falls."
            }
          />
          <ScoreDistributionChart runs={page.recentRuns} />
        </PageSection>
      )}

      <Grid className="grid-cols-2 max-[1100px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Recent runs"
            title="Latest runs"
            body="The newest runs for this scenario."
          />
          {page.recentRuns.length > 0 ? (
            <ScrollArea className="max-h-[min(64vh,820px)] overflow-auto rounded-[18px] border border-line bg-white/2">
              <table className="min-w-full text-left text-sm">
                <thead className="border-b border-line text-[11px] uppercase tracking-[0.08em] text-muted">
                  <tr>
                    <th className="px-4 py-3">Player</th>
                    <th className="px-4 py-3">Score</th>
                    <th className="px-4 py-3">Acc</th>
                    <th className="px-4 py-3">When</th>
                    <th className="px-4 py-3">Run</th>
                  </tr>
                </thead>
                <tbody>
                  {page.recentRuns.map((run) => {
                    const isBest = Math.round(run.score) === Math.round(page.bestScore);
                    return (
                      <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0">
                        <td className="px-4 py-3 text-text">
                          <Link
                            className="text-cyan underline underline-offset-3"
                            to={`/profiles/${run.userHandle || run.userDisplayName}`}
                          >
                            {run.userDisplayName || run.userHandle}
                          </Link>
                        </td>
                        <td className="px-4 py-3">
                          <span className={isBest ? "text-mint font-medium" : "text-text"}>
                            {Math.round(run.score).toLocaleString()}
                          </span>
                          {isBest && (
                            <span className="ml-2 text-[10px] text-mint/70 uppercase tracking-wider">best</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-text">{run.accuracy.toFixed(1)}%</td>
                        <td className="px-4 py-3 text-text">{new Date(run.playedAtIso).toLocaleDateString()}</td>
                        <td className="px-4 py-3 text-text">
                          <Link
                            className="text-cyan underline underline-offset-3"
                            to={`/runs/${run.runId || run.sessionId}`}
                          >
                            Open
                          </Link>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </ScrollArea>
          ) : (
            <EmptyState title="No runs yet" body="This scenario has not received any uploaded runs yet." />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Quick read"
            title="What this page helps you see"
            body="Use this page to get a fast read on volume, score range, consistency, and who is actively playing the scenario."
          />
          <ul className="grid gap-2.5 pl-[18px] text-sm leading-7 text-muted">
            <li>How much history exists for this scenario</li>
            <li>Whether score ceiling and consistency are diverging</li>
            <li>Who is actively playing it right now</li>
            <li>Which runs are worth opening next</li>
          </ul>
          {page.bestScore > 0 && page.averageScore > 0 && (
            <div className="mt-6 rounded-[14px] border border-line bg-white/2 p-4 text-sm text-muted">
              <p className="mb-3 text-[11px] uppercase tracking-[0.08em] text-muted-2">Score ceiling gap</p>
              <div className="flex items-end justify-between gap-2">
                <div>
                  <p className="text-text text-lg font-medium">{Math.round(page.bestScore).toLocaleString()}</p>
                  <p className="text-[11px] text-muted-2">best</p>
                </div>
                <div className="flex-1 mx-3 h-px bg-line relative">
                  <div
                    className="absolute inset-y-0 left-0 bg-mint/40 h-px"
                    style={{ width: `${Math.min(100, (page.averageScore / page.bestScore) * 100)}%` }}
                  />
                </div>
                <div className="text-right">
                  <p className="text-gold text-lg font-medium">{Math.round(page.averageScore).toLocaleString()}</p>
                  <p className="text-[11px] text-muted-2">avg</p>
                </div>
              </div>
              <p className="mt-3 text-[11px] text-muted-2">
                Average is {Math.round((page.averageScore / page.bestScore) * 100)}% of the best score — the gap shows ceiling room.
              </p>
            </div>
          )}
        </PageSection>
      </Grid>
    </PageStack>
  );
}
