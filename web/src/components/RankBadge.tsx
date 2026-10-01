import { cn } from "../lib/cn";
import { hasRankName, rankColor } from "../lib/benchmarkSheet";

type RankBadgeProps = {
  rankName?: string | null;
  iconUrl?: string | null;
  color?: string | null;
  rankIndex?: number;
  size?: "sm" | "md" | "lg";
  /** Text for players without a rank. */
  emptyLabel?: string;
  className?: string;
};

/** A benchmark rank: icon, name and the rank's colour as an accent. */
export function RankBadge({ rankName, iconUrl, color, rankIndex = 0, size = "sm", emptyLabel = "Unranked", className }: RankBadgeProps) {
  const ranked = hasRankName(rankName);
  const accent = ranked ? rankColor(color ?? undefined, rankIndex) : "#6b7280";
  const icon = size === "lg" ? "h-10 w-10" : size === "md" ? "h-6 w-6" : "h-4 w-4";
  const text = size === "lg" ? "text-lg font-semibold" : size === "md" ? "text-sm font-medium" : "text-xs font-medium";
  return (
    <span
      className={cn("inline-flex max-w-full items-center gap-1.5 rounded-md border px-1.5 py-0.5 align-middle", size === "lg" && "gap-2.5 px-3 py-2", className)}
      style={{ borderColor: `${accent}55`, background: `${accent}14` }}
    >
      {ranked && iconUrl ? <img src={iconUrl} alt="" className={cn(icon, "shrink-0 object-contain")} loading="lazy" /> : (
        <span aria-hidden className={cn("shrink-0 rounded-full", size === "lg" ? "h-3 w-3" : "h-2 w-2")} style={{ background: accent }} />
      )}
      <span className={cn("truncate", text)} style={{ color: ranked ? accent : undefined }}>
        {ranked ? rankName : emptyLabel}
      </span>
    </span>
  );
}
