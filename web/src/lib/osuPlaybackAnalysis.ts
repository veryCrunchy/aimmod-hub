import type { BeatmapData, HitResult } from "replayviewer-js";
import type { OsuReplayAnalysis } from "./osuCommunity";

export function playbackAnalysis(results: readonly HitResult[], beatmap: BeatmapData): OsuReplayAnalysis {
  return {
    timeBasis: "beatmap",
    judgements: results.filter(hit => !hit.isSliderSub || hit.comboBreak).map(hit => {
      const object = beatmap.hitObjects[hit.objectIndex];
      return {
        objectIndex: hit.objectIndex,
        objectType: object?.type,
        startTimeMs: hit.isSliderSub ? hit.time : object?.time ?? hit.time,
        result: hit.isSliderSub && hit.comboBreak ? "SliderBreak" : hit.judgement === 0 ? "Miss" : String(hit.judgement),
        // Miss display time is a timeout, not a measured keypress offset.
        timeOffsetMs: hit.judgement !== 0 && object?.type === "circle" ? hit.time - object.time : undefined,
      };
    }),
  };
}
