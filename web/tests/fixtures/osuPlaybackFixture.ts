import { createHash } from "node:crypto";
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const lzma = require("lzma") as { compress: (data: string, mode: number, callback: (bytes: number[], error?: unknown) => void) => void };

export const playbackBeatmap = `osu file format v14
[General]
AudioFilename: test.wav
Mode: 0
StackLeniency: 0.7
[Metadata]
Title: Playback verification
Artist: AimMod
Creator: AimMod
Version: Curves and repeats
BeatmapID: 42
BeatmapSetID: 12
[Difficulty]
HPDrainRate:5
CircleSize:4
OverallDifficulty:7
ApproachRate:6
SliderMultiplier:1.4
SliderTickRate:1
[Events]
[TimingPoints]
0,500,4,1,0,75,1,0
[HitObjects]
100,150,1000,1,0,0:0:0:0:
380,120,1500,1,0,0:0:0:0:
150,220,2200,2,0,B|250:60|390:220,1,300,0|0,0:0|0:0,0:0:0:0:
380,290,4200,6,0,P|260:150|140:290,2,340,0|0|0,0:0|0:0|0:0,0:0:0:0:
256,192,7200,8,0,8300,0:0:0:0:
`;

// Test-fixture encoder only. Production parsing and replay behaviour belong to replayviewer-js.
export async function createPlaybackReplay(mods = 0): Promise<Buffer> {
  let frames = "0|256|192|0,";
  for (let time = 16; time < 8600; time += 16) {
    const x = 256 + 150 * Math.sin(time / 420), y = 192 + 100 * Math.cos(time / 510);
    frames += `16|${x.toFixed(2)}|${y.toFixed(2)}|${time % 320 < 32 ? 5 : 0},`;
  }
  const compressed = Buffer.from(await new Promise<number[]>((resolve, reject) => lzma.compress(frames, 1, (data, error) => error ? reject(error) : resolve(data))));
  const chunks: Buffer[] = [];
  const integer = (value: number, size: number) => { const data = Buffer.alloc(size); data.writeUIntLE(value, 0, size); chunks.push(data); };
  const string = (value: string) => {
    const bytes = Buffer.from(value);
    if (!bytes.length) { integer(0, 1); return; }
    integer(11, 1); let length = bytes.length;
    while (length >= 128) { integer((length & 127) | 128, 1); length >>>= 7; }
    integer(length, 1); chunks.push(bytes);
  };
  integer(0, 1); integer(20260101, 4);
  string(createHash("md5").update(playbackBeatmap).digest("hex")); string("AimMod QA"); string("");
  for (const count of [2, 0, 0, 0, 0, 3]) integer(count, 2);
  integer(100000, 4); integer(2, 2); integer(0, 1); integer(mods, 4); string("");
  chunks.push(Buffer.alloc(8)); integer(compressed.length, 4); chunks.push(compressed, Buffer.alloc(8));
  return Buffer.concat(chunks);
}
