import { Link } from "react-router-dom";
import { MatchState, type Entrant, type Match } from "../gen/aimmod/tournament/v1/tournament_pb";
import { bracketSections } from "../lib/bracketLayout";
import { cn } from "../lib/cn";
import { entrantName, matchStateLabels, resolutionLabels, sideLabels } from "../lib/tournaments";

const CARD = 64; // px, one match card
const GAP = 14;

function Slot({ entrants, id, empty, wins, winner, highlight }: {
  entrants: Map<string, Entrant>; id: string; empty: boolean; wins: number; winner: boolean; highlight?: string;
}) {
  const e = id ? entrants.get(id) : undefined;
  return (
    <div className={cn("flex h-[28px] items-center justify-between gap-2 px-2 text-[12px]",
      winner ? "text-text" : "text-muted", highlight && id === highlight && "bg-mint/10")}>
      <span className="flex min-w-0 items-center gap-1.5">
        {e?.seed ? <span className="w-5 shrink-0 text-right text-[10px] text-muted-2">{e.seed}</span> : <span className="w-5 shrink-0" />}
        <span className={cn("truncate", winner && "font-semibold")}>{empty ? "Bye" : entrantName(entrants, id)}</span>
      </span>
      <span className={cn("tabular-nums", winner ? "text-mint" : "text-muted-2")}>{id ? wins : ""}</span>
    </div>
  );
}

export function MatchCard({ match, entrants, tournamentId, highlight }: {
  match: Match; entrants: Map<string, Entrant>; tournamentId: string; highlight?: string;
}) {
  const live = match.state === MatchState.LIVE || match.state === MatchState.VETO;
  const attention = match.state === MatchState.DISPUTED || match.flags.includes("needs-review") || match.flags.includes("no-show-both");
  const winner = match.winnerEntrantId;
  const note = match.state === MatchState.COMPLETE ? resolutionLabels[match.resolution] : matchStateLabels[match.state];
  return (
    <Link
      to={`/tournaments/${encodeURIComponent(tournamentId)}/matches/${encodeURIComponent(match.id)}`}
      title={`${match.label} · ${note ?? ""}`}
      className={cn("block w-[210px] overflow-hidden rounded-[10px] border bg-white/2 transition-colors hover:border-cyan/50",
        live ? "border-mint/60" : attention ? "border-gold/60" : "border-line",
        match.state === MatchState.SKIPPED && "opacity-40")}
      style={{ height: CARD }}
    >
      <Slot entrants={entrants} id={match.slotA?.entrantId ?? ""} empty={Boolean(match.slotA?.empty)} wins={match.winsA} winner={!!winner && winner === match.slotA?.entrantId} highlight={highlight} />
      <div className="h-px bg-line" />
      <Slot entrants={entrants} id={match.slotB?.entrantId ?? ""} empty={Boolean(match.slotB?.empty)} wins={match.winsB} winner={!!winner && winner === match.slotB?.entrantId} highlight={highlight} />
      <span className="sr-only">{note}</span>
    </Link>
  );
}

// A bracket drawn as columns of rounds. Columns share one height and spread
// their cards evenly, so each match sits centred between the two it follows.
export function TournamentBracket({ matches, entrants, tournamentId, highlight }: {
  matches: Match[]; entrants: Map<string, Entrant>; tournamentId: string; highlight?: string;
}) {
  const sections = bracketSections(matches);
  if (sections.length === 0) return null;
  return (
    <div className="grid gap-6">
      {sections.map((section) => {
        const tallest = Math.max(...section.columns.map((c) => c.matches.length));
        const height = tallest * CARD + (tallest - 1) * GAP;
        return (
          <section key={section.side} aria-label={sideLabels[section.side]}>
            <h3 className="mb-2 text-sm font-semibold text-text">{sideLabels[section.side]}</h3>
            <div className="overflow-x-auto pb-2">
              <div className="flex min-w-max gap-6">
                {section.columns.map((column) => (
                  <div key={column.round} className="flex w-[210px] shrink-0 flex-col">
                    <div className="mb-2 truncate text-[11px] font-medium uppercase tracking-[0.06em] text-muted">{column.title}</div>
                    <div className="flex flex-col justify-around" style={{ height }}>
                      {column.matches.map((m) => (
                        <MatchCard key={m.id} match={m} entrants={entrants} tournamentId={tournamentId} highlight={highlight} />
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </section>
        );
      })}
    </div>
  );
}
