import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import {
  DisputeDecision,
  DisputeStatus,
  MatchState,
  ReplayStatus,
  Resolution,
  VetoAction,
  type Entrant,
  type GetMatchResponse,
} from "../gen/aimmod/tournament/v1/tournament_pb";
import { PageSeo } from "../components/PageSeo";
import { SectionHeader } from "../components/SectionHeader";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { cn } from "../lib/cn";
import { REPLAY_DOWNLOAD, errorMessage, formatWhen, matchStateLabels, resolutionLabels, tournamentClient, vetoLabels } from "../lib/tournaments";

const replayLabels: Record<number, string> = {
  [ReplayStatus.PENDING]: "Checking",
  [ReplayStatus.VERIFIED]: "Verified",
  [ReplayStatus.SUSPICIOUS]: "Needs review",
  [ReplayStatus.REJECTED]: "Rejected",
};

const flagLabels: Record<string, string> = {
  "seed-mismatch": "Played with a different seed than the match's",
  "replay-suspicious": "A replay didn't match the report",
  "replay-rejected": "A replay was rejected",
  tie: "Tied; replayed",
  "needs-review": "Waiting for an organiser to review",
  "no-show-both": "Neither player showed up",
  "auto-confirmed": "Accepted after the confirmation window",
  replayed: "Replayed after a dispute",
  "too-many-ties": "Too many ties; an organiser decides",
};

function name(e?: Entrant) {
  return e?.user?.displayName || e?.user?.handle || "TBD";
}

