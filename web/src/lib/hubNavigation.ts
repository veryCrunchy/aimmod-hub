import type { HubGame } from "./hubGame";

export type NavGroup = { label: string; links: [to: string, label: string][] };

export const kovaaksNav: NavGroup[] = [
  { label: "", links: [["/kovaaks", "Overview"], ["/live", "Live now"]] },
  { label: "Stats", links: [["/leaderboard", "Leaderboards"], ["/benchmarks", "Benchmarks"], ["/community", "Players & scenarios"], ["/replays", "Replays"]] },
  { label: "Compete", links: [["/tournaments", "Tournaments"]] },
  { label: "Improve", links: [["/learn", "Training guides"]] },
  { label: "AimMod", links: [["/app/kovaaks", "Get AimMod for KovaaK's"]] },
];

export const osuNav: NavGroup[] = [
  { label: "Browse", links: [["/osu", "Overview"], ["/osu/pp-targets", "PP beatmaps"], ["/osu/beatmaps", "All beatmaps"], ["/osu/replays", "Replays"]] },
  { label: "Make it yours", links: [["/osu/skins", "Skins"], ["/osu/skin-builder", "Skin builder"]] },
  { label: "Community", links: [["/osu/players", "Players"], ["/osu/community", "Activity"], ["/osu/learn", "Learning guides"]] },
  { label: "AimMod", links: [["/app/osu", "Download AimMod"], ["/osu/help", "App guide"]] },
];

export type AccountNavInput = { authenticated: boolean; isAdmin: boolean; profileHandle?: string };

/** Links for the signed-in player, scoped to the game they are browsing. */
export function accountNav(game: HubGame, account: AccountNavInput): [to: string, label: string][] {
  if (!account.authenticated) return [];
  const links: [string, string][] = [];
  if (game === "kovaaks" && account.profileHandle) links.push([`/profiles/${account.profileHandle}`, "My profile"]);
  if (game === "osu") links.push(["/osu/training", "My training"]);
  links.push(["/account", "Account & devices"]);
  if (account.isAdmin) links.push(["/admin", "Administration"]);
  return links;
}

export function navFor(game: HubGame): NavGroup[] {
  return game === "osu" ? osuNav : kovaaksNav;
}
