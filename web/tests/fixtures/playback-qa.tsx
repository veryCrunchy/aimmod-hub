import React from "react";
import { createRoot } from "react-dom/client";
import { OsuReplayPlayer } from "../../src/components/OsuReplayPlayer";

const root = createRoot(document.getElementById("root")!);
(window as unknown as { unmountReplay: () => void }).unmountReplay = () => root.unmount();
root.render(<main style={{ maxWidth: 1160, margin: "24px auto", padding: "0 12px" }}><OsuReplayPlayer replayUrl="/__qa/replay.osr" beatmapId={42} beatmapsetId={12}
  beatmapUrl="/__qa/beatmap.osu" title="AimMod playback verification" onAnalysis={analysis => { (window as any).__replayAnalysis = analysis; }} /></main>);
