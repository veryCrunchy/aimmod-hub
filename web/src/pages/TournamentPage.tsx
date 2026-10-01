import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  AdvanceAction,
  DisputeStatus,
  EntrantStatus,
  MatchState,
  SeedingMethod,
  StaffRole,
  TournamentFormat,
  TournamentStatus,
  type GetTournamentResponse,
} from "../gen/aimmod/tournament/v1/tournament_pb";
import { PageSeo } from "../components/PageSeo";
import { SectionHeader } from "../components/SectionHeader";
import { TournamentBracket } from "../components/TournamentBracket";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { useAuth } from "../lib/AuthContext";
import { discordStartUrl } from "../lib/auth";
import {
  byId,
  entrantName,
  entrantStatusLabels,
  errorMessage,
  formatLabels,
  formatWhen,
  isActive,
  seedingLabels,
  statusLabels,
  tournamentClient,
} from "../lib/tournaments";

const roleLabels: Record<number, string> = { [StaffRole.ORGANISER]: "Organiser", [StaffRole.ADMIN]: "Admin", [StaffRole.CASTER]: "Caster" };

export function TournamentPage() {
  const { tournamentId = "" } = useParams();
  const auth = useAuth();
  const [data, setData] = useState<GetTournamentResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setData(await tournamentClient.getTournament({ tournament: tournamentId }));
      setError(null);
    } catch (err) {
      setError(errorMessage(err, "Could not load the tournament."));
    }
  }, [tournamentId]);

  useEffect(() => {
    setData(null);
    void load();
    // Live events refresh on their own.
    const timer = window.setInterval(() => { if (document.visibilityState === "visible") void load(); }, 15000);
    return () => window.clearInterval(timer);
  }, [load, auth.authenticated]);

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

  const entrants = useMemo(() => byId(data?.entrants ?? []), [data]);

  if (error && !data) {
    return <PageStack><PageSection><EmptyState title="Tournament unavailable" body={error} /></PageSection></PageStack>;
  }
  if (!data?.tournament) {
    return <PageStack><PageSection><Skeleton className="mb-3 h-8 w-72" /><Skeleton className="h-64" /></PageSection></PageStack>;
  }
  const t = data.tournament;
  const viewer = data.viewer;
  const id = t.id;
  const active = data.entrants.filter((e) => ![EntrantStatus.WITHDRAWN, EntrantStatus.NO_SHOW].includes(e.status));
  const seeded = [...active].sort((a, b) => (a.seed || 9999) - (b.seed || 9999) || a.registeredAt.localeCompare(b.registeredAt));
  const live = data.matches.filter(isActive);
  const self = viewer?.entrantId ? entrants.get(viewer.entrantId) : undefined;
  const preStart = t.status === TournamentStatus.DRAFT || t.status === TournamentStatus.REGISTRATION || t.status === TournamentStatus.CHECK_IN;
  const table = t.format === TournamentFormat.ROUND_ROBIN || t.format === TournamentFormat.SWISS;

  return (
    <PageStack>
      <PageSeo title={`${t.name} · AimMod tournament`} description={`${formatLabels[t.format]} KovaaK's tournament on AimMod Hub. ${statusLabels[t.status]}.`} noindex={t.status === TournamentStatus.DRAFT} />
      <PageSection>
        <Breadcrumb crumbs={[{ label: "Tournaments", to: "/tournaments" }, { label: t.name }]} />
        <SectionHeader
          level={1}
          eyebrow={statusLabels[t.status]}
          title={t.name}
          body={`${formatLabels[t.format]} · best of ${t.ruleset?.bestOf ?? 1} · ${t.entrantCount}/${t.maxEntrants} players · ${seedingLabels[t.seeding]} seeding${t.organiser ? ` · run by ${t.organiser.displayName || t.organiser.handle}` : ""}`}
          aside={<div className="flex flex-wrap gap-2">
            {t.status === TournamentStatus.IN_PROGRESS ? <Button to={`/tournaments/${encodeURIComponent(t.slug || id)}/overview`}>Live overview</Button> : null}
          </div>}
        />
        {t.description ? <p className="max-w-[72ch] whitespace-pre-line text-sm leading-6 text-muted">{t.description}</p> : null}
        <dl className="mt-3 grid gap-x-6 gap-y-1 text-[13px] text-muted sm:grid-cols-2 lg:grid-cols-3">
          {t.startsAt ? <div><dt className="inline text-muted-2">Starts </dt><dd className="inline">{formatWhen(t.startsAt)}</dd></div> : null}
          {t.checkInOpensAt ? <div><dt className="inline text-muted-2">Check-in </dt><dd className="inline">{formatWhen(t.checkInOpensAt)} to {formatWhen(t.checkInClosesAt)}</dd></div> : null}
          <div><dt className="inline text-muted-2">Pool </dt><dd className="inline">{t.ruleset?.pool.map((p) => p.name).join(", ")}</dd></div>
          {t.prize ? <div><dt className="inline text-muted-2">Prize </dt><dd className="inline">{t.prize}</dd></div> : null}
          {t.championEntrantId ? <div><dt className="inline text-muted-2">Champion </dt><dd className="inline font-semibold text-mint">{entrantName(entrants, t.championEntrantId)}</dd></div> : null}
        </dl>
        <div className="mt-4 flex flex-wrap items-center gap-2">
          {!auth.authenticated && preStart ? <Button href={discordStartUrl(`/tournaments/${t.slug || id}`)} variant="primary">Sign in to enter</Button> : null}
          {viewer?.canRegister ? <Button variant="primary" disabled={busy} onClick={() => void act(() => tournamentClient.register({ tournamentId: id }), "You're in.")}>Enter tournament</Button> : null}
          {viewer?.canCheckIn ? <Button variant="primary" disabled={busy} onClick={() => void act(() => tournamentClient.checkIn({ tournamentId: id }), "Checked in. Good luck!")}>Check in</Button> : null}
          {self && self.checkedInAt ? <span className="text-[13px] text-mint">You're checked in.</span> : null}
          {self && (preStart || t.status === TournamentStatus.IN_PROGRESS) ? (
            <Button disabled={busy} onClick={() => { if (window.confirm(t.status === TournamentStatus.IN_PROGRESS ? "Withdraw? Your remaining matches will be forfeited." : "Leave this tournament?")) void act(() => tournamentClient.withdraw({ tournamentId: id }), "You've withdrawn."); }}>Withdraw</Button>
          ) : null}
          {notice ? <span role="status" className="text-[13px] text-muted">{notice}</span> : null}
        </div>
        {self && t.status === TournamentStatus.IN_PROGRESS ? (
          <p className="mt-3 text-[13px] text-muted">Your matches appear in AimMod's Tournaments page in KovaaK's. The lobby is created for you when you and your opponent are both ready.</p>
        ) : null}
      </PageSection>

      {viewer?.canManage ? <OrganiserPanel data={data} busy={busy} act={act} /> : null}

      {live.length > 0 ? (
        <PageSection>
          <SectionHeader eyebrow="Now" title="Live matches" />
          <ul className="divide-y divide-line border-y border-line text-sm">
            {live.map((m) => (
              <li key={m.id}><Link className="flex justify-between gap-3 px-1 py-2 hover:bg-white/2" to={`/tournaments/${encodeURIComponent(t.slug || id)}/matches/${m.id}`}>
                <span>{m.label}: {entrantName(entrants, m.slotA?.entrantId)} vs {entrantName(entrants, m.slotB?.entrantId)}</span>
                <span className="text-muted">{m.winsA}–{m.winsB}{m.state === MatchState.DISPUTED ? " · disputed" : ""}</span>
              </Link></li>
            ))}
          </ul>
        </PageSection>
      ) : null}

      {data.matches.length > 0 ? (
        <PageSection>
          <SectionHeader eyebrow="Bracket" title={table ? "Rounds" : "Bracket"} />
          <TournamentBracket matches={data.matches} entrants={entrants} tournamentId={t.slug || id} highlight={viewer?.entrantId} />
        </PageSection>
      ) : null}

      {data.standings.length > 0 ? (
        <PageSection>
          <SectionHeader eyebrow="Results" title="Standings" />
          <div className="overflow-x-auto">
            <table className="w-full min-w-[420px] text-left text-sm">
              <thead className="text-[11px] uppercase tracking-[0.06em] text-muted"><tr>
                <th className="py-2 pr-3">Place</th><th className="py-2 pr-3">Player</th><th className="py-2 pr-3">W–L</th><th className="py-2 pr-3">Games</th>
                {t.format === TournamentFormat.SWISS ? <th className="py-2 pr-3">Buchholz</th> : null}
              </tr></thead>
              <tbody className="divide-y divide-line">
                {data.standings.map((s) => (
                  <tr key={s.entrantId} className={s.eliminated ? "text-muted" : ""}>
                    <td className="py-2 pr-3 tabular-nums">{s.place}</td>
                    <td className="py-2 pr-3">{entrantName(entrants, s.entrantId)}</td>
                    <td className="py-2 pr-3 tabular-nums">{s.wins}–{s.losses}</td>
                    <td className="py-2 pr-3 tabular-nums">{s.gamesWon}–{s.gamesLost}</td>
                    {t.format === TournamentFormat.SWISS ? <td className="py-2 pr-3 tabular-nums">{s.buchholz}</td> : null}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </PageSection>
      ) : null}

      <PageSection>
        <SectionHeader eyebrow="Field" title={`Players (${active.length})`} />
        {seeded.length === 0 ? <EmptyState title="No players yet" body={preStart ? "Be the first to enter." : "Nobody played in this tournament."} /> : (
          <ul className="grid gap-x-6 sm:grid-cols-2 lg:grid-cols-3">
            {seeded.map((e) => (
              <li key={e.id} className="flex items-center justify-between gap-3 border-b border-line py-2 text-sm">
                <span className="flex min-w-0 items-center gap-2">
                  <span className="w-6 text-right text-[11px] text-muted-2">{e.seed || ""}</span>
                  {e.user?.handle ? <Link className="truncate hover:text-cyan" to={`/profiles/${encodeURIComponent(e.user.handle)}`}>{e.user.displayName || e.user.handle}</Link> : <span className="truncate">Player</span>}
                </span>
                <span className="shrink-0 text-[12px] text-muted">{e.ratingLabel ? `${e.ratingLabel} · ` : ""}{entrantStatusLabels[e.status]}</span>
              </li>
            ))}
          </ul>
        )}
      </PageSection>

      {data.staff.length > 1 ? (
        <PageSection>
          <SectionHeader eyebrow="Staff" title="Run by" />
          <ul className="flex flex-wrap gap-3 text-sm">{data.staff.map((s) => <li key={s.user?.userExternalId + String(s.role)}>{s.user?.displayName || s.user?.handle} <span className="text-muted">({roleLabels[s.role]})</span></li>)}</ul>
        </PageSection>
      ) : null}
    </PageStack>
  );
}

function OrganiserPanel({ data, busy, act }: { data: GetTournamentResponse; busy: boolean; act: (run: () => Promise<unknown>, done?: string) => Promise<void> }) {
  const t = data.tournament!;
  const id = t.id;
  const entrants = byId(data.entrants);
  const [invites, setInvites] = useState("");
  const [staffHandle, setStaffHandle] = useState("");
  const [staffRole, setStaffRole] = useState(StaffRole.ADMIN);
  const [order, setOrder] = useState<string[]>([]);
  const field = data.entrants.filter((e) => e.status !== EntrantStatus.WITHDRAWN && e.status !== EntrantStatus.NO_SHOW && e.status !== EntrantStatus.DISQUALIFIED);
  const preStart = t.status === TournamentStatus.DRAFT || t.status === TournamentStatus.REGISTRATION || t.status === TournamentStatus.CHECK_IN;

  useEffect(() => {
    setOrder([...field].sort((a, b) => (a.seed || 9999) - (b.seed || 9999) || a.registeredAt.localeCompare(b.registeredAt)).map((e) => e.id));
  }, [data]);

  const advance = (action: AdvanceAction, done: string, confirm?: string) => {
    if (confirm && !window.confirm(confirm)) return;
    void act(() => tournamentClient.advanceTournament({ tournamentId: id, action }), done);
  };
  const move = (i: number, d: number) => {
    const next = [...order];
    const j = i + d;
    if (j < 0 || j >= next.length) return;
    [next[i], next[j]] = [next[j], next[i]];
    setOrder(next);
  };
  const openDisputes = data.disputes.filter((d) => d.status === DisputeStatus.OPEN);
  const review = data.matches.filter((m) => m.flags.includes("needs-review") || m.flags.includes("no-show-both"));

  return (
    <PageSection className="rounded-[14px] border border-line bg-white/2 px-4">
      <SectionHeader eyebrow="Organiser" title="Run the tournament" body="Only you and your staff see this panel." />
      <div className="flex flex-wrap gap-2">
        {t.status === TournamentStatus.DRAFT ? <Button variant="primary" disabled={busy} onClick={() => advance(AdvanceAction.OPEN_REGISTRATION, "Registration is open.")}>Open registration</Button> : null}
        {t.status === TournamentStatus.REGISTRATION && t.checkInOpensAt ? <Button disabled={busy} onClick={() => advance(AdvanceAction.OPEN_CHECK_IN, "Check-in is open.")}>Open check-in now</Button> : null}
        {t.status === TournamentStatus.REGISTRATION || t.status === TournamentStatus.CHECK_IN ? (
          <Button variant="primary" disabled={busy} onClick={() => advance(AdvanceAction.START, "The bracket is out.", t.checkInOpensAt ? "Start now? Players who haven't checked in are removed." : "Start now and generate the bracket?")}>Start tournament</Button>
        ) : null}
        {t.status !== TournamentStatus.COMPLETED && t.status !== TournamentStatus.CANCELLED ? (
          <Button disabled={busy} onClick={() => advance(AdvanceAction.CANCEL, "Cancelled.", "Cancel this tournament for everyone?")}>Cancel tournament</Button>
        ) : null}
      </div>

      {openDisputes.length > 0 || review.length > 0 ? (
        <div className="mt-4">
          <h3 className="mb-1 text-sm font-semibold text-gold">Needs you</h3>
          <ul className="text-sm">
            {openDisputes.map((d) => <li key={d.id}><Link className="text-cyan" to={`/tournaments/${encodeURIComponent(t.slug || id)}/matches/${d.matchId}`}>Dispute on {d.matchId}</Link>: {d.reason}</li>)}
            {review.map((m) => <li key={m.id}><Link className="text-cyan" to={`/tournaments/${encodeURIComponent(t.slug || id)}/matches/${m.id}`}>{m.label}</Link>: {m.flags.includes("no-show-both") ? "neither player showed up" : "a result needs review"}</li>)}
          </ul>
        </div>
      ) : null}

      {(preStart || t.status === TournamentStatus.IN_PROGRESS) && field.length > 1 ? (
        <details className="mt-4">
          <summary className="cursor-pointer text-sm font-semibold">Seeds</summary>
          <p className="my-2 text-[12px] text-muted">Seeds can change until the first match starts. Saving switches seeding to manual.</p>
          <ol className="grid max-w-md gap-1 text-sm">
            {order.map((eid, i) => (
              <li key={eid} className="flex items-center justify-between gap-2 border-b border-line py-1">
                <span><span className="mr-2 text-muted-2">{i + 1}</span>{entrantName(entrants, eid)}</span>
                <span className="flex gap-1">
                  <button type="button" className="px-2 text-muted hover:text-text" aria-label="Move up" onClick={() => move(i, -1)}>Up</button>
                  <button type="button" className="px-2 text-muted hover:text-text" aria-label="Move down" onClick={() => move(i, 1)}>Down</button>
                </span>
              </li>
            ))}
          </ol>
          <div className="mt-2 flex flex-wrap gap-2">
            <Button disabled={busy} onClick={() => void act(() => tournamentClient.setSeeds({ tournamentId: id, entrantIds: order }), "Seeds saved.")}>Save seeds</Button>
            <Button disabled={busy} onClick={() => void act(() => tournamentClient.reseed({ tournamentId: id, method: SeedingMethod.RANDOM }), "Reseeded at random.")}>Random reseed</Button>
            {t.seedingSource ? <Button disabled={busy} onClick={() => void act(() => tournamentClient.reseed({ tournamentId: id, method: t.seeding, seedingSource: t.seedingSource }), "Reseeded by stats.")}>Reseed by {seedingLabels[t.seeding]}</Button> : null}
          </div>
        </details>
      ) : null}

      {preStart ? (
        <details className="mt-4">
          <summary className="cursor-pointer text-sm font-semibold">Invites ({data.invitedHandles.length})</summary>
          <p className="my-2 text-[12px] text-muted">Invite players by their AimMod Hub handle, separated by commas. Invite-only events accept only these players.</p>
          <div className="flex max-w-lg gap-2">
            <input className="min-h-10 flex-1 rounded-md border border-line bg-bg-2 px-3 text-sm" value={invites} onChange={(e) => setInvites(e.target.value)} aria-label="Handles to invite" />
            <Button disabled={busy || !invites.trim()} onClick={() => void act(() => tournamentClient.inviteEntrants({ tournamentId: id, handles: invites.split(",").map((h) => h.trim()).filter(Boolean) }).then(() => setInvites("")), "Invited.")}>Invite</Button>
          </div>
          {data.invitedHandles.length ? <p className="mt-2 text-[12px] text-muted">{data.invitedHandles.join(", ")}</p> : null}
        </details>
      ) : null}

      <details className="mt-4">
        <summary className="cursor-pointer text-sm font-semibold">Staff and players</summary>
        <div className="mt-2 flex max-w-lg flex-wrap gap-2">
          <input className="min-h-10 flex-1 rounded-md border border-line bg-bg-2 px-3 text-sm" value={staffHandle} onChange={(e) => setStaffHandle(e.target.value)} aria-label="Staff handle" />
          <select className="min-h-10 rounded-md border border-line bg-bg-2 px-2 text-sm" value={staffRole} onChange={(e) => setStaffRole(Number(e.target.value))} aria-label="Role">
            <option value={StaffRole.ADMIN}>Admin</option><option value={StaffRole.CASTER}>Caster</option><option value={StaffRole.UNSPECIFIED}>Remove</option>
          </select>
          <Button disabled={busy || !staffHandle.trim()} onClick={() => void act(() => tournamentClient.setStaff({ tournamentId: id, handle: staffHandle.trim(), role: staffRole }), "Staff updated.")}>Save</Button>
        </div>
        <ul className="mt-3 grid max-w-lg gap-1 text-sm">
          {field.map((e) => (
            <li key={e.id} className="flex items-center justify-between border-b border-line py-1">
              <span>{e.user?.displayName || e.user?.handle}</span>
              <button type="button" className="text-[12px] text-danger hover:underline" onClick={() => {
                const reason = window.prompt(`Disqualify ${e.user?.displayName || e.user?.handle}? Their remaining matches are forfeited. Reason:`);
                if (reason !== null) void act(() => tournamentClient.disqualify({ tournamentId: id, entrantId: e.id, reason }), "Disqualified.");
              }}>Disqualify</button>
            </li>
          ))}
        </ul>
      </details>

      {data.audit.length > 0 ? (
        <details className="mt-4">
          <summary className="cursor-pointer text-sm font-semibold">Activity log</summary>
          <ul className="mt-2 max-h-64 overflow-y-auto text-[12px] text-muted">
            {data.audit.map((a, i) => <li key={i}>{formatWhen(a.at)} · {a.actor?.displayName || a.actor?.handle}: {a.action.replace(/_/g, " ")}{a.detail ? ` (${a.detail})` : ""}</li>)}
          </ul>
        </details>
      ) : null}
    </PageSection>
  );
}
