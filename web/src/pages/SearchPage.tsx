import { useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { filterChoice, updateFilterQuery } from "../lib/savedPageFilters";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { ReplayResultCard } from "../components/ReplayResultCard";
import { SectionHeader } from "../components/SectionHeader";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { VerificationBadge } from "../components/VerificationBadge";
import { Button } from "../components/ui/Button";
import { PageHeader } from "../components/ui/PageHeader";
import { TableSkeleton } from "../components/ui/Skeleton";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import {
  fetchOverview,
  formatDurationMs,
  formatRelativeTime,
  searchHub,
  type HubSearchBenchmark,
  type HubSearchProfile,
  type HubSearchResponse,
  type HubSearchRun,
  type HubSearchScenario,
} from "../lib/api";

type SearchView = "all" | "scenarios" | "players" | "runs" | "replays" | "benchmarks";

type SearchScenarioCardData = Pick<
  HubSearchScenario,
  "scenarioName" | "scenarioSlug" | "scenarioType" | "runCount"
>;

type SearchProfileCardData = Pick<
  HubSearchProfile,
  "userHandle" | "userDisplayName" | "isVerified" | "runCount" | "scenarioCount" | "primaryScenarioType"
>;

type SearchRunCardData = Pick<
  HubSearchRun,
  "publicRunID" | "sessionID" | "scenarioName" | "scenarioType" | "playedAt" | "score" | "accuracy" | "durationMS" | "userHandle" | "userDisplayName"
>;

type SearchQuickResult = {
  kind: "scenario" | "player" | "run" | "benchmark";
  key: string;
  title: string;
  subtitle: string;
  meta: string;
  to: string;
  badge: string;
};

function normalize(value: string) {
  return value.trim().toLowerCase();
}

function containsSubsequence(candidate: string, query: string) {
  if (!candidate || !query) return false;
  let qi = 0;
  for (let i = 0; i < candidate.length && qi < query.length; i += 1) {
    if (candidate[i] === query[qi]) qi += 1;
  }
  return qi === query.length;
}

function matchScore(query: string, candidate: string) {
  const q = normalize(query);
  const c = normalize(candidate);
  if (!q || !c) return 0;
  if (c === q) return 300;
  if (c.startsWith(q)) return 220;
  if (c.split(/[\s_-]+/).some((token) => token.startsWith(q))) return 185;
  if (c.includes(q)) return 140;
  if (containsSubsequence(c, q)) return 90;
  return 0;
}

function rankScenarios(query: string, scenarios: HubSearchScenario[]) {
  return [...scenarios]
    .map((scenario) => ({
      scenario,
      score:
        matchScore(query, scenario.scenarioName) +
        Math.min(scenario.runCount, 200) / 10,
    }))
    .sort((a, b) => b.score - a.score || b.scenario.runCount - a.scenario.runCount);
}

function rankProfiles(query: string, profiles: HubSearchProfile[]) {
  return [...profiles]
    .map((profile) => ({
      profile,
      score:
        Math.max(
          matchScore(query, profile.userHandle),
          matchScore(query, profile.userDisplayName),
        ) +
        Math.min(profile.runCount, 300) / 12,
    }))
    .sort((a, b) => b.score - a.score || b.profile.runCount - a.profile.runCount);
}

function rankRuns(query: string, runs: HubSearchRun[]) {
  return [...runs]
    .map((run) => ({
      run,
      score: Math.max(
        matchScore(query, run.scenarioName),
        matchScore(query, run.publicRunID),
        matchScore(query, run.userHandle),
        matchScore(query, run.userDisplayName),
      ) + Math.min(run.score, 50_000) / 2_000,
    }))
    .sort((a, b) => b.score - a.score || b.run.score - a.run.score);
}

function SearchScenarioCard({ scenario }: { scenario: SearchScenarioCardData }) {
  return (
    <Link
      to={`/scenarios/${scenario.scenarioSlug}`}
      className="rounded-md border border-line bg-white/2 px-4 py-3 transition-colors hover:border-mint/30 hover:bg-white/[0.045]"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <strong className="block truncate text-[15px] text-text">{scenario.scenarioName}</strong>
          <div className="mt-1.5">
            <ScenarioTypeBadge type={scenario.scenarioType} />
          </div>
        </div>
        <span className="shrink-0 text-sm text-mint">{scenario.runCount.toLocaleString()} runs</span>
      </div>
    </Link>
  );
}

function SearchProfileCard({ profile }: { profile: SearchProfileCardData }) {
  return (
    <Link
      to={`/profiles/${profile.userHandle}`}
      className="rounded-md border border-line bg-white/2 px-4 py-3 transition-colors hover:border-cyan/30 hover:bg-white/[0.045]"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <strong className="block truncate text-[15px] text-text">
              {profile.userDisplayName || profile.userHandle}
            </strong>
            <VerificationBadge verified={profile.isVerified} />
          </div>
          <p className="mt-1 truncate text-[12px] text-muted">@{profile.userHandle}</p>
        </div>
        <span className="shrink-0 text-sm text-cyan">{profile.runCount.toLocaleString()} runs</span>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-2">
        <span className="text-[12px] text-muted">{profile.scenarioCount.toLocaleString()} scenarios</span>
        <ScenarioTypeBadge type={profile.primaryScenarioType} />
      </div>
    </Link>
  );
}

function SearchRunCard({ run }: { run: SearchRunCardData }) {
  return (
    <Link
      to={`/runs/${run.publicRunID || run.sessionID}`}
      className="rounded-md border border-line bg-white/2 px-4 py-3 transition-colors hover:border-gold/30 hover:bg-white/[0.045]"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <strong className="block truncate text-[15px] text-text">{run.scenarioName}</strong>
          <p className="mt-1 truncate text-[12px] text-muted">{run.userDisplayName || run.userHandle}</p>
        </div>
        <ScenarioTypeBadge type={run.scenarioType} />
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-muted">
        <span className="text-text">{Math.round(run.score).toLocaleString()} score</span>
        <span>{run.accuracy.toFixed(1)}% acc</span>
        <span>{formatDurationMs(run.durationMS)}</span>
        {run.playedAt ? <span>{formatRelativeTime(run.playedAt)}</span> : null}
      </div>
    </Link>
  );
}

function SearchBenchmarkCard({ benchmark }: { benchmark: HubSearchBenchmark }) {
  return (
    <Link
      to={`/benchmarks/${benchmark.benchmarkId}`}
      className="rounded-md border border-line bg-white/2 px-4 py-3 transition-colors hover:border-cyan/30 hover:bg-white/[0.045]"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          {benchmark.benchmarkIconUrl ? (
            <img src={benchmark.benchmarkIconUrl} alt="" className="h-8 w-8 shrink-0 rounded-lg border border-white/10 object-cover" />
          ) : null}
          <div className="min-w-0">
            <strong className="block truncate text-[15px] text-text">{benchmark.benchmarkName}</strong>
            {benchmark.benchmarkAuthor ? (
              <p className="mt-0.5 truncate text-[12px] text-muted">by {benchmark.benchmarkAuthor}</p>
            ) : null}
          </div>
        </div>
        <span className="shrink-0 text-sm text-cyan">{benchmark.playerCount} player{benchmark.playerCount !== 1 ? "s" : ""}</span>
      </div>
    </Link>
  );
}

function SearchQuickJump({
  items,
  activeIndex,
  onHover,
}: {
  items: SearchQuickResult[];
  activeIndex: number;
  onHover: (index: number) => void;
}) {
  const itemRefs = useRef<(HTMLAnchorElement | null)[]>([]);

  useEffect(() => {
    itemRefs.current[activeIndex]?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [activeIndex]);

  if (items.length === 0) return null;

  return (
    <PageSection className="border-mint/14 bg-[rgba(255,255,255,0.02)]">
      <div className="flex items-center justify-between gap-3">
        <SectionHeader title="Top results" />
        <div className="hidden shrink-0 items-center gap-2 rounded-full border border-line bg-[rgba(255,255,255,0.03)] px-3 py-1 text-[11px] text-muted md:inline-flex">
          <span>↑ ↓ move</span>
          <span className="text-muted-2">•</span>
          <span>Enter open</span>
        </div>
      </div>
      <div className="mt-4 grid gap-2.5">
        {items.map((item, index) => (
          <Link
            key={item.key}
            ref={(el) => { itemRefs.current[index] = el; }}
            to={item.to}
            onMouseEnter={() => onHover(index)}
            className={[
              "rounded-md border px-4 py-3 transition-colors",
              activeIndex === index
                ? "border-mint/40 bg-[rgba(121,201,151,0.12)]"
                : "border-line bg-[rgba(255,255,255,0.03)] hover:border-mint/25 hover:bg-[rgba(255,255,255,0.05)]",
            ].join(" ")}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-normal text-cyan">
                  <span>
                    {item.kind === "scenario" ? "Scenario" : item.kind === "player" ? "Player" : item.kind === "benchmark" ? "Benchmark" : "Run"}
                  </span>
                  <ScenarioTypeBadge type={item.badge} />
                </div>
                <strong className="mt-2 block truncate text-[15px] text-text">{item.title}</strong>
                <p className="mt-1 truncate text-[12px] text-muted">{item.subtitle}</p>
              </div>
              <span className="shrink-0 text-[12px] text-mint">{item.meta}</span>
            </div>
          </Link>
        ))}
      </div>
    </PageSection>
  );
}

function SearchSuggestions({ overview }: { overview: GetOverviewResponse }) {
  return (
    <Grid className="grid-cols-3 max-[1180px]:grid-cols-1">
      <PageSection>
        <SectionHeader title="Popular scenarios" />
        <ScrollArea className="max-h-[420px] pr-2">
          <div className="grid gap-2.5">
            {overview.topScenarios.slice(0, 10).map((scenario) => (
              <SearchScenarioCard key={scenario.scenarioSlug} scenario={scenario} />
            ))}
          </div>
        </ScrollArea>
      </PageSection>

      <PageSection>
        <SectionHeader title="Most active players" />
        <ScrollArea className="max-h-[420px] pr-2">
          <div className="grid gap-2.5">
            {overview.activeProfiles.slice(0, 10).map((profile) => (
              <SearchProfileCard key={profile.userHandle} profile={profile} />
            ))}
          </div>
        </ScrollArea>
      </PageSection>

      <PageSection>
        <SectionHeader title="Recent runs" />
        <ScrollArea className="max-h-[420px] pr-2">
          <div className="grid gap-2.5">
            {overview.recentRuns.slice(0, 10).map((run) => (
              <SearchRunCard key={run.runId || run.sessionId} run={{
                publicRunID: run.runId,
                sessionID: run.sessionId,
                scenarioName: run.scenarioName,
                scenarioType: run.scenarioType,
                playedAt: run.playedAtIso,
                score: run.score,
                accuracy: run.accuracy,
                durationMS: Number(run.durationMs),
                userHandle: run.userHandle,
                userDisplayName: run.userDisplayName,
              }} />
            ))}
          </div>
        </ScrollArea>
      </PageSection>
    </Grid>
  );
}

export function SearchPage() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const query = params.get("q")?.trim() ?? "";
  const [draftQuery, setDraftQuery] = useState(query);
  const [results, setResults] = useState<HubSearchResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [overview, setOverview] = useState<GetOverviewResponse | null>(null);
  const view = filterChoice<SearchView>(params.get("view"), ["all", "scenarios", "players", "runs", "replays", "benchmarks"], "all");
  const setView = (view: SearchView) => setParams(current => updateFilterQuery(current, { view }), { replace: true });
  const [activeQuickIndex, setActiveQuickIndex] = useState(0);
  const [quickSelectionActive, setQuickSelectionActive] = useState(false);

  useEffect(() => {
    setDraftQuery(query);
  }, [query]);

  // Auto-query as the user types
  useEffect(() => {
    const tid = setTimeout(() => {
      const next = draftQuery.trim();
      if (next === query) return;
      setParams(current => updateFilterQuery(current, { q: next }), { replace: true });
    }, 400);
    return () => clearTimeout(tid);
  }, [draftQuery, query, setParams]);

  useEffect(() => {
    if (!query) {
      void fetchOverview().then(setOverview).catch(() => {});
    }
  }, [query]);

  useEffect(() => {
    setActiveQuickIndex(0);
    setQuickSelectionActive(false);
  }, [query, view]);

  useEffect(() => {
    let cancelled = false;
    setResults(null);
    setError(null);
    if (!query) return;
    void searchHub(query)
      .then((next) => {
        if (!cancelled) setResults(next);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Could not search the hub.");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [query]);

  useEffect(() => {
    if (!query) return;
    const id = window.setInterval(() => {
      void searchHub(query).then(setResults).catch(() => {});
    }, 60_000);
    return () => window.clearInterval(id);
  }, [query]);

  const scenarioCount = results?.scenarios.length ?? 0;
  const profileCount = results?.profiles.length ?? 0;
  const runCount = results?.runs.length ?? 0;
  const replayCount = results?.replays.length ?? 0;
  const benchmarkCount = results?.benchmarks.length ?? 0;
  const totalCount = scenarioCount + profileCount + runCount + replayCount + benchmarkCount;
  const hasResults = totalCount > 0;

  const ranked = useMemo(() => {
    if (!results || !query) {
      return {
        scenario: null,
        profile: null,
        run: null,
        scenarios: [] as HubSearchScenario[],
        profiles: [] as HubSearchProfile[],
        runs: [] as HubSearchRun[],
        replays: [] as HubSearchRun[],
      };
    }
    const rankedScenarios = rankScenarios(query, results.scenarios);
    const rankedProfiles = rankProfiles(query, results.profiles);
    const rankedRuns = rankRuns(query, results.runs);
    const rankedReplays = rankRuns(query, results.replays);
    return {
      scenario: rankedScenarios[0]?.scenario ?? null,
      profile: rankedProfiles[0]?.profile ?? null,
      run: rankedRuns[0]?.run ?? null,
      scenarios: rankedScenarios.map((entry) => entry.scenario),
      profiles: rankedProfiles.map((entry) => entry.profile),
      runs: rankedRuns.map((entry) => entry.run),
      replays: rankedReplays.map((entry) => entry.run),
    };
  }, [results, query]);

  const quickResults = useMemo<SearchQuickResult[]>(() => {
    if (!query) return [];
    return [
      ...ranked.scenarios.slice(0, 3).map((scenario) => ({
        kind: "scenario" as const,
        key: `scenario:${scenario.scenarioSlug}`,
        title: scenario.scenarioName,
        subtitle: "Scenario page",
        meta: `${scenario.runCount.toLocaleString()} runs`,
        to: `/scenarios/${scenario.scenarioSlug}`,
        badge: scenario.scenarioType,
      })),
      ...ranked.profiles.slice(0, 3).map((profile) => ({
        kind: "player" as const,
        key: `profile:${profile.userHandle}`,
        title: profile.userDisplayName || profile.userHandle,
        subtitle: `@${profile.userHandle}`,
        meta: `${profile.runCount.toLocaleString()} runs`,
        to: `/profiles/${profile.userHandle}`,
        badge: profile.primaryScenarioType,
      })),
      ...ranked.runs.slice(0, 4).map((run) => ({
        kind: "run" as const,
        key: `run:${run.publicRunID || run.sessionID}`,
        title: run.scenarioName,
        subtitle: run.userDisplayName || run.userHandle,
        meta: `${Math.round(run.score).toLocaleString()} score`,
        to: `/runs/${run.publicRunID || run.sessionID}`,
        badge: run.scenarioType,
      })),
      ...ranked.replays.slice(0, 2).map((run) => ({
        kind: "run" as const,
        key: `replay:${run.publicRunID || run.sessionID}`,
        title: `${run.scenarioName} replay`,
        subtitle: run.userDisplayName || run.userHandle,
        meta: run.hasVideo ? "video replay" : "mouse path",
        to: `/runs/${run.publicRunID || run.sessionID}`,
        badge: run.scenarioType,
      })),
      ...(results?.benchmarks ?? []).slice(0, 2).map((b) => ({
        kind: "benchmark" as const,
        key: `benchmark:${b.benchmarkId}`,
        title: b.benchmarkName,
        subtitle: b.benchmarkAuthor ? `by ${b.benchmarkAuthor}` : "Benchmark",
        meta: `${b.playerCount} player${b.playerCount !== 1 ? "s" : ""}`,
        to: `/benchmarks/${b.benchmarkId}`,
        badge: b.benchmarkType,
      })),
    ].slice(0, 8);
  }, [query, ranked, results]);

  useEffect(() => {
    if (!query || quickResults.length === 0) return;

    function handlePageKeyDown(event: globalThis.KeyboardEvent) {
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      const active = document.activeElement as HTMLElement | null;
      const tag = active?.tagName;
      const isTypingTarget =
        tag === "INPUT" ||
        tag === "TEXTAREA" ||
        tag === "SELECT" ||
        active?.isContentEditable;

      if (event.key === "ArrowDown") {
        event.preventDefault();
        setQuickSelectionActive(true);
        setActiveQuickIndex((current) => (current + 1) % quickResults.length);
        return;
      }

      if (event.key === "ArrowUp") {
        event.preventDefault();
        setQuickSelectionActive(true);
        setActiveQuickIndex((current) => (current - 1 + quickResults.length) % quickResults.length);
        return;
      }

      if (event.key === "Escape") {
        setActiveQuickIndex(0);
        setQuickSelectionActive(false);
        return;
      }

      if (event.key === "Enter" && quickSelectionActive && !isTypingTarget) {
        const activeQuickResult = quickResults[activeQuickIndex];
        if (!activeQuickResult) return;
        event.preventDefault();
        navigate(activeQuickResult.to);
      }
    }

    window.addEventListener("keydown", handlePageKeyDown);
    return () => window.removeEventListener("keydown", handlePageKeyDown);
  }, [query, quickResults, quickSelectionActive, activeQuickIndex, navigate]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const activeQuickResult = quickSelectionActive ? quickResults[activeQuickIndex] : null;
    if (activeQuickResult) {
      navigate(activeQuickResult.to);
      return;
    }
    const next = draftQuery.trim();
    setParams(current => updateFilterQuery(current, { q: next }));
  }

  function handleInputKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (!quickResults.length) return;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setQuickSelectionActive(true);
      setActiveQuickIndex((current) => (current + 1) % quickResults.length);
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setQuickSelectionActive(true);
      setActiveQuickIndex((current) => (current - 1 + quickResults.length) % quickResults.length);
      return;
    }
    if (event.key === "Escape") {
      setActiveQuickIndex(0);
      setQuickSelectionActive(false);
    }
  }

  // In the combined view, only sections with matches are shown.
  const showScenarios = view === "scenarios" || (view === "all" && scenarioCount > 0);
  const showProfiles = view === "players" || (view === "all" && profileCount > 0);
  const showRuns = view === "runs" || (view === "all" && runCount > 0);
  const showReplays = view === "replays" || (view === "all" && replayCount > 0);
  const showBenchmarks = view === "benchmarks" || (view === "all" && benchmarkCount > 0);

  return (
    <PageStack>
      <Helmet>
        <title>{query ? `"${query}" · Search · AimMod Hub` : "Search · AimMod Hub"}</title>
        <meta name="description" content={query ? `Search results for "${query}" on AimMod Hub.` : "Search for players, scenarios, runs, and replays across AimMod Hub."} />
      </Helmet>
      <PageSection>
        <PageHeader
          title={query ? `Results for “${query}”` : "Search"}
          meta={query && results ? `${totalCount.toLocaleString()} ${totalCount === 1 ? "result" : "results"}` : "Players, scenarios, runs, replays and benchmarks"}
        />
        <form onSubmit={handleSubmit} className="mt-4 flex flex-wrap items-center gap-2">
          <input
            value={draftQuery}
            onChange={(event) => {
              setDraftQuery(event.target.value);
              setQuickSelectionActive(false);
            }}
            onKeyDown={handleInputKeyDown}
            aria-label="Search"
            type="search"
            placeholder="Player, scenario or run id"
            className="min-h-10 min-w-0 flex-1 rounded-md border border-line bg-panel px-3 text-sm text-text placeholder:text-muted-2"
          />
          <Button type="submit" variant="primary">Search</Button>
          {query ? (
            <Button
              type="button"
              onClick={() => {
                setDraftQuery("");
                setParams(current => updateFilterQuery(current, { q: null }));
              }}
            >
              Clear
            </Button>
          ) : null}
        </form>
        {query ? (
          <div className="mt-4 flex flex-wrap gap-2">
            {[
              { key: "all", label: "All", count: totalCount },
              { key: "scenarios", label: "Scenarios", count: scenarioCount },
              { key: "players", label: "Players", count: profileCount },
              { key: "benchmarks", label: "Benchmarks", count: benchmarkCount },
              { key: "runs", label: "Runs", count: runCount },
              { key: "replays", label: "Replays", count: replayCount },
            ].map((item) => (
              <button
                key={item.key}
                onClick={() => setView(item.key as SearchView)}
                className={[
                  "rounded-full border px-3 py-1.5 text-[12px] transition-colors",
                  view === item.key
                    ? "border-mint/40 bg-[rgba(121,201,151,0.14)] text-text"
                    : "border-line bg-[rgba(255,255,255,0.03)] text-muted hover:text-text",
                ].join(" ")}
              >
                {item.label} <span className="text-muted-2">{item.count}</span>
              </button>
            ))}
          </div>
        ) : null}
      </PageSection>

      {!query ? (
        overview ? (
          <SearchSuggestions overview={overview} />
        ) : (
          <TableSkeleton rows={6} />
        )
      ) : error ? (
        <PageSection>
          <EmptyState title="Search could not be loaded." body="Try again in a moment." />
        </PageSection>
      ) : !results ? (
        <div role="status" aria-label="Searching"><TableSkeleton rows={6} /></div>
        ) : !hasResults ? (
        <PageSection>
          <EmptyState title={`Nothing matches “${query}”.`} body="Try part of a player or scenario name.">
            <Button to="/community">Browse players & scenarios</Button>
          </EmptyState>
        </PageSection>
      ) : (
        <>
          {view === "all" && <SearchQuickJump
            items={quickResults}
            activeIndex={activeQuickIndex}
            onHover={(index) => {
              setActiveQuickIndex(index);
              setQuickSelectionActive(true);
            }}
          />}

          <Grid className="grid-cols-3 items-start max-[1280px]:grid-cols-1">
            {showReplays ? (
              <PageSection>
                <SectionHeader title="Replays" />
                <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                  <div className="grid gap-2.5">
                    {replayCount > 0
                      ? ranked.replays.map((run) => (
                          <ReplayResultCard key={`replay:${run.publicRunID || run.sessionID}`} run={run} />
                        ))
                      : <EmptyState title="No replays match." />}
                  </div>
                </ScrollArea>
              </PageSection>
            ) : null}

            {showScenarios ? (
              <PageSection>
                <SectionHeader title="Scenarios" />
                <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                  <div className="grid gap-2.5">
                    {scenarioCount > 0
                      ? ranked.scenarios.map((scenario) => (
                          <SearchScenarioCard key={scenario.scenarioSlug} scenario={scenario} />
                        ))
                      : <EmptyState title="No scenarios match." />}
                  </div>
                </ScrollArea>
              </PageSection>
            ) : null}

            {showProfiles ? (
              <PageSection>
                <SectionHeader title="Players" />
                <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                  <div className="grid gap-2.5">
                    {profileCount > 0
                      ? ranked.profiles.map((profile) => (
                          <SearchProfileCard key={profile.userHandle} profile={profile} />
                        ))
                      : <EmptyState title="No players match." />}
                  </div>
                </ScrollArea>
              </PageSection>
            ) : null}

            {showBenchmarks && benchmarkCount > 0 ? (
              <PageSection>
                <SectionHeader title="Benchmarks" />
                <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                  <div className="grid gap-2.5">
                    {(results?.benchmarks ?? []).map((b) => (
                      <SearchBenchmarkCard key={b.benchmarkId} benchmark={b} />
                    ))}
                  </div>
                </ScrollArea>
              </PageSection>
            ) : null}

            {showRuns ? (
              <PageSection>
                <SectionHeader title="Runs" />
                <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                  <div className="grid gap-2.5">
                    {runCount > 0
                      ? ranked.runs.map((run) => (
                          <SearchRunCard key={run.publicRunID || run.sessionID} run={run} />
                        ))
                      : <EmptyState title="No runs match." />}
                  </div>
                </ScrollArea>
              </PageSection>
            ) : null}
          </Grid>
        </>
      )}
    </PageStack>
  );
}
