import { snapshotStatus } from "../lib/snapshotStatus";
import { Code, ConnectError } from "@connectrpc/connect";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import type { BenchmarkLeaderboardEntry, BenchmarkListItem } from "../gen/aimmod/hub/v1/hub_pb";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageHeader } from "../components/ui/PageHeader";
import { Button } from "../components/ui/Button";
import { Helmet } from "../lib/helmet";
import { PageStack } from "../components/ui/Stack";
import { fetchBenchmarkLeaderboard, fetchBenchmarkList, fetchKovaaksPlayer } from "../lib/api";
import { RankBadge } from "../components/RankBadge";
import { Pager } from "../components/ui/Pager";
import { useAuth } from "../lib/AuthContext";
import { cn } from "../lib/cn";
import { intParam, useUrlState } from "../lib/urlState";

/** Looks up any KovaaK's player and opens their sheet for this benchmark. */
function KovaaksLookup({ benchmarkId }: { benchmarkId: string }) {
  const navigate = useNavigate();
  const [value, setValue] = useState("");
  const [state, setState] = useState<"idle" | "busy" | "missing" | "unreachable">("idle");
  return (
    <form className="flex flex-wrap items-center gap-2" onSubmit={(e) => {
      e.preventDefault();
      const q = value.trim();
      if (!q) return;
      setState("busy");
      void fetchKovaaksPlayer(q).then((p) => navigate(`/u/${p.steamId}/benchmarks/${benchmarkId}`)).catch((err) => setState(ConnectError.from(err).code === Code.NotFound ? "missing" : "unreachable"));
    }}>
      <label htmlFor="kovaaks-lookup" className="text-sm text-muted">Any KovaaK's player</label>
      <input id="kovaaks-lookup" value={value} onChange={(e) => { setValue(e.target.value); setState("idle"); }} placeholder="KovaaK's name, Steam id or profile link"
        className="min-h-9 w-full rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted sm:w-72" />
      <Button type="submit" disabled={state === "busy"}>{state === "busy" ? "Looking up" : "Open sheet"}</Button>
      {state === "unreachable" ? <span role="alert">KovaaK's is unreachable. Try again.</span> : null}
      {state === "missing" ? <span role="status" className="text-sm text-muted">No KovaaK's player matches that.</span> : null}
    </form>
  );
}
import { groupBenchmarks, extractDifficulty } from "../lib/benchmarkGroups";

export function BenchmarkLeaderboardPage() {
  const { benchmarkId = "" } = useParams();
  const navigate = useNavigate();
  const [entries, setEntries] = useState<BenchmarkLeaderboardEntry[] | null>(null);
  const [snapshot, setSnapshot] = useState({ updated: "", partial: false, pending: false, total: 0, page: 1 });
  const [refresh, setRefresh] = useState(0);
  const [benchmarkName, setBenchmarkName] = useState("");
  const [allBenchmarks, setAllBenchmarks] = useState<BenchmarkListItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const auth = useAuth();
  const me = auth.user?.profileHandle?.toLowerCase() ?? "";
  const [state, setState] = useUrlState({ q: "", page: "1", size: "50" }, { size: ["25", "50", "100"] });

  // Load leaderboard and benchmark list in parallel
  useEffect(() => {
    if (!benchmarkId) return;
    let cancelled = false;
    setEntries(null);
    setError(null);

    const leaderboardP = fetchBenchmarkLeaderboard(Number(benchmarkId), intParam(state.page, 1, 1), intParam(state.size, 50, 10, 100), state.q)
      .then((res) => {
        if (!cancelled) {
          setEntries(res.entries ?? []);
          setSnapshot(res);
          setBenchmarkName(res.benchmarkName || `Benchmark #${benchmarkId}`);
        }
      });

    const listP = fetchBenchmarkList()
      .then((res) => { if (!cancelled) setAllBenchmarks(res.benchmarks ?? []); })
      .catch(() => { /* sibling tabs are optional */ });

    Promise.all([leaderboardP, listP]).catch((err) => {
      if (!cancelled) setError(err instanceof Error ? err.message : "Could not load leaderboard.");
    });

    return () => { cancelled = true; };
  }, [benchmarkId, state.page, state.size, state.q, refresh]);
  useEffect(() => { if (!snapshot.pending) return; const timer = setTimeout(() => setRefresh((v) => v + 1), 5000); return () => clearTimeout(timer); }, [snapshot.pending, refresh]);

  // Find sibling benchmarks in the same series
  const siblings = useMemo(() => {
    if (!allBenchmarks || !benchmarkName) return null;
    const groups = groupBenchmarks(allBenchmarks);
    const group = groups.find((g) =>
      g.variants.some((v) => String(v.item.benchmarkId) === benchmarkId)
    );
    return group && group.variants.length > 1 ? group : null;
  }, [allBenchmarks, benchmarkName, benchmarkId]);

  const pageSize = intParam(state.size, 50, 10, 100);
  const page = snapshot.page - 1;
  const ranked = (entries ?? []).map((entry, i) => ({ entry, position: page * pageSize + i + 1 }));
  const visible = ranked;

  if (error) {
    return (
      <PageStack>
        <PageHeader title="Benchmark" />
        <EmptyState title="This benchmark leaderboard could not be loaded.">
          <Button onClick={() => setRefresh((v) => v + 1)}>Try again</Button><Button to="/benchmarks">All benchmarks</Button>
        </EmptyState>
      </PageStack>
    );
  }

  if (!entries) {
    return <PageStack><PageSkeleton stats={0} rows={8} label="Loading benchmark leaderboard" /></PageStack>;
  }

  const { difficulty: currentDifficulty } = extractDifficulty(benchmarkName);
  const title = siblings ? siblings.base : (benchmarkName || `Benchmark #${benchmarkId}`);
  // How the ranked players spread across ranks, highest rank first.
  const rankSpread = [...entries.reduce((map, e) => {
    const key = e.overallRankName || "Unranked";
    const current = map.get(key) ?? { name: key, icon: e.overallRankIconUrl, index: e.overallRankIndex, count: 0 };
    current.count += 1;
    return map.set(key, current);
  }, new Map<string, { name: string; icon: string; index: number; count: number }>()).values()].sort((a, b) => b.index - a.index);

  return (
    <PageStack>
      <Helmet><title>{`${title} · Benchmark · AimMod Hub`}</title></Helmet>
      <PageSection>
        <PageHeader
          before={<Breadcrumb crumbs={[{ label: "Benchmarks", to: "/benchmarks" }, { label: title }]} />}
          title={currentDifficulty && siblings ? `${title} · ${currentDifficulty}` : title}
          meta={`${snapshot.total.toLocaleString()} ranked AimMod ${entries.length === 1 ? "player" : "players"} · ordered by overall rank, then KovaaK's progress`}
        />
        <p role="status" className="mt-3 text-sm text-muted">{snapshotStatus(snapshot.updated, snapshot.pending)}</p>
        {snapshot.partial && !snapshot.pending ? <p role="status">Some players couldn't be refreshed.</p> : null}
        <div className="mt-4"><KovaaksLookup benchmarkId={benchmarkId} /></div>
        <input type="search" value={state.q} onChange={(e) => setState({ q: e.target.value, page: "1" })} placeholder="Find a player" aria-label="Find a player"
          className="mt-4 min-h-9 w-full rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted sm:w-56" />
        {/* Difficulty tabs — only shown when the benchmark has variants */}
        {siblings && (
          <div className="mt-4 flex flex-wrap gap-2">
            {siblings.variants.map(({ item, difficulty }) => {
              const isActive = String(item.benchmarkId) === benchmarkId;
              const label = difficulty ?? item.benchmarkName;
              return (
                <button
                  key={item.benchmarkId}
                  onClick={() => navigate(`/benchmarks/${item.benchmarkId}`)}
                  className={[
                    "rounded-full border px-4 py-1.5 text-[11px] font-medium uppercase tracking-[0.06em] transition-colors",
                    isActive
                      ? "border-cyan/40 bg-cyan/10 text-cyan"
                      : "border-line text-muted-2 hover:border-line/80 hover:text-text",
                  ].join(" ")}
                >
                  {label}
                  {item.playerCount > 0 && (
                    <span className={`ml-1.5 text-[11px] ${isActive ? "text-cyan/70" : "text-muted-2"}`}>
                      {item.playerCount}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        )}
      </PageSection>

      {entries.length === 0 ? (
        <PageSection>
          <EmptyState title="No AimMod players are ranked here yet." body="Ranks appear once AimMod has seen a player signed in to Steam in KovaaK's. You can still open any KovaaK's player's sheet above.">
            <Button to="/app/kovaaks">Get AimMod for KovaaK's</Button>
          </EmptyState>
        </PageSection>
      ) : (
        <PageSection className="grid gap-4">
          {rankSpread.length > 1 && (
            <ul className="flex flex-wrap gap-2" aria-label="Players per rank">
              {rankSpread.map((rank) => (
                <li key={rank.name} className="flex items-center gap-2 rounded-md border border-line bg-panel px-3 py-1.5 text-sm">
                  {rank.icon && <img src={rank.icon} alt="" className="h-5 w-5 rounded object-cover" />}
                  <span>{rank.name}</span>
                  <span className="tabular-nums text-muted">{rank.count}</span>
                </li>
              ))}
            </ul>
          )}
          <div className="flex flex-wrap items-center gap-2">
            {me !== "" ? (
              <button type="button" onClick={() => setState({ q: me, page: "1" })}
                className="ml-auto inline-flex min-h-9 items-center rounded-md border border-mint/50 bg-mint/10 px-3 text-sm font-medium hover:bg-mint/20">Find me</button>
            ) : null}
          </div>
          <div className="overflow-x-auto rounded-md border border-line">
            <table className="w-full border-collapse text-left text-sm">
              <caption className="sr-only">AimMod players ranked on this benchmark</caption>
              <thead className="border-b border-line text-xs text-muted">
                <tr>
                  <th scope="col" className="w-12 px-3 py-2 font-medium">#</th>
                  <th scope="col" className="px-3 py-2 font-medium">Player</th>
                  <th scope="col" className="px-3 py-2 font-medium">Rank</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium max-sm:hidden">KovaaK's progress</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium"><span className="sr-only">Sheet</span></th>
                </tr>
              </thead>
              <tbody>
                {visible.map(({ entry, position }) => {
                  const mine = me !== "" && entry.userHandle.toLowerCase() === me;
                  return (
                    <tr key={entry.userHandle} className={cn("border-b border-line/60 last:border-0", mine ? "bg-mint/10" : "hover:bg-white/[0.02]")} aria-current={mine ? "true" : undefined}>
                      <td className={cn("px-3 py-2 tabular-nums", position <= 3 ? "text-gold" : "text-muted-2")}>{position}</td>
                      <td className="max-w-[220px] px-3 py-2 sm:max-w-none">
                        <Link to={`/profiles/${entry.userHandle}`} className="flex min-w-0 items-center gap-2 hover:text-cyan">
                          {entry.avatarUrl ? <img src={entry.avatarUrl} alt="" className="h-6 w-6 shrink-0 rounded-full border border-line object-cover" /> : null}
                          <span className="truncate font-medium">{entry.displayName || entry.userHandle}</span>
                          <span className="shrink-0 text-xs text-muted-2 max-sm:hidden">@{entry.userHandle}</span>
                        </Link>
                      </td>
                      <td className="px-3 py-2"><RankBadge rankName={entry.overallRankName} iconUrl={entry.overallRankIconUrl} color={entry.overallRankColor} rankIndex={entry.overallRankIndex} /></td>
                      <td className="px-3 py-2 text-right tabular-nums text-muted max-sm:hidden">{entry.benchmarkProgress ? Math.round(entry.benchmarkProgress).toLocaleString() : "–"}</td>
                      <td className="px-3 py-2 text-right">
                        <Link to={`/profiles/${entry.userHandle}/benchmarks/${benchmarkId}`} className="text-xs text-cyan hover:underline">Sheet</Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {snapshot.total > pageSize ? (
            <Pager page={page} pageSize={pageSize} total={snapshot.total} onPage={(p) => setState({ page: String(p + 1) })}
              pageSizes={[25, 50, 100]} onPageSize={(size) => setState({ size: String(size), page: "1" })} />
          ) : null}
        </PageSection>
      )}
    </PageStack>
  );
}
