import type * as Rosu from "rosu-pp-js";
import { scorePpPerformanceArgs, validateScorePpObjectCount, validScorePp, type ScorePpInput, type ScorePpResult } from "./scorePp";

export function calculateScorePp(rosu: Pick<typeof Rosu, "Performance" | "Difficulty">, map: Rosu.Beatmap, input: ScorePpInput): ScorePpResult {
  validateScorePpObjectCount(input, map.nObjects);
  const performance = new rosu.Performance(scorePpPerformanceArgs(input));
  try {
    const result = performance.calculate(map);
    try {
      if (!validScorePp(result.pp)) throw new Error("Calculation returned invalid PP");
      // Partial-play PP and full-map difficulty have different object limits.
      const difficulty = new rosu.Difficulty({ mods: input.mods!, lazer: input.lazer! });
      try {
        const attributes = difficulty.calculate(map);
        try { return { pp: result.pp, stars: attributes.stars, objectCount: map.nObjects }; }
        finally { attributes.free(); }
      } finally { difficulty.free(); }
    } finally { result.free(); }
  } finally { performance.free(); }
}
