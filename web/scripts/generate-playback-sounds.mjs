import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

// Original short percussion samples, reproducible without external media.
const directory = new URL("../public/playback/aimmod-sounds/", import.meta.url);
mkdirSync(directory, { recursive: true });
for (const [set, fundamental] of [["normal", 880], ["soft", 660], ["drum", 220]]) {
  for (const [kind, duration, overtone] of [["hitnormal", .055, 2], ["hitwhistle", .09, 3], ["hitfinish", .14, 5], ["hitclap", .055, 7]]) {
    const rate = 22050, count = Math.ceil(rate * duration), bytes = Buffer.alloc(44 + count * 2);
    bytes.write("RIFF", 0); bytes.writeUInt32LE(bytes.length - 8, 4); bytes.write("WAVEfmt ", 8);
    bytes.writeUInt32LE(16, 16); bytes.writeUInt16LE(1, 20); bytes.writeUInt16LE(1, 22);
    bytes.writeUInt32LE(rate, 24); bytes.writeUInt32LE(rate * 2, 28); bytes.writeUInt16LE(2, 32); bytes.writeUInt16LE(16, 34);
    bytes.write("data", 36); bytes.writeUInt32LE(count * 2, 40);
    for (let i = 0; i < count; i++) {
      const time = i / rate, envelope = Math.min(1, time / .002) * Math.exp(-time / (duration / 6));
      const sample = (Math.sin(2 * Math.PI * fundamental * time) + .22 * Math.sin(2 * Math.PI * fundamental * overtone * time)) * envelope * .3;
      bytes.writeInt16LE(Math.round(sample * 32767), 44 + i * 2);
    }
    writeFileSync(fileURLToPath(new URL(`${set}-${kind}.wav`, directory)), bytes);
  }
}
