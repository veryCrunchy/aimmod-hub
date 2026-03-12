import { useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { Helmet } from "react-helmet-async";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { ReplayResultCard } from "../components/ReplayResultCard";
import { SectionHeader } from "../components/SectionHeader";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { VerificationBadge } from "../components/VerificationBadge";
import { Button } from "../components/ui/Button";
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
      className="rounded-[16px] border border-line bg-white/2 px-4 py-3 transition-colors hover:border-mint/30 hover:bg-white/[0.045]"
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
      className="rounded-[16px] border border-line bg-white/2 px-4 py-3 transition-colors hover:border-cyan/30 hover:bg-white/[0.045]"
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
      className="rounded-[16px] border border-line bg-white/2 px-4 py-3 transition-colors hover:border-gold/30 hover:bg-white/[0.045]"
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
      className="rounded-[16px] border border-line bg-white/2 px-4 py-3 transition-colors hover:border-cyan/30 hover:bg-white/[0.045]"
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
        <SectionHeader
          eyebrow="Quick jump"
          title="Best places to open next"
          body="Use the arrow keys and press Enter to open the highlighted result."
        />
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
              "rounded-[16px] border px-4 py-3 transition-colors",
              activeIndex === index
                ? "border-mint/40 bg-[rgba(121,201,151,0.12)]"
                : "border-line bg-[rgba(255,255,255,0.03)] hover:border-mint/25 hover:bg-[rgba(255,255,255,0.05)]",
            ].join(" ")}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.08em] text-cyan">
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

function SearchBestMatch({
  query,
  scenario,
  profile,
  run,
}: {
  query: string;
  scenario: HubSearchScenario | null;
  profile: HubSearchProfile | null;
  run: HubSearchRun | null;
}) {
  const candidates = [
    scenario
      ? {
          kind: "Scenario",
          title: scenario.scenarioName,
          body: "Jump straight into the scenario page with all uploaded history and leaderboard context.",
          to: `/scenarios/${scenario.scenarioSlug}`,
          meta: `${scenario.runCount.toLocaleString()} runs`,
          badge: scenario.scenarioType,
        }
      : null,
    profile
      ? {
          kind: "Player",
          title: profile.userDisplayName || profile.userHandle,
          body: "Open the player profile to see their top scenarios, recent runs, and overall practice shape.",
          to: `/profiles/${profile.userHandle}`,
          meta: `${profile.runCount.toLocaleString()} runs · ${profile.scenarioCount.toLocaleString()} scenarios`,
          badge: profile.primaryScenarioType,
        }
      : null,
    run
      ? {
          kind: "Run",
          title: run.scenarioName,
          body: `Open the run from ${run.userDisplayName || run.userHandle} and inspect the saved detail.`,
          to: `/runs/${run.publicRunID || run.sessionID}`,
          meta: `${Math.round(run.score).toLocaleString()} score · ${run.accuracy.toFixed(1)}% acc`,
          badge: run.scenarioType,
        }
      : null,
  ].filter(Boolean) as {
    kind: string;
    title: string;
    body: string;
    to: string;
    meta: string;
    badge: string;
  }[];

  const best = candidates[0] ?? null;
  if (!best) return null;

  return (
    <PageSection className="border-mint/18 bg-[radial-gradient(circle_at_top_left,rgba(121,201,151,0.16),transparent_28%),linear-gradient(135deg,rgba(9,25,18,0.98),rgba(4,12,9,0.98))]">
      <SectionHeader
        eyebrow="Best match"
        title={`Most likely match for “${query}”`}
        body="Use this when you already know roughly what you want and just want to get there fast."
      />
      <Link
        to={best.to}
        className="block rounded-[18px] border border-mint/18 bg-[rgba(255,255,255,0.03)] px-5 py-4 transition-colors hover:border-mint/35 hover:bg-[rgba(255,255,255,0.05)]"
      >
        <div className="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.08em] text-cyan">
          <span>{best.kind}</span>
          <ScenarioTypeBadge type={best.badge} />
        </div>
        <h3 className="mt-3 text-[24px] leading-[1.05] text-text">{best.title}</h3>
        <p className="mt-3 max-w-[70ch] text-[14px] leading-6 text-muted">{best.body}</p>
        <p className="mt-4 text-[13px] text-mint">{best.meta}</p>
      </Link>
    </PageSection>
  );
}

