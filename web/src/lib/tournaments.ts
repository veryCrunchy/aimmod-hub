import { createClient, ConnectError } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { API_BASE_URL } from "./config";
import { TournamentService } from "../gen/aimmod/tournament/v1/tournament_connect";
import {
  BracketSide,
  EntrantStatus,
  MatchState,
  Resolution,
  SeedingMethod,
  TournamentFormat,
  TournamentStatus,
  VetoAction,
  type Entrant,
  type Match,
} from "../gen/aimmod/tournament/v1/tournament_pb";

// Tournament calls carry the session cookie: registering, checking in and
// organising all act as the signed-in player.
const transport = createConnectTransport({
  baseUrl: API_BASE_URL,
  fetch: (input, init) => fetch(input, { ...init, credentials: "include" }),
});

export const tournamentClient = createClient(TournamentService, transport);

export const REPLAY_DOWNLOAD = (tournamentId: string, replayId: string) =>
  `${API_BASE_URL}/api/tournaments/v1/replays/${encodeURIComponent(tournamentId)}/${encodeURIComponent(replayId)}`;

export function errorMessage(err: unknown, fallback = "Something went wrong."): string {
  if (err instanceof ConnectError) return err.rawMessage || fallback;
  if (err instanceof Error) return err.message || fallback;
  return fallback;
}

export const formatLabels: Record<number, string> = {
  [TournamentFormat.SINGLE_ELIMINATION]: "Single elimination",
  [TournamentFormat.DOUBLE_ELIMINATION]: "Double elimination",
  [TournamentFormat.ROUND_ROBIN]: "Round robin",
  [TournamentFormat.SWISS]: "Swiss",
};

export const statusLabels: Record<number, string> = {
  [TournamentStatus.DRAFT]: "Draft",
  [TournamentStatus.REGISTRATION]: "Registration open",
  [TournamentStatus.CHECK_IN]: "Check-in",
  [TournamentStatus.IN_PROGRESS]: "In progress",
  [TournamentStatus.COMPLETED]: "Completed",
  [TournamentStatus.CANCELLED]: "Cancelled",
};

export const seedingLabels: Record<number, string> = {
  [SeedingMethod.MANUAL]: "Manual",
  [SeedingMethod.RANDOM]: "Random",
  [SeedingMethod.BENCHMARK_RANK]: "Benchmark rank",
  [SeedingMethod.SCENARIO_PB]: "Scenario personal best",
};

export const matchStateLabels: Record<number, string> = {
  [MatchState.PENDING]: "Waiting",
  [MatchState.READY]: "Ready to play",
  [MatchState.VETO]: "Picks and bans",
  [MatchState.LIVE]: "Live",
  [MatchState.AWAITING_CONFIRMATION]: "Awaiting confirmation",
  [MatchState.DISPUTED]: "Disputed",
  [MatchState.COMPLETE]: "Complete",
  [MatchState.SKIPPED]: "Not needed",
};

export const entrantStatusLabels: Record<number, string> = {
  [EntrantStatus.REGISTERED]: "Registered",
  [EntrantStatus.CHECKED_IN]: "Checked in",
  [EntrantStatus.ACTIVE]: "Playing",
  [EntrantStatus.ELIMINATED]: "Eliminated",
  [EntrantStatus.DISQUALIFIED]: "Disqualified",
  [EntrantStatus.WITHDRAWN]: "Withdrawn",
  [EntrantStatus.NO_SHOW]: "No-show",
};

export const resolutionLabels: Record<number, string> = {
  [Resolution.WALKOVER]: "Bye",
  [Resolution.FORFEIT]: "Forfeit",
  [Resolution.NO_SHOW]: "No-show",
  [Resolution.ADMIN_OVERRIDE]: "Set by organisers",
};

export const vetoLabels: Record<number, string> = {
  [VetoAction.BAN]: "bans",
  [VetoAction.PICK]: "picks",
  [VetoAction.DECIDER]: "decider",
};

export const sideLabels: Record<number, string> = {
  [BracketSide.WINNERS]: "Winners bracket",
  [BracketSide.LOSERS]: "Losers bracket",
  [BracketSide.GRAND_FINAL]: "Grand final",
  [BracketSide.THIRD_PLACE]: "Bronze match",
  [BracketSide.ROUND_ROBIN]: "Round robin",
  [BracketSide.SWISS]: "Swiss",
};

export function entrantName(entrants: Map<string, Entrant>, id: string | undefined, empty = "TBD"): string {
  if (!id) return empty;
  const e = entrants.get(id);
  return e?.user?.displayName || e?.user?.handle || "Unknown player";
}

export function byId(entrants: Entrant[]): Map<string, Entrant> {
  return new Map(entrants.map((e) => [e.id, e]));
}

export function formatWhen(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

// toLocalInput / fromLocalInput convert between RFC 3339 and <input type="datetime-local">.
export function toLocalInput(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function fromLocalInput(value: string): string {
  if (!value) return "";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? "" : d.toISOString().replace(/\.\d{3}Z$/, "Z");
}

export function isActive(m: Match): boolean {
  return m.state === MatchState.READY || m.state === MatchState.VETO || m.state === MatchState.LIVE ||
    m.state === MatchState.AWAITING_CONFIRMATION || m.state === MatchState.DISPUTED;
}
