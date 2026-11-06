export function snapshotStatus(updated: string, pending: boolean, now = Date.now()): string {
 if (pending) return "Refreshing leaderboard. Check back shortly.";
 const stamp = Date.parse(updated);
 return Number.isFinite(stamp) ? `Updated ${Math.max(0, Math.floor((now - stamp) / 60000))} min ago` : "Waiting for an update";
}
