import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import type { BenchmarkListItem, GetKovaaksPlayerResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { KovaaksPlayerNotice } from "../components/KovaaksPlayerNotice";
import { RankBadge } from "../components/RankBadge";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { Section } from "../components/ui/Section";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { fetchBenchmarkList, fetchKovaaksPlayer, formatRelativeTime } from "../lib/api";
import { countryFlag, countryName } from "../lib/country";
import { formatScore } from "../lib/kovaaksStats";
import { Helmet } from "../lib/helmet";
import { useUrlState } from "../lib/urlState";

/** /u/kovaaks/:username resolves the username and moves to the Steam id page. */
export function ExternalKovaaksPage() {
  const { kovaaksUsername = "" } = useParams();
  const navigate = useNavigate();
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let cancelled = false;
    void fetchKovaaksPlayer(kovaaksUsername)
      .then((player) => { if (!cancelled) navigate(`/u/${player.steamId}`, { replace: true }); })
      .catch(() => { if (!cancelled) setFailed(true); });
    return () => { cancelled = true; };
  }, [kovaaksUsername, navigate]);
  if (failed) return <PageStack><EmptyState title="No KovaaK's player matches that name." /></PageStack>;
  return <PageStack><PageSkeleton label="Looking up player" /></PageStack>;
}

