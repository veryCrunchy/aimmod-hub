import { useCallback, useEffect, useMemo, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useSearchParams } from "react-router-dom";
import { filterChoice, updateFilterQuery } from "../lib/savedPageFilters";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { VerificationBadge } from "../components/VerificationBadge";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageHeader } from "../components/ui/PageHeader";
import { Section } from "../components/ui/Section";
import { PageSkeleton } from "../components/ui/Skeleton";
import { SortableTh } from "../components/ui/SortableTh";
import { TypeFilterBar } from "../components/ui/TypeFilterBar";
import { PageStack } from "../components/ui/Stack";
import { displayScenarioType, fetchOverview } from "../lib/api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";

type ScenarioSortField = "runCount" | "name";

export function CommunityPage() {
  const [overview, setOverview] = useState<GetOverviewResponse | null>(null);
  const [error, setError] = useState(false);
  const [params, setParams] = useSearchParams();
  const sortField = filterChoice<ScenarioSortField>(params.get("sort"), ["runCount", "name"], "runCount");
  const sortDir = filterChoice(params.get("direction"), ["asc", "desc"] as const, "desc");
  const query = params.get("q") ?? "";
  const setQuery = (q: string) => setParams(current => updateFilterQuery(current, { q: q || null }), { replace: true });
  const setScenarioTypeFilter = (scenarioType: string | null) => setParams(current => updateFilterQuery(current, { scenarioType }), { replace: true });

  const load = useCallback(() => {
    void fetchOverview()
      .then((next) => { setOverview(next); setError(false); })
      .catch(() => setError(true));
  }, []);
  useEffect(load, [load]);
  useAutoRefresh(load, 60_000);

  function handleScenarioSort(field: string) {
    const f = filterChoice<ScenarioSortField>(field, ["runCount", "name"], "runCount");
    const direction = f === sortField ? (sortDir === "asc" ? "desc" : "asc") : f === "name" ? "asc" : "desc";
    setParams(current => updateFilterQuery(current, { sort: f, direction }), { replace: true });
  }

  const scenarioTypes = useMemo(() => {
    const types = new Set<string>();
    for (const s of overview?.topScenarios ?? []) {
      if (s.scenarioType?.trim() && s.scenarioType !== "Unknown") types.add(s.scenarioType);
    }
    return [...types].sort();
  }, [overview]);
  const typeFilter = scenarioTypes.includes(params.get("scenarioType") ?? "") ? params.get("scenarioType") : null;

  const q = query.trim().toLowerCase();
  const scenarios = useMemo(() => {
    const list = (overview?.topScenarios ?? []).filter((s) => (!typeFilter || s.scenarioType === typeFilter) && (!q || s.scenarioName.toLowerCase().includes(q)));
    return list.sort((a, b) => {
      const diff = sortField === "runCount" ? Number(a.runCount) - Number(b.runCount) : a.scenarioName.localeCompare(b.scenarioName);
      return sortDir === "asc" ? diff : -diff;
    });
  }, [overview, typeFilter, q, sortField, sortDir]);
  const players = useMemo(() => (overview?.activeProfiles ?? []).filter((p) => !q || `${p.userDisplayName} ${p.userHandle}`.toLowerCase().includes(q)), [overview, q]);

  const head = <Helmet>
    <title>Players & scenarios · AimMod Hub</title>
    <meta name="description" content="KovaaK's scenarios and players with runs shared through AimMod." />
  </Helmet>;

  if (!overview) {
    return <PageStack>{head}{error
      ? <><PageHeader title="Players & scenarios" /><EmptyState title="This list could not be loaded."><Button onClick={load}>Try again</Button></EmptyState></>
      : <PageSkeleton stats={0} label="Loading players and scenarios" />}</PageStack>;
  }

  const totalRuns = Math.max(1, Number(overview.totalRuns));

  return (
    <PageStack>
      {head}
      <PageHeader
        title="Players & scenarios"
        meta={`${overview.totalPlayers.toLocaleString()} players · ${overview.totalScenarios.toLocaleString()} scenarios`}
      />

      <div className="flex flex-wrap items-center gap-3">
        <label className="sr-only" htmlFor="community-search">Filter players and scenarios</label>
        <input id="community-search" type="search" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Filter by name"
          className="min-h-9 w-full max-w-xs rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted-2" />
        <Link to={`/search${q ? `?q=${encodeURIComponent(query.trim())}` : ""}`} className="text-sm text-cyan hover:underline">Search everything</Link>
      </div>

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <Section title="Scenarios" aside={`${scenarios.length.toLocaleString()} shown`}>
          {scenarioTypes.length > 1 && <TypeFilterBar types={scenarioTypes} active={typeFilter} onChange={setScenarioTypeFilter} className="mb-3" />}
          {scenarios.length ? (
            <div className="overflow-x-auto rounded-md border border-line">
              <table className="w-full sm:min-w-[420px] text-left text-sm">
                <thead className="border-b border-line text-xs text-muted">
                  <tr>
                    <SortableTh label="Scenario" field="name" sortField={sortField} sortDir={sortDir} onSort={handleScenarioSort} />
                    <th scope="col" className="px-3 py-2 font-medium max-sm:hidden">Type</th>
                    <SortableTh label="Runs" field="runCount" sortField={sortField} sortDir={sortDir} onSort={handleScenarioSort} className="text-right" />
                    <th scope="col" className="px-3 py-2 text-right font-medium max-sm:hidden">Share</th>
                  </tr>
                </thead>
                <tbody>
                  {scenarios.map((scenario) => (
                    <tr key={scenario.scenarioSlug} className="border-b border-line/60 last:border-b-0 hover:bg-white/[0.02]">
                      <td className="max-w-[280px] truncate px-3 py-2"><Link className="text-text hover:text-cyan" to={`/scenarios/${scenario.scenarioSlug}`}>{scenario.scenarioName}</Link></td>
                      <td className="px-3 py-2 text-muted max-sm:hidden">{displayScenarioType(scenario.scenarioType) ?? "Other"}</td>
                      <td className="px-3 py-2 text-right tabular-nums">{scenario.runCount.toLocaleString()}</td>
                      <td className="px-3 py-2 text-right tabular-nums text-muted max-sm:hidden">{((Number(scenario.runCount) / totalRuns) * 100).toFixed(1)}%</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : <EmptyState title="No scenarios match." />}
        </Section>

        <Section id="players" title="Most active players" aside={`${players.length.toLocaleString()} shown`}>
          {players.length ? (
            <div className="overflow-x-auto rounded-md border border-line">
              <table className="w-full sm:min-w-[380px] text-left text-sm">
                <thead className="border-b border-line text-xs text-muted">
                  <tr>
                    <th scope="col" className="px-3 py-2 font-medium">Player</th>
                    <th scope="col" className="px-3 py-2 text-right font-medium">Runs</th>
                    <th scope="col" className="px-3 py-2 text-right font-medium">Scenarios</th>
                    <th scope="col" className="px-3 py-2 font-medium max-sm:hidden">Plays most</th>
                  </tr>
                </thead>
                <tbody>
                  {players.map((profile) => (
                    <tr key={profile.userHandle} className="border-b border-line/60 last:border-b-0 hover:bg-white/[0.02]">
                      <td className="max-w-[200px] px-3 py-2">
                        <span className="flex items-center gap-1.5">
                          <Link className="truncate text-text hover:text-cyan" to={`/profiles/${profile.userHandle}`}>{profile.userDisplayName || profile.userHandle}</Link>
                          <VerificationBadge verified={Boolean(profile.isVerified)} />
                        </span>
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">{profile.runCount.toLocaleString()}</td>
                      <td className="px-3 py-2 text-right tabular-nums text-muted">{profile.scenarioCount.toLocaleString()}</td>
                      <td className="px-3 py-2 text-muted max-sm:hidden">{displayScenarioType(profile.primaryScenarioType) ?? "Mixed"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : <EmptyState title="No players match." />}
        </Section>
      </div>
    </PageStack>
  );
}
