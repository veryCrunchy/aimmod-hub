import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { MatchState, type GetOverviewResponse, type LivePlayer } from "../gen/aimmod/tournament/v1/tournament_pb";
import { PageSeo } from "../components/PageSeo";
import { SectionHeader } from "../components/SectionHeader";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { cn } from "../lib/cn";
import { byId, entrantName, errorMessage, matchStateLabels, tournamentClient } from "../lib/tournaments";

// Every running match at once: a card per player with live score, accuracy,
// time left, ping and connection, as reported by each match's lobby host.
export function TournamentOverviewPage() {
  const { tournamentId = "" } = useParams();
  const [data, setData] = useState<GetOverviewResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setData(await tournamentClient.getOverview({ tournamentId }));
      setError(null);
    } catch (err) {
      setError(errorMessage(err, "Could not load the overview."));
    }
  }, [tournamentId]);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => { if (document.visibilityState === "visible") void load(); }, 3000);
    return () => window.clearInterval(timer);
  }, [load]);

  const entrants = useMemo(() => byId(data?.entrants ?? []), [data]);
  const live = useMemo(() => new Map((data?.live ?? []).map((l) => [l.matchId, l])), [data]);

  if (error && !data) return <PageStack><PageSection><EmptyState title="Overview unavailable" body={error} /></PageSection></PageStack>;
  if (!data?.tournament) return <PageStack><PageSection><Skeleton className="h-64" /></PageSection></PageStack>;
  const t = data.tournament;

  return (
    <PageStack>
      <PageSeo title={`Live: ${t.name} · AimMod Hub`} description={`Every running match of ${t.name} at once.`} noindex />
      <PageSection>
        <Breadcrumb crumbs={[{ label: "Tournaments", to: "/tournaments" }, { label: t.name, to: `/tournaments/${encodeURIComponent(t.slug || t.id)}` }, { label: "Live overview" }]} />
        <SectionHeader level={1} eyebrow="Live overview" title={t.name}
          body="Updates every few seconds from each match's lobby host. Spectate a match in KovaaK's from AimMod's Tournaments page; casters can add AimMod's tournament overlay to OBS." />
      </PageSection>
      <PageSection>
        {data.active.length === 0 ? <EmptyState title="No matches running" body="Matches show up here as soon as both players are ready." /> : (
          <div className="grid gap-4 lg:grid-cols-2">
            {data.active.map((m) => {
              const l = live.get(m.id);
              const players: (LivePlayer | undefined)[] = [m.slotA?.entrantId, m.slotB?.entrantId].map((id) => l?.players.find((p) => p.entrantId === id));
              const leader = l && l.players.length === 2 ? (l.players[0].score >= l.players[1].score ? l.players[0].entrantId : l.players[1].entrantId) : "";
              return (
                <div key={m.id} className="rounded-[14px] border border-line bg-white/2 p-3">
                  <div className="mb-2 flex items-center justify-between gap-2 text-[12px] text-muted">
                    <Link className="font-semibold text-text hover:text-cyan" to={`/tournaments/${encodeURIComponent(t.slug || t.id)}/matches/${m.id}`}>{m.label}</Link>
                    <span>{l ? `Game ${l.gameIndex + 1} · ${l.scenario || "lobby"} · ${l.phase}` : matchStateLabels[m.state]} · {m.winsA}–{m.winsB}</span>
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    {[m.slotA?.entrantId, m.slotB?.entrantId].map((id, i) => {
                      const p = players[i];
                      const stale = !p;
                      return (
                        <div key={id ?? i} className={cn("rounded-[10px] border p-2", id && id === leader ? "border-mint/50" : "border-line")}>
                          <div className="truncate text-sm font-semibold">{entrantName(entrants, id)}</div>
                          <div className="text-2xl tabular-nums">{p ? Math.round(p.score).toLocaleString() : "–"}</div>
                          <div className="text-[11px] text-muted">
                            {p ? `${p.accuracy ? p.accuracy.toFixed(1) + "% · " : ""}${Math.max(0, Math.ceil(p.secondsLeft))} s left` : m.state === MatchState.READY ? (m.readyEntrantIds.includes(id ?? "") ? "Ready" : "Not ready") : "No live data"}
                          </div>
                          <div className={cn("text-[11px]", p?.connection === "connected" ? "text-mint" : stale ? "text-muted-2" : "text-gold")}>
                            {p ? `${p.connection}${p.pingMs ? ` · ${p.pingMs} ms` : ""} · ${p.status}` : ""}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </PageSection>
    </PageStack>
  );
}
