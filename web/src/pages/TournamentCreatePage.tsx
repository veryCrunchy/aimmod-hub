import { useState, type FormEvent, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import {
  RegistrationMode,
  SchedulingMode,
  SeedingMethod,
  TournamentFormat,
  VetoAction,
  VetoActor,
} from "../gen/aimmod/tournament/v1/tournament_pb";
import { PageSeo } from "../components/PageSeo";
import { SectionHeader } from "../components/SectionHeader";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { useAuth } from "../lib/AuthContext";
import { discordStartUrl } from "../lib/auth";
import { errorMessage, formatLabels, fromLocalInput, seedingLabels, tournamentClient } from "../lib/tournaments";
import { defaultVeto, parsePool } from "../lib/tournamentForm";

const input = "min-h-10 w-full rounded-md border border-line bg-bg-2 px-3 text-sm text-text focus:border-cyan focus:outline-none";

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="grid gap-1.5 text-sm">
      <span className="font-medium text-text">{label}</span>
      {children}
      {hint ? <span className="text-[12px] text-muted">{hint}</span> : null}
    </label>
  );
}

export function TournamentCreatePage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [format, setFormat] = useState(TournamentFormat.SINGLE_ELIMINATION);
  const [maxEntrants, setMaxEntrants] = useState(16);
  const [inviteOnly, setInviteOnly] = useState(false);
  const [seeding, setSeeding] = useState(SeedingMethod.RANDOM);
  const [seedingSource, setSeedingSource] = useState("");
  const [scheduled, setScheduled] = useState(false);
  const [startsAt, setStartsAt] = useState("");
  const [checkInOpens, setCheckInOpens] = useState("");
  const [checkInCloses, setCheckInCloses] = useState("");
  const [bestOf, setBestOf] = useState(3);
  const [finalBestOf, setFinalBestOf] = useState(0);
  const [pool, setPool] = useState("");
  const [veto, setVeto] = useState(true);
  const [requireReplays, setRequireReplays] = useState(true);
  const [thirdPlace, setThirdPlace] = useState(false);
  const [reset, setReset] = useState(true);
  const [cycles, setCycles] = useState(1);
  const [swissRounds, setSwissRounds] = useState(0);
  const [prize, setPrize] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!auth.loading && !auth.authenticated) {
    return (
      <PageStack>
        <PageSection>
          <EmptyState title="Sign in to create a tournament" body="Tournaments are run from your AimMod Hub account.">
            <Button href={discordStartUrl("/tournaments/new")} variant="primary">Sign in with Discord</Button>
          </EmptyState>
        </PageSection>
      </PageStack>
    );
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const scenarios = parsePool(pool);
    if (scenarios.error) { setError(scenarios.error); return; }
    setBusy(true);
    try {
      // Without enough scenarios for every game, the games cycle through the pool instead.
      const steps = veto && scenarios.pool.length > 1 && scenarios.pool.length >= Math.max(bestOf, finalBestOf) ? defaultVeto(scenarios.pool.length, bestOf) : [];
      const res = await tournamentClient.createTournament({
        spec: {
          name, description, format, maxEntrants, prize,
          registrationMode: inviteOnly ? RegistrationMode.INVITE_ONLY : RegistrationMode.OPEN,
          seeding, seedingSource,
          scheduling: scheduled ? SchedulingMode.SCHEDULED : SchedulingMode.READY_WHEN_ONLINE,
          startsAt: fromLocalInput(startsAt), checkInOpensAt: fromLocalInput(checkInOpens), checkInClosesAt: fromLocalInput(checkInCloses),
          ruleset: {
            gameMode: "score-race", bestOf, requireReplays,
            pool: scenarios.pool.map((p) => ({ name: p.name, timeLimitSeconds: p.timeLimit })),
            veto: steps.map((s) => ({ action: s.action === "ban" ? VetoAction.BAN : VetoAction.PICK, actor: s.actor === "higher" ? VetoActor.HIGHER_SEED : VetoActor.LOWER_SEED })),
            // The grand final (and its reset) or the winners final.
            roundBestOf: finalBestOf ? [format === TournamentFormat.DOUBLE_ELIMINATION ? { side: 3, fromRound: 1, bestOf: finalBestOf } : { side: 1, fromRound: 0, bestOf: finalBestOf }] : [],
          },
          options: { thirdPlaceMatch: thirdPlace, grandFinalReset: reset, roundRobinCycles: cycles, swissRounds },
        },
      });
      const t = res.tournament!;
      navigate(`/tournaments/${encodeURIComponent(t.slug || t.id)}`);
    } catch (err) {
      setError(errorMessage(err, "Could not create the tournament."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <PageStack>
      <PageSeo title="Create a tournament · AimMod Hub" description="Set up an AimMod tournament for KovaaK's." noindex />
      <PageSection>
        <Breadcrumb crumbs={[{ label: "Tournaments", to: "/tournaments" }, { label: "Create" }]} />
        <SectionHeader level={1} eyebrow="Organise" title="Create a tournament"
          body="It starts as a draft only you can see. Open registration when it's ready. Formats, seeding and the ruleset are fixed once the bracket is out." />
      </PageSection>
      <PageSection>
        <form className="grid max-w-3xl gap-5" onSubmit={(e) => void submit(e)}>
          <Field label="Name"><input className={input} value={name} onChange={(e) => setName(e.target.value)} required minLength={3} maxLength={80} /></Field>
          <Field label="Description" hint="Rules, schedule, where to talk. Plain text.">
            <textarea className={`${input} min-h-24 py-2`} value={description} onChange={(e) => setDescription(e.target.value)} maxLength={2000} />
          </Field>
          <div className="grid gap-4 md:grid-cols-2">
            <Field label="Format">
              <select className={input} value={format} onChange={(e) => setFormat(Number(e.target.value))}>
                {[1, 2, 3, 4].map((f) => <option key={f} value={f}>{formatLabels[f]}</option>)}
              </select>
            </Field>
            <Field label="Player cap"><input className={input} type="number" min={2} max={256} value={maxEntrants} onChange={(e) => setMaxEntrants(Number(e.target.value))} /></Field>
            <Field label="Registration">
              <select className={input} value={inviteOnly ? "invite" : "open"} onChange={(e) => setInviteOnly(e.target.value === "invite")}>
                <option value="open">Open to everyone</option>
                <option value="invite">Invite only</option>
              </select>
            </Field>
            <Field label="Seeding" hint={seeding === SeedingMethod.BENCHMARK_RANK ? "Players without a rank are seeded last." : undefined}>
              <select className={input} value={seeding} onChange={(e) => setSeeding(Number(e.target.value))}>
                {[2, 1, 3, 4].map((s) => <option key={s} value={s}>{seedingLabels[s]}</option>)}
              </select>
            </Field>
            {seeding === SeedingMethod.BENCHMARK_RANK || seeding === SeedingMethod.SCENARIO_PB ? (
              <Field label={seeding === SeedingMethod.BENCHMARK_RANK ? "Benchmark id" : "Scenario"} hint={seeding === SeedingMethod.SCENARIO_PB ? "Personal bests from runs uploaded to AimMod Hub." : "The number in the benchmark's Hub address."}>
                <input className={input} value={seedingSource} onChange={(e) => setSeedingSource(e.target.value)} required />
              </Field>
            ) : null}
          </div>
          <fieldset className="grid gap-4 md:grid-cols-2">
            <legend className="mb-2 text-sm font-semibold text-text">Schedule</legend>
            <Field label="Matches start" hint={scheduled ? "Players who aren't ready 10 minutes after the start forfeit." : "Each match starts when both players are online and ready."}>
              <select className={input} value={scheduled ? "scheduled" : "ready"} onChange={(e) => setScheduled(e.target.value === "scheduled")}>
                <option value="ready">When both players are ready</option>
                <option value="scheduled">At the start time</option>
              </select>
            </Field>
            <Field label="Start time"><input className={input} type="datetime-local" value={startsAt} onChange={(e) => setStartsAt(e.target.value)} required={scheduled} /></Field>
            <Field label="Check-in opens" hint="Leave both empty for no check-in."><input className={input} type="datetime-local" value={checkInOpens} onChange={(e) => setCheckInOpens(e.target.value)} /></Field>
            <Field label="Check-in closes"><input className={input} type="datetime-local" value={checkInCloses} onChange={(e) => setCheckInCloses(e.target.value)} /></Field>
          </fieldset>
          <fieldset className="grid gap-4 md:grid-cols-2">
            <legend className="mb-2 text-sm font-semibold text-text">Ruleset</legend>
            <Field label="Best of">
              <select className={input} value={bestOf} onChange={(e) => setBestOf(Number(e.target.value))}>
                {[1, 3, 5, 7].map((n) => <option key={n} value={n}>Best of {n}</option>)}
              </select>
            </Field>
            <Field label="Final">
              <select className={input} value={finalBestOf} onChange={(e) => setFinalBestOf(Number(e.target.value))}>
                <option value={0}>Same as other matches</option>
                {[3, 5, 7].map((n) => <option key={n} value={n}>Best of {n}</option>)}
              </select>
            </Field>
            <div className="md:col-span-2">
              <Field label="Scenario pool" hint="One scenario per line, exactly as named in KovaaK's. Add | seconds to set a length, e.g. 1wall6targets TE | 60.">
                <textarea className={`${input} min-h-28 py-2 font-mono text-[12px]`} value={pool} onChange={(e) => setPool(e.target.value)} required />
              </Field>
            </div>
            <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={veto} onChange={(e) => setVeto(e.target.checked)} /> Picks and bans (alternating, higher seed first)</label>
            <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={requireReplays} onChange={(e) => setRequireReplays(e.target.checked)} /> Require replays from both players</label>
          </fieldset>
          <fieldset className="grid gap-4 md:grid-cols-2">
            <legend className="mb-2 text-sm font-semibold text-text">Format options</legend>
            {format === TournamentFormat.SINGLE_ELIMINATION ? <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={thirdPlace} onChange={(e) => setThirdPlace(e.target.checked)} /> Bronze match</label> : null}
            {format === TournamentFormat.DOUBLE_ELIMINATION ? <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={reset} onChange={(e) => setReset(e.target.checked)} /> Grand final reset</label> : null}
            {format === TournamentFormat.ROUND_ROBIN ? (
              <Field label="Each pair meets"><select className={input} value={cycles} onChange={(e) => setCycles(Number(e.target.value))}><option value={1}>Once</option><option value={2}>Twice</option></select></Field>
            ) : null}
            {format === TournamentFormat.SWISS ? (
              <Field label="Rounds" hint="0 picks enough rounds to find one unbeaten player."><input className={input} type="number" min={0} max={15} value={swissRounds} onChange={(e) => setSwissRounds(Number(e.target.value))} /></Field>
            ) : null}
          </fieldset>
          <Field label="Prize" hint="Shown on the event page. AimMod doesn't handle prizes."><input className={input} value={prize} onChange={(e) => setPrize(e.target.value)} maxLength={500} /></Field>
          {error ? <p role="alert" className="text-sm text-danger">{error}</p> : null}
          <div><Button type="submit" variant="primary" disabled={busy}>{busy ? "Creating..." : "Create draft"}</Button></div>
        </form>
      </PageSection>
    </PageStack>
  );
}