/** A KovaaK's player who may not use AimMod, from KovaaK's public data. */
export function ExternalProfilePage() {
  const { steamId = "" } = useParams();
  const [player, setPlayer] = useState<GetKovaaksPlayerResponse | null>(null);
  const [catalog, setCatalog] = useState<BenchmarkListItem[]>([]);
  const [error, setError] = useState(false);
  const [state, setState] = useUrlState({ all: "", q: "" });

  useEffect(() => {
    let cancelled = false;
    setPlayer(null);
    setError(false);
    void fetchKovaaksPlayer(steamId).then((next) => { if (!cancelled) setPlayer(next); }).catch(() => { if (!cancelled) setError(true); });
    void fetchBenchmarkList().then((r) => { if (!cancelled) setCatalog(r.benchmarks); }).catch(() => {});
    return () => { cancelled = true; };
  }, [steamId]);

  const hidden = useMemo(() => new Set(catalog.filter((b) => b.hidden).map((b) => b.benchmarkId)), [catalog]);
  const benchmarks = useMemo(() => {
    const list = player?.benchmarks ?? [];
    const q = state.q.trim().toLowerCase();
    return list
      .filter((b) => state.all || !hidden.has(b.benchmarkId))
      .filter((b) => !q || b.benchmarkName.toLowerCase().includes(q) || b.benchmarkAuthor.toLowerCase().includes(q));
  }, [player, hidden, state.all, state.q]);
  const hiddenCount = (player?.benchmarks ?? []).filter((b) => hidden.has(b.benchmarkId)).length;

  if (error) {
    return (
      <PageStack>
        <EmptyState title="No KovaaK's player found." body="Check the Steam id, Steam profile link or KovaaK's username." />
      </PageStack>
    );
  }
  if (!player) return <PageStack><PageSkeleton label="Loading player" /></PageStack>;

  const name = player.kovaaksUsername || player.steamName || steamId;
  const flag = countryFlag(player.country);
  return (
    <PageStack>
      <Helmet><title>{`${name} · KovaaK's player · AimMod Hub`}</title></Helmet>
      <header className="flex flex-wrap items-start gap-4 pt-1">
        {player.avatarUrl ? <img src={player.avatarUrl} alt="" className="h-14 w-14 rounded-md border border-line object-cover" /> : (
          <span aria-hidden className="grid h-14 w-14 place-items-center rounded-md border border-line bg-panel text-xl font-semibold text-muted">{name.slice(0, 1).toUpperCase()}</span>
        )}
        <div className="min-w-0">
          <Breadcrumb crumbs={[{ label: "Players", to: "/community" }, { label: name }]} />
          <h1 className="break-words text-2xl font-semibold leading-tight md:text-3xl">{name}</h1>
          <p className="mt-1 text-sm text-muted">
            {[
              flag ? countryName(player.country) : null,
              player.steamName && player.steamName !== name ? `Steam: ${player.steamName}` : null,
              player.scenariosPlayed ? `${Number(player.scenariosPlayed).toLocaleString()} scenarios played` : null,
              player.lastAccessAtIso ? `seen ${formatRelativeTime(player.lastAccessAtIso)}` : null,
            ].filter(Boolean).join(" · ")}
          </p>
        </div>
      </header>

      <KovaaksPlayerNotice steamId={player.steamId} aimmodHandle={player.aimmodHandle} name={name} />

      <Section title="Benchmarks" aside={hiddenCount ? (
        <button type="button" className="text-cyan hover:underline" onClick={() => setState({ all: state.all ? "" : "1" })}>
          {state.all ? "Hide test and empty benchmarks" : `Show all (${hiddenCount} hidden)`}
        </button>
      ) : null}>
        {(player.benchmarks.length > 6) ? (
          <input type="search" value={state.q} onChange={(e) => setState({ q: e.target.value })} placeholder="Filter benchmarks"
            aria-label="Filter benchmarks" className="mb-3 w-full max-w-sm rounded-md border border-line bg-panel px-3 py-2 text-sm placeholder:text-muted" />
        ) : null}
        {benchmarks.length ? (
          <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {benchmarks.map((b) => (
              <li key={b.benchmarkId}>
                <Link to={`/u/${player.steamId}/benchmarks/${b.benchmarkId}`} className="flex min-h-16 items-center gap-3 rounded-md border border-line bg-panel px-3 py-2.5 transition-colors hover:border-line-strong">
                  {b.benchmarkIconUrl ? <img src={b.benchmarkIconUrl} alt="" className="h-9 w-9 shrink-0 rounded border border-line object-cover" loading="lazy" /> : null}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">{b.benchmarkName}</span>
                    <span className="block truncate text-xs text-muted-2">{b.benchmarkAuthor ? `by ${b.benchmarkAuthor}` : "Benchmark"}</span>
                  </span>
                  <RankBadge rankName={b.overallRank?.rankName} iconUrl={b.overallRank?.iconUrl} color={b.overallRank?.color} />
                </Link>
              </li>
            ))}
          </ul>
        ) : <EmptyState title="No benchmark ranks yet." body={`${name} has no rank on a listed benchmark.`} />}
      </Section>

      <Section title="Most played scenarios" aside={player.scenarioTotal ? `${player.scenarioTotal.toLocaleString()} on KovaaK's leaderboards` : null}>
        {player.scenarios.length ? (
          <div className="overflow-x-auto rounded-md border border-line">
            <table className="w-full text-left text-sm">
              <caption className="sr-only">Scenarios by plays</caption>
              <thead className="border-b border-line text-xs text-muted">
                <tr>
                  <th scope="col" className="px-3 py-2 font-medium">Scenario</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">Plays</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">Score</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">Rank</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium max-sm:hidden">cm/360</th>
                </tr>
              </thead>
              <tbody>
                {player.scenarios.map((s) => (
                  <tr key={s.leaderboardId || s.scenarioName} className="border-b border-line/60 last:border-b-0 hover:bg-white/[0.02]">
                    <td className="max-w-[220px] truncate px-3 py-2 sm:max-w-[360px]">
                      <Link className="hover:text-cyan" to={`/scenarios/${s.scenarioSlug}?tab=kovaaks&name=${encodeURIComponent(s.scenarioName)}&player=${player.steamId}`}>{s.scenarioName}</Link>
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums text-muted">{Number(s.plays).toLocaleString()}</td>
                    <td className="px-3 py-2 text-right font-medium tabular-nums">{formatScore(s.score)}</td>
                    <td className="px-3 py-2 text-right tabular-nums">#{s.rank.toLocaleString()}</td>
                    <td className="px-3 py-2 text-right tabular-nums text-muted max-sm:hidden">{s.cm360 ? s.cm360.toFixed(1) : "–"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : <EmptyState title="No public scenario scores." />}
      </Section>
    </PageStack>
  );
}
