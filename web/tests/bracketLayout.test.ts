import { test } from "node:test";
import assert from "node:assert/strict";
import { bracketSections } from "../src/lib/bracketLayout";
import { gameForPath } from "../src/lib/hubGame";
import { fromLocalInput, toLocalInput } from "../src/lib/tournaments";

const m = (id: string, side: number, round: number, position: number, label = `R${round}`) => ({ id, side, round, position, label });

test("sections follow the bracket sides and rounds in order", () => {
  const sections = bracketSections([
    m("GF1", 3, 1, 1, "Grand final"),
    m("L1-1", 2, 1, 1),
    m("W2-1", 1, 2, 1, "Winners final"),
    m("W1-2", 1, 1, 2, "Winners semi-final"),
    m("W1-1", 1, 1, 1, "Winners semi-final"),
    m("GF2", 3, 2, 1, "Grand final reset"),
  ]);
  assert.deepEqual(sections.map((s) => s.side), [1, 2, 3]);
  assert.deepEqual(sections[0].columns.map((c) => c.title), ["Winners semi-final", "Winners final"]);
  assert.deepEqual(sections[0].columns[0].matches.map((x) => x.id), ["W1-1", "W1-2"]);
  assert.deepEqual(sections[2].columns.map((c) => c.title), ["Grand final", "Grand final reset"]);
});

test("mixed labels fall back to the round number", () => {
  const [section] = bracketSections([m("R1-1", 5, 1, 1, "a"), m("R1-2", 5, 1, 2, "b")]);
  assert.equal(section.columns[0].title, "Round 1");
});

test("tournament pages belong to the KovaaK's section", () => {
  for (const path of ["/tournaments", "/tournaments/new", "/tournaments/synthetic-cup/matches/W1-1"]) assert.equal(gameForPath(path), "kovaaks");
});

test("datetime inputs round-trip through RFC 3339", () => {
  const iso = "2026-05-01T18:30:00Z";
  assert.equal(fromLocalInput(toLocalInput(iso)), iso);
  assert.equal(fromLocalInput(""), "");
  assert.equal(toLocalInput("not a date"), "");
});

test("pool lines parse names and lengths", async () => {
  const { parsePool, defaultVeto } = await import("../src/lib/tournamentForm");
  assert.deepEqual(parsePool("Alpha\n\n Bravo | 60 \r\nCharlie|").pool, [
    { name: "Alpha", timeLimit: 0 }, { name: "Bravo", timeLimit: 60 }, { name: "Charlie", timeLimit: 0 },
  ]);
  assert.match(parsePool("Alpha\nalpha").error ?? "", /twice/);
  assert.match(parsePool("Alpha | 5").error ?? "", /10 to 600/);
  assert.match(parsePool("  ").error ?? "", /at least one/);
  assert.deepEqual(defaultVeto(5, 3), [
    { action: "ban", actor: "higher" }, { action: "ban", actor: "lower" }, { action: "pick", actor: "higher" }, { action: "pick", actor: "lower" },
  ]);
  assert.deepEqual(defaultVeto(1, 1), []);
});
