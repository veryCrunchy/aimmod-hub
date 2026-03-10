import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "../components/SectionHeader";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { fetchOverview, formatDurationMs, formatRelativeTime, searchHub, type HubSearchResponse } from "../lib/api";

export function SearchPage() {
  const [params] = useSearchParams();
  const query = params.get("q")?.trim() ?? "";
  const [results, setResults] = useState<HubSearchResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [overview, setOverview] = useState<GetOverviewResponse | null>(null);

  useEffect(() => {
    if (!query) {
      void fetchOverview().then(setOverview).catch(() => {});
    }
  }, [query]);

  useEffect(() => {
    let cancelled = false;
    setResults(null);
    setError(null);
    if (!query) return;
    void searchHub(query)
      .then((next) => { if (!cancelled) setResults(next); })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Could not search the hub.");
      });
    return () => { cancelled = true; };
  }, [query]);

  if (!query) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader
            eyebrow="Search"
            title="Find players, scenarios, and runs"
            body="Type in the search bar above to jump to a player profile, scenario page, or specific run."
          />
        </PageSection>

        {overview && (
          <Grid className="grid-cols-2 max-[900px]:grid-cols-1">
            <PageSection>
              <SectionHeader eyebrow="Top scenarios" title="Most active scenarios" />
              <ScrollArea className="max-h-[min(60vh,720px)] pr-2">
                <div className="grid gap-3">
                  {overview.topScenarios.slice(0, 8).map((scenario) => (
                    <Link
                      key={scenario.scenarioSlug}
                      to={`/scenarios/${scenario.scenarioSlug}`}
                      className="rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <strong className="block text-text">{scenario.scenarioName}</strong>
                          <div className="mt-1.5"><ScenarioTypeBadge type={scenario.scenarioType} /></div>
                        </div>
                        <span className="shrink-0 text-sm text-mint">{scenario.runCount.toLocaleString()} runs</span>
                      </div>
                    </Link>
                  ))}
                </div>
              </ScrollArea>
            </PageSection>

            <PageSection>
              <SectionHeader eyebrow="Active players" title="Most active profiles" />
              <ScrollArea className="max-h-[min(60vh,720px)] pr-2">
                <div className="grid gap-3">
                  {overview.activeProfiles.slice(0, 8).map((profile) => (
                    <Link
                      key={profile.userHandle}
                      to={`/profiles/${profile.userHandle}`}
                      className="rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <strong className="block truncate text-text">{profile.userDisplayName || profile.userHandle}</strong>
                          <p className="mt-1 text-sm text-muted">@{profile.userHandle}</p>
                        </div>
                        <span className="shrink-0 text-sm text-cyan">{profile.runCount.toLocaleString()} runs</span>
                      </div>
                      <div className="mt-2 flex items-center gap-2">
                        <span className="text-sm text-muted">{profile.scenarioCount.toLocaleString()} scenarios</span>
                        <ScenarioTypeBadge type={profile.primaryScenarioType} />
                      </div>
                    </Link>
                  ))}
                </div>
              </ScrollArea>
            </PageSection>
          </Grid>
        )}
      </PageStack>
    );
  }

  if (error) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Search" title={`Results for “${query}”`} />
          <EmptyState title="Search is unavailable right now" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  const scenarioCount = results?.scenarios.length ?? 0;
  const profileCount = results?.profiles.length ?? 0;
  const runCount = results?.runs.length ?? 0;
  const hasResults = scenarioCount > 0 || profileCount > 0 || runCount > 0;

  return (
    <PageStack>
      <PageSection>
        <SectionHeader
          eyebrow="Search"
          title={`Results for “${query}”`}
          body={
            hasResults
              ? `${scenarioCount} scenarios • ${profileCount} players • ${runCount} runs`
              : "No matches yet."
          }
        />
      </PageSection>

      {!results ? (
        <PageSection>
          <SectionHeader eyebrow="Search" title="Searching" />
        </PageSection>
      ) : !hasResults ? (
        <PageSection>
          <EmptyState
            title="No matches found"
            body="Try a scenario name, a player name, or part of a run id."
          />
        </PageSection>
      ) : (
        <Grid className="grid-cols-3 items-start max-[1280px]:grid-cols-1">
          <PageSection>
            <SectionHeader eyebrow="Scenarios" title="Matching scenario pages" />
            {scenarioCount > 0 ? (
              <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                <div className="grid gap-3">
                  {results.scenarios.map((scenario) => (
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
                </div>
              </ScrollArea>
            ) : (
              <EmptyState title="No scenario matches" body="No scenario names matched this search." />
            )}
          </PageSection>

          <PageSection>
            <SectionHeader eyebrow="Players" title="Matching profiles" />
            {profileCount > 0 ? (
              <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                <div className="grid gap-3">
                  {results.profiles.map((profile) => (
                    <Link
                      key={profile.userHandle}
                      to={`/profiles/${profile.userHandle}`}
                      className="rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                    >
                      <strong className="block text-text">{profile.userDisplayName || profile.userHandle}</strong>
                      <p className="mt-1 text-sm text-muted">@{profile.userHandle}</p>
                      <div className="mt-3 flex items-center gap-2">
                        <span className="text-sm text-muted">{profile.scenarioCount.toLocaleString()} scenarios</span>
                        <ScenarioTypeBadge type={profile.primaryScenarioType} />
                      </div>
                      <p className="mt-1 text-sm text-cyan">{profile.runCount.toLocaleString()} runs</p>
                    </Link>
                  ))}
                </div>
              </ScrollArea>
            ) : (
              <EmptyState title="No player matches" body="No player profiles matched this search." />
            )}
          </PageSection>

          <PageSection>
            <SectionHeader eyebrow="Runs" title="Matching runs" />
            {runCount > 0 ? (
              <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                <div className="grid gap-3">
                  {results.runs.map((run) => (
                    <Link
                      key={run.publicRunID || run.sessionID}
                      to={`/runs/${run.publicRunID || run.sessionID}`}
                      className="rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                    >
                      <div className="flex items-start justify-between gap-2">
                        <strong className="block text-text">{run.scenarioName}</strong>
                        <ScenarioTypeBadge type={run.scenarioType} />
                      </div>
                      <p className="mt-1 text-sm text-muted">{run.userDisplayName || run.userHandle}</p>
                      <div className="mt-3 grid gap-1 text-sm text-muted">
                        <span>{Math.round(run.score).toLocaleString()} score · {run.accuracy.toFixed(1)}% accuracy</span>
                        <span>{formatDurationMs(run.durationMS)}{run.playedAt ? ` · ${formatRelativeTime(run.playedAt)}` : ""}</span>
                      </div>
                    </Link>
                  ))}
                </div>
              </ScrollArea>
            ) : (
              <EmptyState title="No run matches" body="No runs matched this search." />
            )}
          </PageSection>
        </Grid>
      )}
    </PageStack>
  );
}
