export type HubGame = "osu" | "kovaaks";

export function gameForPath(path: string): HubGame | null {
  if (/^\/osu(?:\/|$)/.test(path) || /^\/app\/osu(?:\/|$)/.test(path)) return "osu";
  if (path === "/kovaaks" || /^\/(community|replays|live|benchmarks|leaderboard|profiles|scenarios|runs|u|search|learn)(?:\/|$)/.test(path) || path === "/app/kovaaks") return "kovaaks";
  return null;
}
