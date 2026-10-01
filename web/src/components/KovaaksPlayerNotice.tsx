import { Link } from "react-router-dom";
import { Button } from "./ui/Button";

/** Marks a page that shows a KovaaK's player from public KovaaK's data. */
export function KovaaksPlayerNotice({ steamId, aimmodHandle, name }: { steamId: string; aimmodHandle?: string; name?: string }) {
  if (aimmodHandle) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-mint/40 bg-mint/5 px-4 py-3 text-sm">
        <span>{name ? `${name} is` : "This player is"} on AimMod. Their profile adds runs, replays and score history.</span>
        <Button to={`/profiles/${aimmodHandle}`}>Open AimMod profile</Button>
      </div>
    );
  }
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-line bg-panel px-4 py-3 text-sm">
      <div className="min-w-0">
        <div className="font-medium">KovaaK's player · not on AimMod</div>
        <p className="mt-0.5 text-muted">Ranks and scores come from KovaaK's public leaderboards. Nothing here was uploaded through AimMod.</p>
      </div>
      <Link to={`/app/kovaaks?claim=${encodeURIComponent(steamId)}#link-account`} className="inline-flex min-h-10 items-center rounded-md border border-line bg-bg-2 px-3.5 text-sm font-medium hover:border-line-strong">
        This is me: link my account
      </Link>
    </div>
  );
}