export function TournamentMatchPage() {
  const { tournamentId = "", matchId = "" } = useParams();
  const [data, setData] = useState<GetMatchResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [reason, setReason] = useState("");
  const [override, setOverride] = useState({ winner: "", winsA: 0, winsB: 0, note: "" });

  const load = useCallback(async () => {
    try {
      // Links use the tournament's slug; the API accepts a slug or an id.
      setData(await tournamentClient.getMatch({ tournamentId, matchId }));
      setError(null);
    } catch (err) {
      setError(errorMessage(err, "Could not load the match."));
    }
  }, [tournamentId, matchId]);

  useEffect(() => {
    setData(null);
    void load();
    const timer = window.setInterval(() => { if (document.visibilityState === "visible") void load(); }, 5000);
    return () => window.clearInterval(timer);
  }, [load]);

  const entrants = useMemo(() => new Map([data?.entrantA, data?.entrantB].filter(Boolean).map((e) => [e!.id, e!])), [data]);

  async function act(run: () => Promise<unknown>, done?: string) {
    setBusy(true);
    setNotice(null);
    try {
      await run();
      if (done) setNotice(done);
      await load();
    } catch (err) {
      setNotice(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  if (error && !data) return <PageStack><PageSection><EmptyState title="Match unavailable" body={error} /></PageSection></PageStack>;
  if (!data?.match || !data.tournament) return <PageStack><PageSection><Skeleton className="mb-3 h-8 w-72" /><Skeleton className="h-48" /></PageSection></PageStack>;

  const t = data.tournament;
  const m = data.match;
  const viewer = data.viewer;
  const tid = t.id;
  const a = data.entrantA, b = data.entrantB;
  const mine = viewer?.entrantId && (viewer.entrantId === a?.id || viewer.entrantId === b?.id);
  const ready = new Set(m.readyEntrantIds);
  const pool = t.ruleset?.pool ?? [];
  const used = new Set(m.veto.map((v) => v.scenario.toLowerCase()));
  const myTurn = mine && m.vetoTurnEntrantId === viewer?.entrantId;
  const canDispute = mine && (m.state === MatchState.LIVE || m.state === MatchState.AWAITING_CONFIRMATION);
  const canConfirm = mine && m.state === MatchState.AWAITING_CONFIRMATION && m.reportedBy !== viewer?.entrantId;
  const disputeOpen = data.dispute && data.dispute.status === DisputeStatus.OPEN;
  const live = data.live;

  return (
    <PageStack>
      <PageSeo title={`${m.label}: ${name(a)} vs ${name(b)} · ${t.name}`} description={`${m.label} of ${t.name}, an AimMod tournament in KovaaK's.`} />
      <PageSection>
        <Breadcrumb crumbs={[{ label: "Tournaments", to: "/tournaments" }, { label: t.name, to: `/tournaments/${encodeURIComponent(t.slug || tid)}` }, { label: m.label }]} />
        <SectionHeader level={1} eyebrow={`${m.label} · best of ${m.bestOf}`} title={`${name(a)} vs ${name(b)}`}
          body={m.state === MatchState.COMPLETE && m.resolution !== Resolution.PLAYED && resolutionLabels[m.resolution] ? `${matchStateLabels[m.state]} · ${resolutionLabels[m.resolution]}` : matchStateLabels[m.state]} />
        <div className="grid max-w-2xl grid-cols-[1fr_auto_1fr] items-center gap-4 rounded-[14px] border border-line bg-white/2 p-4">
          {[a, b].map((e, i) => (
            <div key={i} className={cn("min-w-0", i === 1 && "order-3 text-right")}>
              <div className={cn("truncate text-lg font-semibold", m.winnerEntrantId && m.winnerEntrantId === e?.id ? "text-mint" : "text-text")}>{name(e)}</div>
              <div className="text-[12px] text-muted">
                {e?.seed ? `Seed ${e.seed}` : ""}{e && m.hostEntrantId === e.id ? " · hosts the lobby" : ""}{e && ready.has(e.id) && m.state === MatchState.READY ? " · ready" : ""}
              </div>
            </div>
          ))}
          <div className="order-2 text-center text-3xl font-semibold tabular-nums">{m.winsA}<span className="mx-2 text-muted">–</span>{m.winsB}</div>
        </div>
        {m.deadline && (m.state === MatchState.READY || m.state === MatchState.AWAITING_CONFIRMATION) ? (
          <p className="mt-2 text-[13px] text-muted">{m.state === MatchState.READY ? "No-show deadline" : "Confirm or dispute by"} {formatWhen(m.deadline)}.</p>
        ) : null}
        {m.flags.length > 0 ? <ul className="mt-2 text-[13px] text-gold">{m.flags.map((f) => <li key={f}>{flagLabels[f] ?? f}</li>)}</ul> : null}
        <div className="mt-4 flex flex-wrap items-center gap-2">
          {mine && m.state === MatchState.READY && !ready.has(viewer!.entrantId) ? (
            <Button variant="primary" disabled={busy} onClick={() => void act(() => tournamentClient.markReady({ tournamentId: tid, matchId: m.id, ready: true, canHost: false }), "You're ready. Playing from KovaaK's? Use the Tournaments page in AimMod so it can host the lobby.")}>I'm ready</Button>
          ) : null}
          {canConfirm ? <Button variant="primary" disabled={busy} onClick={() => void act(() => tournamentClient.confirmResult({ tournamentId: tid, matchId: m.id }), "Result confirmed.")}>Confirm result</Button> : null}
          {notice ? <span role="status" className="text-[13px] text-muted">{notice}</span> : null}
        </div>
      </PageSection>

      {live && m.state !== MatchState.COMPLETE ? (
        <PageSection>
          <SectionHeader eyebrow="Live" title={`Game ${live.gameIndex + 1}${live.scenario ? ` · ${live.scenario}` : ""}`} body={`${live.phase}${live.spectators ? ` · ${live.spectators} watching in game` : ""}`} />
          <div className="grid max-w-2xl gap-3 sm:grid-cols-2">
            {live.players.map((p) => (
              <div key={p.entrantId} className="rounded-[12px] border border-line bg-white/2 p-3">
                <div className="font-semibold">{name(entrants.get(p.entrantId))}</div>
                <div className="text-2xl tabular-nums">{Math.round(p.score).toLocaleString()}</div>
                <div className="text-[12px] text-muted">{p.accuracy ? `${p.accuracy.toFixed(1)}% · ` : ""}{Math.ceil(p.secondsLeft)} s left · {p.pingMs} ms · {p.connection}</div>
              </div>
            ))}
          </div>
        </PageSection>
      ) : null}

      {m.state === MatchState.VETO || m.veto.length > 0 ? (
        <PageSection>
          <SectionHeader eyebrow="Picks and bans" title="Veto" body={m.vetoTurnEntrantId ? `${name(entrants.get(m.vetoTurnEntrantId))} ${vetoLabels[m.vetoTurnAction]} next.` : undefined} />
          <ol className="grid max-w-lg gap-1 text-sm">
            {m.veto.map((v) => <li key={v.step}><span className="text-muted-2">{v.step}.</span> {v.action === VetoAction.DECIDER ? "Decider" : name(entrants.get(v.entrantId))} {v.action === VetoAction.DECIDER ? "is" : vetoLabels[v.action]} <strong>{v.scenario}</strong></li>)}
          </ol>
          {myTurn ? (
            <div className="mt-3 flex flex-wrap gap-2">
              {pool.filter((p) => !used.has(p.name.toLowerCase())).map((p) => (
                <Button key={p.name} disabled={busy} onClick={() => void act(() => tournamentClient.submitVeto({ tournamentId: tid, matchId: m.id, scenario: p.name }))}>
                  {m.vetoTurnAction === VetoAction.BAN ? "Ban" : "Pick"} {p.name}
                </Button>
              ))}
            </div>
          ) : null}
        </PageSection>
      ) : null}

      {m.games.length > 0 ? (
        <PageSection>
          <SectionHeader eyebrow="Series" title="Games" body="Both players play each game with the same seed, so the targets appear in the same order for both." />
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px] text-left text-sm">
              <thead className="text-[11px] uppercase tracking-[0.06em] text-muted"><tr>
                <th className="py-2 pr-3">Game</th><th className="py-2 pr-3">Scenario</th><th className="py-2 pr-3">{name(a)}</th><th className="py-2 pr-3">{name(b)}</th><th className="py-2 pr-3">Verification</th>
              </tr></thead>
              <tbody className="divide-y divide-line align-top">
                {m.games.map((g) => (
                  <tr key={g.index}>
                    <td className="py-2 pr-3 tabular-nums">{g.index + 1}</td>
                    <td className="py-2 pr-3">{g.scenario}{g.seed ? <div className="text-[11px] text-muted-2">Seed {g.seed}</div> : null}</td>
                    <td className={cn("py-2 pr-3 tabular-nums", g.winner === 0 && "font-semibold text-mint")}>{g.hasScore ? g.scoreA.toLocaleString() : m.currentGame === g.index ? "Playing" : ""}</td>
                    <td className={cn("py-2 pr-3 tabular-nums", g.winner === 1 && "font-semibold text-mint")}>{g.hasScore ? g.scoreB.toLocaleString() : ""}</td>
                    <td className="py-2 pr-3 text-[12px] text-muted">
                      {g.hostValidated ? <div>Checked by the lobby host</div> : null}
                      {g.replays.map((r) => (
                        <div key={r.id}>
                          {name(entrants.get(r.entrantId))}'s replay: <span className={r.status === ReplayStatus.VERIFIED ? "text-mint" : r.status === ReplayStatus.PENDING ? "" : "text-gold"}>{replayLabels[r.status]}</span>
                          {mine || viewer?.canManage ? <> · <a className="text-cyan hover:underline" href={REPLAY_DOWNLOAD(tid, r.id)}>Download</a></> : null}
                          {r.status !== ReplayStatus.VERIFIED && r.checks.length ? <ul className="ml-3 list-disc">{r.checks.filter((c) => !/matches|later phase|not in this replay/.test(c)).map((c) => <li key={c}>{c}</li>)}</ul> : null}
                        </div>
                      ))}
                      {g.flags.map((f) => <div key={f} className="text-gold">{flagLabels[f] ?? f}</div>)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </PageSection>
      ) : null}

      {canDispute && !disputeOpen ? (
        <PageSection>
          <SectionHeader eyebrow="Something wrong?" title="Dispute the result" body="The match stops until an organiser decides. They see both replays." />
          <div className="flex max-w-xl gap-2">
            <input className="min-h-10 flex-1 rounded-md border border-line bg-bg-2 px-3 text-sm" value={reason} onChange={(e) => setReason(e.target.value)} aria-label="What went wrong" maxLength={500} />
            <Button disabled={busy || reason.trim().length < 5} onClick={() => void act(() => tournamentClient.openDispute({ tournamentId: tid, matchId: m.id, reason }), "Dispute sent to the organisers.")}>Send dispute</Button>
          </div>
        </PageSection>
      ) : null}

      {data.dispute ? (
        <PageSection>
          <SectionHeader eyebrow={disputeOpen ? "Open dispute" : "Dispute"} title={`Raised by ${name(entrants.get(data.dispute.openedByEntrantId))}`} body={data.dispute.reason} />
          {!disputeOpen && data.dispute.resolutionNote ? <p className="text-sm text-muted">Decision: {data.dispute.resolutionNote}</p> : null}
          {disputeOpen && viewer?.canManage ? (
            <div className="flex flex-wrap gap-2">
              <Button disabled={busy || m.state !== MatchState.DISPUTED} onClick={() => void act(() => tournamentClient.resolveDispute({ tournamentId: tid, disputeId: data.dispute!.id, decision: DisputeDecision.UPHOLD, note: "The reported result stands." }), "Result upheld.")}>Uphold the report</Button>
              <Button disabled={busy} onClick={() => { if (window.confirm("Clear the games and replay this match?")) void act(() => tournamentClient.resolveDispute({ tournamentId: tid, disputeId: data.dispute!.id, decision: DisputeDecision.REPLAY, note: "Replayed." }), "The match will be replayed."); }}>Replay the match</Button>
            </div>
          ) : null}
        </PageSection>
      ) : null}

      {viewer?.canManage && a && b ? (
        <PageSection className="rounded-[14px] border border-line bg-white/2 px-4">
          <SectionHeader eyebrow="Organiser" title="Set the result" body="Overrides close any open dispute. Changing a finished match's winner resets the later matches that depended on it." />
          <div className="grid max-w-2xl gap-3 sm:grid-cols-4">
            <select className="min-h-10 rounded-md border border-line bg-bg-2 px-2 text-sm sm:col-span-2" value={override.winner} onChange={(e) => setOverride({ ...override, winner: e.target.value })} aria-label="Winner">
              <option value="">Winner...</option><option value={a.id}>{name(a)}</option><option value={b.id}>{name(b)}</option>
            </select>
            <input className="min-h-10 rounded-md border border-line bg-bg-2 px-3 text-sm" type="number" min={0} max={9} value={override.winsA} onChange={(e) => setOverride({ ...override, winsA: Number(e.target.value) })} aria-label={`${name(a)} games`} />
            <input className="min-h-10 rounded-md border border-line bg-bg-2 px-3 text-sm" type="number" min={0} max={9} value={override.winsB} onChange={(e) => setOverride({ ...override, winsB: Number(e.target.value) })} aria-label={`${name(b)} games`} />
            <input className="min-h-10 rounded-md border border-line bg-bg-2 px-3 text-sm sm:col-span-4" value={override.note} onChange={(e) => setOverride({ ...override, note: e.target.value })} aria-label="Reason" maxLength={500} />
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button disabled={busy || !override.winner} onClick={() => void act(() => disputeOpen
              ? tournamentClient.resolveDispute({ tournamentId: tid, disputeId: data.dispute!.id, decision: DisputeDecision.OVERTURN, winnerEntrantId: override.winner, winsA: override.winsA, winsB: override.winsB, note: override.note })
              : tournamentClient.setMatchResult({ tournamentId: tid, matchId: m.id, winnerEntrantId: override.winner, winsA: override.winsA, winsB: override.winsB, resolution: Resolution.ADMIN_OVERRIDE, reason: override.note }), "Result set.")}>Set result</Button>
            {m.state !== MatchState.COMPLETE ? (
              <Button disabled={busy || !override.winner} onClick={() => void act(() => tournamentClient.setMatchResult({ tournamentId: tid, matchId: m.id, winnerEntrantId: override.winner, resolution: Resolution.NO_SHOW, reason: override.note }), "Recorded as a no-show.")}>Award as no-show</Button>
            ) : null}
          </div>
        </PageSection>
      ) : null}
    </PageStack>
  );
}
