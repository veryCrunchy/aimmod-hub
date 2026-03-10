import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { GetProfileResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { RunTrendChart } from "../components/charts/RunTrendChart";
import { ScenarioTypeChart } from "../components/charts/ScenarioTypeChart";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Skeleton } from "../components/ui/Skeleton";
import { SortableTh } from "../components/ui/SortableTh";
import { TypeFilterBar } from "../components/ui/TypeFilterBar";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchProfile, formatDurationMs, slugifyScenarioName } from "../lib/api";

type RunSortField = "score" | "accuracy" | "duration";

export function ProfilePage() {
  const { handle = "" } = useParams();
  const [profile, setProfile] = useState<GetProfileResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [scenarioTypeFilter, setScenarioTypeFilter] = useState<string | null>(null);
  const [runSortField, setRunSortField] = useState<RunSortField>("score");
  const [runSortDir, setRunSortDir] = useState<"asc" | "desc">("desc");

  useEffect(() => {
    let cancelled = false;
    setProfile(null);
    setError(null);
    void fetchProfile(handle)
      .then((next) => {
        if (!cancelled) setProfile(next);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this profile.");
      });
    return () => {
      cancelled = true;
    };
  }, [handle]);

  const topScenarios = profile?.topScenarios ?? [];

  const scenarioTypes = useMemo(() => {
    const types = new Set<string>();
    for (const scenario of topScenarios) {
      if (scenario.scenarioType?.trim() && scenario.scenarioType !== "Unknown") {
        types.add(scenario.scenarioType);
      }
    }
    return [...types];
  }, [topScenarios]);

  function handleRunSort(field: string) {
    const f = field as RunSortField;
    if (f === runSortField) {
      setRunSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setRunSortField(f);
      setRunSortDir("desc");
    }
  }

  if (error) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Profile" title="Could not load this profile" />
          <EmptyState title="Profile not found" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!profile) {
    return (
      <PageStack>
        <PageSection className="grid grid-cols-[1.6fr_minmax(280px,0.9fr)] gap-[18px] max-[1100px]:grid-cols-1">
          <div>
            <Skeleton className="mb-3 h-3 w-16" />
            <Skeleton className="mb-3 h-10 w-48" />
            <Skeleton className="h-4 w-96" />
          </div>
          <Skeleton className="h-28 rounded-[18px]" />
        </PageSection>
        <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
          {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-[100px]" />)}
        </Grid>
      </PageStack>
    );
  }

  const primaryFocus =
    profile.primaryScenarioType && profile.primaryScenarioType !== "Unknown"
      ? profile.primaryScenarioType
      : "Mixed practice";
  const accuracyDetail =
    profile.primaryScenarioType && profile.primaryScenarioType !== "Unknown"
      ? profile.primaryScenarioType
      : "Across all scenarios";

  const hasScenarioTypes =
    profile.topScenarios.length > 0 &&
    profile.topScenarios.some((s) => s.scenarioType?.trim() && s.scenarioType !== "Unknown");

  const hasTrend = profile.recentRuns.length >= 2;

  const filteredScenarios = scenarioTypeFilter
    ? profile.topScenarios.filter((s) => s.scenarioType === scenarioTypeFilter)
    : profile.topScenarios;

  const sortedRuns = [...profile.recentRuns].sort((a, b) => {
    let diff = 0;
    if (runSortField === "score") diff = a.score - b.score;
    else if (runSortField === "accuracy") diff = a.accuracy - b.accuracy;
    else diff = Number(a.durationMs) - Number(b.durationMs);
    return runSortDir === "asc" ? diff : -diff;
  });

  return (
    <PageStack>
      <PageSection className="grid grid-cols-[1.6fr_minmax(280px,0.9fr)] gap-[18px] max-[1100px]:grid-cols-1">
        <div>
          <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Profile</div>
          <h1>@{profile.userHandle}</h1>
          <p className="text-sm leading-7 text-muted">
            {profile.userDisplayName || profile.userHandle} has {profile.runCount.toLocaleString()} runs across{" "}
            {profile.scenarioCount.toLocaleString()} scenarios.
          </p>
        </div>
        <div className="grid content-start gap-2 rounded-[18px] border border-cyan/18 bg-[linear-gradient(180deg,rgba(57,208,255,0.08),rgba(182,151,255,0.06))] p-[18px]">
          <span className="text-[12px] uppercase tracking-[0.08em] text-cyan">Main focus</span>
          <strong className="text-[28px]">{primaryFocus}</strong>
          <em className="text-sm not-italic leading-7 text-muted">
            {primaryFocus === "Mixed practice"
              ? "This player spreads their time across multiple scenario styles."
              : "The scenario family this player spends the most time in."}
          </em>
        </div>
      </PageSection>

      <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
        <StatCard label="Runs" value={profile.runCount.toLocaleString()} detail="Saved practice history" />
        <StatCard
          label="Scenario spread"
          value={profile.scenarioCount.toLocaleString()}
          detail="Different scenarios played"
          accent="cyan"
        />
        <StatCard
          label="Average score"
          value={Math.round(profile.averageScore).toLocaleString()}
          detail="Average result across all runs"
          accent="gold"
        />
        <StatCard
          label="Average accuracy"
          value={`${profile.averageAccuracy.toFixed(1)}%`}
          detail={accuracyDetail}
          accent="violet"
        />
      </Grid>

      {(hasTrend || hasScenarioTypes) && (
        <Grid className="grid-cols-2 max-[1100px]:grid-cols-1">
          {hasTrend && (
            <PageSection>
              <SectionHeader
                eyebrow="Accuracy trend"
                title="Recent accuracy"
                body="Accuracy % across the most recent runs, oldest to newest."
              />
              <RunTrendChart runs={profile.recentRuns} />
            </PageSection>
          )}

          {hasScenarioTypes && (
            <PageSection>
              <SectionHeader
                eyebrow="Focus breakdown"
                title="Scenario type split"
                body="How this player's run volume is distributed across scenario types."
              />
              <ScenarioTypeChart topScenarios={profile.topScenarios} />
            </PageSection>
          )}
        </Grid>
      )}

      <Grid className="grid-cols-2 max-[1100px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Top scenarios"
            title="Where this player spends time"
            body="The scenarios this player returns to most often."
          />
          {profile.topScenarios.length > 0 ? (
            <>
              {scenarioTypes.length > 1 && (
                <TypeFilterBar types={scenarioTypes} active={scenarioTypeFilter} onChange={setScenarioTypeFilter} />
              )}
              <ScrollArea className="max-h-[min(64vh,820px)] pr-2">
                <div className="grid gap-3">
                  {filteredScenarios.map((scenario) => (
                    <Link
                      key={scenario.scenarioSlug}
                      to={`/scenarios/${scenario.scenarioSlug}`}
                      className="rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                    >
                      <strong className="block text-text">{scenario.scenarioName}</strong>
                      <div className="mt-1.5"><ScenarioTypeBadge type={scenario.scenarioType} /></div>
                      <p className="mt-3 text-sm text-mint">{scenario.runCount.toLocaleString()} runs</p>
                    </Link>
                  ))}
                  {filteredScenarios.length === 0 && (
                    <p className="text-sm text-muted">No scenarios match this filter.</p>
                  )}
                </div>
              </ScrollArea>
            </>
          ) : (
            <EmptyState
              title="No scenario history"
              body="This profile does not have any uploaded scenario history yet."
            />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Recent runs"
            title="Latest runs"
            body="The most recent runs on this profile."
          />
          {profile.recentRuns.length > 0 ? (
            <ScrollArea className="max-h-[min(64vh,820px)] overflow-auto rounded-[18px] border border-line bg-white/2">
              <table className="min-w-full text-left text-sm">
                <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                  <tr>
                    <th className="px-4 py-3">Scenario</th>
                    <SortableTh label="Score" field="score" sortField={runSortField} sortDir={runSortDir} onSort={handleRunSort} />
                    <SortableTh label="Acc" field="accuracy" sortField={runSortField} sortDir={runSortDir} onSort={handleRunSort} />
                    <SortableTh label="Duration" field="duration" sortField={runSortField} sortDir={runSortDir} onSort={handleRunSort} />
                    <th className="px-4 py-3">Run</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedRuns.map((run) => (
                    <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0 hover:bg-white/[0.015] transition-colors">
                      <td className="px-4 py-3 text-text">
                        <Link
                          className="text-cyan underline underline-offset-3"
                          to={`/scenarios/${slugifyScenarioName(run.scenarioName)}`}
                        >
                          {run.scenarioName}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-text">{Math.round(run.score).toLocaleString()}</td>
                      <td className="px-4 py-3 text-text">{run.accuracy.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-text">{formatDurationMs(run.durationMs)}</td>
                      <td className="px-4 py-3 text-text">
                        <Link
                          className="text-cyan underline underline-offset-3"
                          to={`/runs/${run.runId || run.sessionId}`}
                        >
                          Open
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </ScrollArea>
          ) : (
            <EmptyState title="No recent runs" body="This profile does not have any uploaded runs yet." />
          )}
        </PageSection>
      </Grid>
    </PageStack>
  );
}