function SearchSuggestions({ overview }: { overview: GetOverviewResponse }) {
  return (
    <Grid className="grid-cols-3 max-[1180px]:grid-cols-1">
      <PageSection>
        <SectionHeader eyebrow="Popular scenarios" title="Jump into active pages" body="The scenarios with the most uploaded history right now." />
        <ScrollArea className="max-h-[420px] pr-2">
          <div className="grid gap-2.5">
            {overview.topScenarios.slice(0, 10).map((scenario) => (
              <SearchScenarioCard key={scenario.scenarioSlug} scenario={scenario} />
            ))}
          </div>
        </ScrollArea>
      </PageSection>

      <PageSection>
        <SectionHeader eyebrow="Active players" title="Profiles worth exploring" body="The players contributing the most recent usable history." />
        <ScrollArea className="max-h-[420px] pr-2">
          <div className="grid gap-2.5">
            {overview.activeProfiles.slice(0, 10).map((profile) => (
              <SearchProfileCard key={profile.userHandle} profile={profile} />
            ))}
          </div>
        </ScrollArea>
      </PageSection>

      <PageSection>
        <SectionHeader eyebrow="Recent runs" title="Fresh runs worth opening" body="The newest uploaded runs across the hub." />
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
  const [params] = useSearchParams();
  const query = params.get("q")?.trim() ?? "";
  const [draftQuery, setDraftQuery] = useState(query);
  const [results, setResults] = useState<HubSearchResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [overview, setOverview] = useState<GetOverviewResponse | null>(null);
  const [view, setView] = useState<SearchView>("all");
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
      if (!next) {
        navigate("/search", { replace: true });
      } else {
        navigate(`/search?q=${encodeURIComponent(next)}`, { replace: true });
      }
    }, 400);
    return () => clearTimeout(tid);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draftQuery]);

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
    if (!next) {
      navigate("/search");
      return;
    }
    navigate(`/search?q=${encodeURIComponent(next)}`);
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

  const showScenarios = view === "all" || view === "scenarios";
  const showProfiles = view === "all" || view === "players";
  const showRuns = view === "all" || view === "runs";
  const showReplays = view === "all" || view === "replays";
  const showBenchmarks = view === "all" || view === "benchmarks";

  return (
    <PageStack>
      <Helmet>
        <title>{query ? `"${query}" · Search · AimMod Hub` : "Search · AimMod Hub"}</title>
        <meta name="description" content={query ? `Search results for "${query}" on AimMod Hub.` : "Search for players, scenarios, runs, and replays across AimMod Hub."} />
      </Helmet>
      <PageSection className="border-mint/18 bg-[radial-gradient(circle_at_top_left,rgba(121,201,151,0.14),transparent_26%),linear-gradient(135deg,rgba(9,25,18,0.98),rgba(4,12,9,0.98))]">
        <SectionHeader
          eyebrow="Search"
          title={query ? `Results for “${query}”` : "Find players, scenarios, runs, and replays"}
          body={
            query
              ? hasResults
                ? `${totalCount} results across scenarios, players, and runs.`
                : "No matches yet."
              : "Search the whole hub from one place, then jump straight into the page, run, or replay you want."
          }
        />
        <form onSubmit={handleSubmit} className="mt-4 flex flex-wrap items-center gap-2">
          <input
            value={draftQuery}
            onChange={(event) => {
              setDraftQuery(event.target.value);
              setQuickSelectionActive(false);
            }}
            onKeyDown={handleInputKeyDown}
            placeholder="Search scenarios, players, run ids, or replay-ready runs"
            className="min-w-0 flex-1 rounded-full border border-line bg-[rgba(255,255,255,0.03)] px-4 py-2.5 text-sm text-text outline-none placeholder:text-muted focus:border-mint/70"
          />
          <Button type="submit" variant="primary">Search</Button>
          {query ? (
            <Button
              type="button"
              onClick={() => {
                setDraftQuery("");
                navigate("/search");
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
          <PageSection>
            <SectionHeader eyebrow="Search" title="Loading the hub" body="Pulling in a few useful starting points." />
          </PageSection>
        )
      ) : error ? (
        <PageSection>
          <EmptyState title="Search is unavailable right now" body={error} />
        </PageSection>
      ) : !results ? (
        <PageSection>
          <SectionHeader eyebrow="Search" title="Searching" body="Looking through players, scenarios, and runs." />
        </PageSection>
        ) : !hasResults ? (
        <PageSection>
          <EmptyState
            title="No matches found"
            body="Try a scenario name, a player name, or part of a run id."
          />
        </PageSection>
      ) : (
        <>
          <SearchQuickJump
            items={quickResults}
            activeIndex={activeQuickIndex}
            onHover={(index) => {
              setActiveQuickIndex(index);
              setQuickSelectionActive(true);
            }}
          />

          <SearchBestMatch
            query={query}
            scenario={ranked.scenario}
            profile={ranked.profile}
            run={ranked.run}
          />

          <Grid className="grid-cols-3 items-start max-[1280px]:grid-cols-1">
            {showReplays ? (
              <PageSection>
                <SectionHeader eyebrow="Replays" title="Replay-ready runs" body="Runs that already have video replay or mouse path data available." />
                <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                  <div className="grid gap-2.5">
                    {replayCount > 0
                      ? ranked.replays.map((run) => (
                          <ReplayResultCard key={`replay:${run.publicRunID || run.sessionID}`} run={run} />
                        ))
                      : <EmptyState title="No replay matches" body="No replay-ready runs matched this search." />}
                  </div>
                </ScrollArea>
              </PageSection>
            ) : null}

            {showScenarios ? (
              <PageSection>
                <SectionHeader eyebrow="Scenarios" title="Scenario pages" body="Open the scenario itself to see shared history, leaderboards, and activity." />
                <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                  <div className="grid gap-2.5">
                    {scenarioCount > 0
                      ? ranked.scenarios.map((scenario) => (
                          <SearchScenarioCard key={scenario.scenarioSlug} scenario={scenario} />
                        ))
                      : <EmptyState title="No scenario matches" body="No scenario names matched this search." />}
                  </div>
                </ScrollArea>
              </PageSection>
            ) : null}

            {showProfiles ? (
              <PageSection>
                <SectionHeader eyebrow="Players" title="Profiles" body="Jump into a player’s uploaded history and top scenarios." />
                <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                  <div className="grid gap-2.5">
                    {profileCount > 0
                      ? ranked.profiles.map((profile) => (
                          <SearchProfileCard key={profile.userHandle} profile={profile} />
                        ))
                      : <EmptyState title="No player matches" body="No player profiles matched this search." />}
                  </div>
                </ScrollArea>
              </PageSection>
            ) : null}

            {showBenchmarks && benchmarkCount > 0 ? (
              <PageSection>
                <SectionHeader eyebrow="Benchmarks" title="Benchmark ranks" body="Hub players ranked in benchmarks matching your search." />
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
                <SectionHeader eyebrow="Runs" title="Run results" body="Useful when you want one exact run instead of the whole scenario or profile page." />
                <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
                  <div className="grid gap-2.5">
                    {runCount > 0
                      ? ranked.runs.map((run) => (
                          <SearchRunCard key={run.publicRunID || run.sessionID} run={run} />
                        ))
                      : <EmptyState title="No run matches" body="No runs matched this search." />}
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
