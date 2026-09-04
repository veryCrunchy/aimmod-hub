export type OsuSkillset = "Aim" | "Speed" | "Reading" | "Consistency" | "Finger control";
export type OsuLocalState = "Installed" | "Not installed" | "Update available";

export interface OsuBeatmap {
  id: string;
  artist: string;
  title: string;
  difficulty: string;
  mapper: string;
  stars: number;
  bpm: number;
  lengthSeconds: number;
  status: "Ranked" | "Loved" | "Qualified";
  source: "osu!" | "osu!Collector";
  localState: OsuLocalState;
  skillsets: OsuSkillset[];
  pp95: number;
  accuracy: number | null;
  playCount: number;
  cover: string;
}

export interface OsuReplay {
  id: string;
  beatmapId: string;
  player: string;
  score: number;
  accuracy: number;
  combo: number;
  misses: number;
  mods: string[];
  playedAt: string;
  pp: number;
  cursorControl: number;
  tapControl: number;
  markers: Array<{ time: string; label: string; severity: "good" | "watch" | "miss" }>;
}

export interface OsuWorkspaceData {
  player: {
    username: string;
    globalRank: number;
    pp: number;
    accuracy: number;
    playCount: number;
  };
  beatmaps: OsuBeatmap[];
  replays: OsuReplay[];
}

export interface OsuHubClient {
  getWorkspace(): Promise<OsuWorkspaceData>;
}

const workspace: OsuWorkspaceData = {
  player: {
    username: "veryCrunchy",
    globalRank: 42817,
    pp: 6142,
    accuracy: 97.84,
    playCount: 18426,
  },
  beatmaps: [
    {
      id: "b-2354778",
      artist: "Camellia",
      title: "Exit This Earth's Atomosphere",
      difficulty: "Evolution",
      mapper: "Akali",
      stars: 6.42,
      bpm: 191,
      lengthSeconds: 245,
      status: "Ranked",
      source: "osu!",
      localState: "Installed",
      skillsets: ["Aim", "Speed", "Consistency"],
      pp95: 284,
      accuracy: 96.72,
      playCount: 7,
      cover: "linear-gradient(135deg,#381935,#b75367 52%,#efb072)",
    },
    {
      id: "b-4128041",
      artist: "TUYU",
      title: "Under Kids",
      difficulty: "Rain",
      mapper: "Aistre",
      stars: 5.83,
      bpm: 187,
      lengthSeconds: 201,
      status: "Ranked",
      source: "osu!Collector",
      localState: "Not installed",
      skillsets: ["Aim", "Reading"],
      pp95: 226,
      accuracy: null,
      playCount: 0,
      cover: "linear-gradient(135deg,#101b34,#265877 52%,#dd7690)",
    },
    {
      id: "b-3901725",
      artist: "Ado",
      title: "Show",
      difficulty: "Encore",
      mapper: "Lasse",
      stars: 6.11,
      bpm: 138,
      lengthSeconds: 189,
      status: "Ranked",
      source: "osu!",
      localState: "Update available",
      skillsets: ["Finger control", "Reading", "Consistency"],
      pp95: 251,
      accuracy: 95.18,
      playCount: 3,
      cover: "linear-gradient(135deg,#171018,#62405d 50%,#f0566f)",
    },
    {
      id: "b-2819592",
      artist: "Kano",
      title: "Stella-rium",
      difficulty: "Asterism",
      mapper: "Nevo",
      stars: 5.26,
      bpm: 178,
      lengthSeconds: 217,
      status: "Loved",
      source: "osu!Collector",
      localState: "Installed",
      skillsets: ["Aim", "Consistency"],
      pp95: 178,
      accuracy: 98.34,
      playCount: 12,
      cover: "linear-gradient(135deg,#151a3a,#4b5fa0 48%,#c8a4df)",
    },
    {
      id: "b-4483719",
      artist: "Feryquitous",
      title: "Ordirehv",
      difficulty: "Memory",
      mapper: "Garden",
      stars: 6.76,
      bpm: 214,
      lengthSeconds: 153,
      status: "Qualified",
      source: "osu!",
      localState: "Not installed",
      skillsets: ["Speed", "Finger control", "Reading"],
      pp95: 319,
      accuracy: null,
      playCount: 0,
      cover: "linear-gradient(135deg,#111c20,#34665f 48%,#c6ab6d)",
    },
    {
      id: "b-3177096",
      artist: "MIMI feat. KAFU",
      title: "Hanafurashi",
      difficulty: "Bloom",
      mapper: "Kalibe",
      stars: 4.91,
      bpm: 165,
      lengthSeconds: 232,
      status: "Ranked",
      source: "osu!Collector",
      localState: "Installed",
      skillsets: ["Aim", "Reading"],
      pp95: 143,
      accuracy: 99.01,
      playCount: 9,
      cover: "linear-gradient(135deg,#223446,#648ba0 48%,#e8c6c6)",
    },
  ],
  replays: [
    {
      id: "r-9021",
      beatmapId: "b-2354778",
      player: "veryCrunchy",
      score: 847291,
      accuracy: 96.72,
      combo: 824,
      misses: 3,
      mods: ["HD"],
      playedAt: "Today, 19:42",
      pp: 271,
      cursorControl: 84,
      tapControl: 77,
      markers: [
        { time: "01:18", label: "Late correction on jump exit", severity: "watch" },
        { time: "02:47", label: "Cleanest stream section", severity: "good" },
        { time: "03:21", label: "Miss after cursor over-travel", severity: "miss" },
      ],
    },
    {
      id: "r-9014",
      beatmapId: "b-3901725",
      player: "veryCrunchy",
      score: 712480,
      accuracy: 95.18,
      combo: 611,
      misses: 5,
      mods: ["HR"],
      playedAt: "Today, 18:06",
      pp: 229,
      cursorControl: 72,
      tapControl: 81,
      markers: [
        { time: "00:43", label: "Stable alternating pattern", severity: "good" },
        { time: "01:36", label: "Reading delay into slider stack", severity: "watch" },
        { time: "02:51", label: "Two misses after early tap", severity: "miss" },
      ],
    },
    {
      id: "r-8978",
      beatmapId: "b-2819592",
      player: "veryCrunchy",
      score: 962314,
      accuracy: 98.34,
      combo: 1062,
      misses: 1,
      mods: ["HD"],
      playedAt: "Yesterday, 22:11",
      pp: 184,
      cursorControl: 91,
      tapControl: 88,
      markers: [
        { time: "00:58", label: "Strong snap control", severity: "good" },
        { time: "02:12", label: "Aim tightened after break", severity: "good" },
        { time: "03:09", label: "Single miss on wide-angle jump", severity: "miss" },
      ],
    },
  ],
};

export const osuHubClient: OsuHubClient = {
  async getWorkspace() {
    return workspace;
  },
};
