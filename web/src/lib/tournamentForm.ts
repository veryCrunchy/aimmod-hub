// Form helpers for creating tournaments (no React, so they can be tested).

export type PoolEntry = { name: string; timeLimit: number };

// One scenario per line; "Name | 60" sets a 60-second length.
export function parsePool(text: string): { pool: PoolEntry[]; error?: string } {
  const pool: PoolEntry[] = [];
  const seen = new Set<string>();
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line) continue;
    const [namePart, limitPart] = line.split("|").map((s) => s.trim());
    let timeLimit = 0;
    if (limitPart !== undefined && limitPart !== "") {
      timeLimit = Number(limitPart);
      if (!Number.isInteger(timeLimit) || timeLimit < 10 || timeLimit > 600) {
        return { pool, error: `“${namePart}”: lengths are 10 to 600 seconds.` };
      }
    }
    if (!namePart) return { pool, error: "A pool line has no scenario name." };
    const key = namePart.toLowerCase();
    if (seen.has(key)) return { pool, error: `“${namePart}” is in the pool twice.` };
    seen.add(key);
    pool.push({ name: namePart, timeLimit });
  }
  if (pool.length === 0) return { pool, error: "Add at least one scenario to the pool." };
  if (pool.length > 16) return { pool, error: "The pool holds at most 16 scenarios." };
  return { pool };
}

export type VetoStepDraft = { action: "ban" | "pick"; actor: "higher" | "lower" };

// Mirrors the Hub's DefaultVeto: alternate bans (higher seed first) down to
// best-of scenarios, then alternate picks; the last game is the decider.
export function defaultVeto(pool: number, bestOf: number): VetoStepDraft[] {
  const steps: VetoStepDraft[] = [];
  const actor = (i: number): VetoStepDraft["actor"] => (i % 2 === 0 ? "higher" : "lower");
  let n = 0;
  for (let i = 0; i < pool - bestOf; i++) steps.push({ action: "ban", actor: actor(n++) });
  for (let i = 0; i < bestOf - 1; i++) steps.push({ action: "pick", actor: actor(i) });
  return steps;
}
