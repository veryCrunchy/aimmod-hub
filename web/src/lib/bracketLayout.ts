// Groups a tournament's matches into drawable bracket sections: one section
// per side (winners, losers, grand final, ...) with a column per round.
// Kept free of React so it can be tested on its own.

export type LayoutMatch = {
  id: string;
  side: number;
  round: number;
  position: number;
  label: string;
};

export type LayoutColumn<M extends LayoutMatch> = { round: number; title: string; matches: M[] };
export type LayoutSection<M extends LayoutMatch> = { side: number; columns: LayoutColumn<M>[] };

// Side order on the page: winners, losers, grand final, bronze, then tables.
const order = [1, 2, 3, 4, 5, 6];

export function bracketSections<M extends LayoutMatch>(matches: M[]): LayoutSection<M>[] {
  const sides = new Map<number, Map<number, M[]>>();
  for (const m of matches) {
    const rounds = sides.get(m.side) ?? new Map<number, M[]>();
    const list = rounds.get(m.round) ?? [];
    list.push(m);
    rounds.set(m.round, list);
    sides.set(m.side, rounds);
  }
  // The grand final and its reset sit in one section, left to right.
  return [...sides.entries()]
    .sort((a, b) => order.indexOf(a[0]) - order.indexOf(b[0]))
    .map(([side, rounds]) => ({
      side,
      columns: [...rounds.entries()]
        .sort((a, b) => a[0] - b[0])
        .map(([round, list]) => {
          const sorted = [...list].sort((a, b) => a.position - b.position);
          return { round, title: columnTitle(sorted), matches: sorted };
        }),
    }));
}

// A column is named after its matches when they share a label ("Semi-final"),
// otherwise by round.
function columnTitle(matches: LayoutMatch[]): string {
  const labels = new Set(matches.map((m) => m.label));
  if (labels.size === 1) return matches[0]?.label ?? "";
  return `Round ${matches[0]?.round ?? ""}`;
}
