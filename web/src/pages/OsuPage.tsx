import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Button } from "../components/ui/Button";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { Helmet } from "../lib/helmet";
import {
  osuHubClient,
  type OsuBeatmap,
  type OsuReplay,
  type OsuSkillset,
  type OsuWorkspaceData,
} from "../lib/osuHub";

type WorkspaceView = "beatmaps" | "replays" | "performance" | "coaching";
type SortMode = "recommended" | "stars-asc" | "stars-desc" | "pp";

const VIEWS: Array<{ id: WorkspaceView; label: string }> = [
  { id: "beatmaps", label: "Beatmaps" },
  { id: "replays", label: "Replays" },
  { id: "performance", label: "Performance" },
  { id: "coaching", label: "Coaching" },
];

const SKILLSETS: Array<"All" | OsuSkillset> = ["All", "Aim", "Speed", "Reading", "Consistency", "Finger control"];

function formatLength(seconds: number) {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

function formatScore(score: number) {
  return score.toLocaleString();
}

function Metric({ label, value, accent = false }: { label: string; value: string; accent?: boolean }) {
  return (
    <div className="min-w-0">
      <div className="text-[9px] uppercase tracking-[0.08em] text-muted-2">{label}</div>
      <div className={`mt-1 truncate text-[14px] tabular-nums ${accent ? "text-[#ff9bc7]" : "text-text"}`}>{value}</div>
    </div>
  );
}

function FieldLabel({ children }: { children: ReactNode }) {
  return <span className="mb-1.5 block text-[9px] uppercase tracking-[0.09em] text-muted-2">{children}</span>;
}

function BeatmapExplorer({ beatmaps }: { beatmaps: OsuBeatmap[] }) {
  const [query, setQuery] = useState("");
  const [skillset, setSkillset] = useState<"All" | OsuSkillset>("All");
  const [source, setSource] = useState<"All" | OsuBeatmap["source"]>("All");
  const [library, setLibrary] = useState<"All" | "Installed" | "Missing">("All");
  const [maxStars, setMaxStars] = useState(7);
  const [sort, setSort] = useState<SortMode>("recommended");
  const [selectedId, setSelectedId] = useState(beatmaps[0]?.id ?? "");
  const [queuedIds, setQueuedIds] = useState<Set<string>>(() => new Set());

  const visibleBeatmaps = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    const next = beatmaps.filter((beatmap) => {
      const searchText = `${beatmap.artist} ${beatmap.title} ${beatmap.difficulty} ${beatmap.mapper} ${beatmap.skillsets.join(" ")}`.toLowerCase();
      return (!normalizedQuery || searchText.includes(normalizedQuery))
        && (skillset === "All" || beatmap.skillsets.includes(skillset))
        && (source === "All" || beatmap.source === source)
        && (library === "All" || (library === "Installed" ? beatmap.localState === "Installed" : beatmap.localState !== "Installed"))
        && beatmap.stars <= maxStars;
    });

    return next.sort((a, b) => {
      if (sort === "stars-asc") return a.stars - b.stars;
      if (sort === "stars-desc") return b.stars - a.stars;
      if (sort === "pp") return b.pp95 - a.pp95;
      const aScore = Math.abs(a.stars - 5.8) + (a.localState === "Installed" ? 0.25 : 0);
      const bScore = Math.abs(b.stars - 5.8) + (b.localState === "Installed" ? 0.25 : 0);
      return aScore - bScore;
    });
  }, [beatmaps, library, maxStars, query, skillset, sort, source]);

  const selected = visibleBeatmaps.find((beatmap) => beatmap.id === selectedId) ?? visibleBeatmaps[0] ?? null;

  function toggleQueue(beatmapId: string) {
    setQueuedIds((current) => {
      const next = new Set(current);
      if (next.has(beatmapId)) next.delete(beatmapId);
      else next.add(beatmapId);
      return next;
    });
  }

  function clearFilters() {
    setQuery("");
    setSkillset("All");
    setSource("All");
    setLibrary("All");
    setMaxStars(7);
    setSort("recommended");
  }

  return (
    <div className="grid min-h-[660px] border-x border-b border-line xl:grid-cols-[220px_minmax(380px,0.92fr)_minmax(330px,0.78fr)]">
      <aside className="border-b border-line p-4 xl:border-b-0 xl:border-r" aria-label="Beatmap filters">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-[13px] tracking-[-0.02em]">Filters</h2>
          <button type="button" onClick={clearFilters} className="text-[10px] text-cyan hover:underline">Reset</button>
        </div>

        <div className="mt-5 grid gap-4 max-xl:grid-cols-2 max-sm:grid-cols-1">
          <label>
            <FieldLabel>Source</FieldLabel>
            <select value={source} onChange={(event) => setSource(event.target.value as typeof source)} className="w-full rounded-lg border border-line bg-bg-2 px-3 py-2 text-[11px] text-text outline-none focus:border-cyan/50">
              <option>All</option>
              <option>osu!</option>
              <option>osu!Collector</option>
            </select>
          </label>
          <label>
            <FieldLabel>Local library</FieldLabel>
            <select value={library} onChange={(event) => setLibrary(event.target.value as typeof library)} className="w-full rounded-lg border border-line bg-bg-2 px-3 py-2 text-[11px] text-text outline-none focus:border-cyan/50">
              <option>All</option>
              <option>Installed</option>
              <option>Missing</option>
            </select>
          </label>
          <label>
            <FieldLabel>Maximum stars</FieldLabel>
            <div className="flex items-center gap-3">
              <input
                type="range"
                min="4"
                max="8"
                step="0.1"
                value={maxStars}
                onChange={(event) => setMaxStars(Number(event.target.value))}
                className="min-w-0 flex-1 accent-[#ff66aa]"
              />
              <span className="w-10 text-right text-[11px] tabular-nums text-[#ff9bc7]">{maxStars.toFixed(1)}★</span>
            </div>
          </label>
          <div className="col-span-full">
            <FieldLabel>Skillset</FieldLabel>
            <div className="flex flex-wrap gap-1.5 xl:grid">
              {SKILLSETS.map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => setSkillset(item)}
                  className={`rounded-full border px-2.5 py-1.5 text-left text-[10px] transition-colors ${skillset === item ? "border-[#ff66aa]/50 bg-[#ff66aa]/10 text-[#ff9bc7]" : "border-line text-muted hover:border-line-strong hover:text-text"}`}
                >
                  {item}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="mt-5 border-t border-line pt-4 text-[10px] leading-relaxed text-muted-2">
          {queuedIds.size} {queuedIds.size === 1 ? "beatmap" : "beatmaps"} in import queue
        </div>
      </aside>

      <section className="min-w-0 border-b border-line xl:border-b-0 xl:border-r" aria-label="Beatmap search results">
        <div className="border-b border-line p-3">
          <label className="relative block">
            <span className="sr-only">Search beatmaps</span>
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search artist, title, mapper, difficulty, or skillset"
              className="w-full rounded-xl border border-line bg-black/20 px-4 py-3 pr-24 text-[12px] text-text outline-none placeholder:text-muted-2 focus:border-[#ff66aa]/45"
            />
            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-[9px] uppercase tracking-[0.08em] text-muted-2">{visibleBeatmaps.length} results</span>
          </label>
          <div className="mt-2 flex items-center justify-between gap-3">
            <span className="text-[10px] text-muted-2">Provider results joined with your local library</span>
            <select value={sort} onChange={(event) => setSort(event.target.value as SortMode)} className="rounded-md border border-line bg-bg-2 px-2 py-1.5 text-[10px] text-muted outline-none">
              <option value="recommended">Recommended</option>
              <option value="stars-asc">Stars, low to high</option>
              <option value="stars-desc">Stars, high to low</option>
              <option value="pp">PP at 95%</option>
            </select>
          </div>
        </div>

        <div className="hub-scroll max-h-[760px] overflow-y-auto">
          {visibleBeatmaps.length ? visibleBeatmaps.map((beatmap) => (
            <button
              key={beatmap.id}
              type="button"
              onClick={() => setSelectedId(beatmap.id)}
              className={`grid w-full grid-cols-[58px_minmax(0,1fr)_auto] gap-3 border-b border-line px-3 py-3 text-left transition-colors ${selected?.id === beatmap.id ? "bg-[#ff66aa]/7" : "hover:bg-white/2"}`}
            >
              <span className="block h-14 rounded-lg border border-white/8" style={{ background: beatmap.cover }} aria-hidden />
              <span className="min-w-0 self-center">
                <strong className="block truncate text-[12px] font-medium text-text">{beatmap.artist} · {beatmap.title}</strong>
                <span className="mt-1 block truncate text-[10px] text-muted">[{beatmap.difficulty}] mapped by {beatmap.mapper}</span>
                <span className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[9px] text-muted-2">
                  <span>{beatmap.bpm} BPM</span><span>{formatLength(beatmap.lengthSeconds)}</span><span>{beatmap.source}</span><span>{beatmap.localState}</span>
                </span>
              </span>
              <span className="self-center text-right">
                <strong className="block text-[13px] tabular-nums text-[#ff9bc7]">{beatmap.stars.toFixed(2)}★</strong>
                <span className="mt-1 block text-[9px] tabular-nums text-muted">{beatmap.pp95}pp</span>
              </span>
            </button>
          )) : (
            <div className="px-5 py-12 text-center">
              <div className="text-[13px] text-text">No beatmaps match these filters.</div>
              <button type="button" onClick={clearFilters} className="mt-2 text-[11px] text-cyan hover:underline">Clear filters</button>
            </div>
          )}
        </div>
      </section>

      <aside className="min-w-0 p-4" aria-label="Selected beatmap details">
        {selected ? (
          <>
            <div className="h-36 rounded-xl border border-white/8" style={{ background: selected.cover }}>
              <div className="flex h-full items-end bg-gradient-to-t from-black/75 to-transparent p-4">
                <div className="min-w-0">
                  <div className="text-[10px] text-[#ffb3d4]">{selected.artist}</div>
                  <h2 className="mt-1 truncate text-[18px] tracking-[-0.035em]">{selected.title}</h2>
                  <p className="mt-1 truncate text-[11px] text-white/65">{selected.difficulty} · {selected.mapper}</p>
                </div>
              </div>
            </div>

            <div className="grid grid-cols-4 gap-3 border-b border-line py-4 max-sm:grid-cols-2">
              <Metric label="Stars" value={`${selected.stars.toFixed(2)}★`} accent />
              <Metric label="BPM" value={String(selected.bpm)} />
              <Metric label="Length" value={formatLength(selected.lengthSeconds)} />
              <Metric label="95% PP" value={`${selected.pp95}pp`} />
            </div>

            <div className="py-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <FieldLabel>Skill profile</FieldLabel>
                  <div className="flex flex-wrap gap-1.5">
                    {selected.skillsets.map((item) => <span key={item} className="rounded-full border border-line px-2 py-1 text-[9px] text-muted">{item}</span>)}
                  </div>
                </div>
                <span className="shrink-0 rounded-full border border-mint/25 bg-mint/6 px-2.5 py-1 text-[9px] text-mint">{selected.status}</span>
              </div>

              <dl className="mt-5 divide-y divide-line border-y border-line text-[11px]">
                <div className="flex items-center justify-between gap-4 py-2.5"><dt className="text-muted">Local library</dt><dd className="m-0 text-text">{selected.localState}</dd></div>
                <div className="flex items-center justify-between gap-4 py-2.5"><dt className="text-muted">Your best accuracy</dt><dd className="m-0 tabular-nums text-text">{selected.accuracy ? `${selected.accuracy.toFixed(2)}%` : "No score"}</dd></div>
                <div className="flex items-center justify-between gap-4 py-2.5"><dt className="text-muted">Plays</dt><dd className="m-0 tabular-nums text-text">{selected.playCount}</dd></div>
                <div className="flex items-center justify-between gap-4 py-2.5"><dt className="text-muted">Provider</dt><dd className="m-0 text-text">{selected.source}</dd></div>
              </dl>
            </div>

            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="primary" onClick={() => toggleQueue(selected.id)}>
                {queuedIds.has(selected.id) ? "Remove from queue" : selected.localState === "Installed" ? "Reimport beatmap" : "Add to import queue"}
              </Button>
              <Button type="button" onClick={() => setSkillset(selected.skillsets[0])}>Similar maps</Button>
            </div>
            {queuedIds.has(selected.id) ? <p className="mt-3 text-[10px] leading-relaxed text-mint">Queued. AimMod will download this map and hand it to your linked osu!lazer client.</p> : null}
          </>
        ) : (
          <div className="flex min-h-72 items-center justify-center text-[12px] text-muted">Select a beatmap to inspect it.</div>
        )}
      </aside>
    </div>
  );
}

function ReplayWorkspace({ replays, beatmaps }: { replays: OsuReplay[]; beatmaps: OsuBeatmap[] }) {
  const [selectedId, setSelectedId] = useState(replays[0]?.id ?? "");
  const selected = replays.find((replay) => replay.id === selectedId) ?? replays[0];
  const selectedMap = beatmaps.find((beatmap) => beatmap.id === selected?.beatmapId);

  return (
    <div className="grid min-h-[620px] border-x border-b border-line lg:grid-cols-[minmax(280px,0.38fr)_minmax(0,0.62fr)]">
      <section className="border-b border-line lg:border-b-0 lg:border-r" aria-label="Replay library">
        <div className="border-b border-line px-4 py-3">
          <div className="text-[9px] uppercase tracking-[0.09em] text-muted-2">Replay library</div>
          <h2 className="mt-1 text-[14px]">Recent local plays</h2>
        </div>
        {replays.map((replay) => {
          const beatmap = beatmaps.find((item) => item.id === replay.beatmapId);
          return (
            <button key={replay.id} type="button" onClick={() => setSelectedId(replay.id)} className={`w-full border-b border-line px-4 py-4 text-left ${selected?.id === replay.id ? "bg-[#ff66aa]/7" : "hover:bg-white/2"}`}>
              <strong className="block truncate text-[12px] font-medium">{beatmap?.title}</strong>
              <span className="mt-1 block truncate text-[10px] text-muted">{beatmap?.difficulty} · {replay.mods.join("") || "NM"}</span>
              <span className="mt-3 flex items-center justify-between gap-3 text-[10px]">
                <span className="tabular-nums text-text">{replay.accuracy.toFixed(2)}%</span>
                <span className="text-muted-2">{replay.playedAt}</span>
              </span>
            </button>
          );
        })}
      </section>

      {selected && selectedMap ? (
        <section className="min-w-0 p-4 md:p-5" aria-label="Replay analysis">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="min-w-0">
              <div className="text-[10px] text-[#ff9bc7]">{selectedMap.artist}</div>
              <h2 className="mt-1 truncate text-[clamp(18px,3vw,28px)] tracking-[-0.04em]">{selectedMap.title}</h2>
              <p className="mt-1 text-[11px] text-muted">{selectedMap.difficulty} · {selected.playedAt}</p>
            </div>
            <span className="rounded-full border border-[#ff66aa]/25 bg-[#ff66aa]/8 px-3 py-1.5 text-[10px] text-[#ff9bc7]">{selected.mods.join("") || "NM"}</span>
          </div>

          <div className="mt-5 grid grid-cols-3 gap-x-4 gap-y-5 border-y border-line py-4 sm:grid-cols-6">
            <Metric label="Score" value={formatScore(selected.score)} />
            <Metric label="Accuracy" value={`${selected.accuracy.toFixed(2)}%`} accent />
            <Metric label="Combo" value={`${selected.combo}x`} />
            <Metric label="Misses" value={String(selected.misses)} />
            <Metric label="PP" value={`${selected.pp}pp`} />
            <Metric label="Player" value={selected.player} />
          </div>

          <div className="mt-6">
            <div className="flex items-center justify-between gap-4">
              <div>
                <div className="text-[9px] uppercase tracking-[0.09em] text-muted-2">Replay timeline</div>
                <h3 className="mt-1 text-[13px]">Cursor and tap stability</h3>
              </div>
              <div className="flex gap-3 text-[9px] text-muted"><span>Cursor {selected.cursorControl}%</span><span>Tap {selected.tapControl}%</span></div>
            </div>
            <div className="mt-4 flex h-28 items-end gap-1 border-b border-line px-1">
              {[63, 74, 71, 82, 86, 78, 91, 84, 80, 68, 76, 88, 93, 85, 79, 72, 89, 83, 77, 90, 86, 69, 81, 87].map((value, index) => (
                <span key={index} className="min-w-0 flex-1 rounded-t-sm bg-[linear-gradient(180deg,#ff78b4,rgba(121,201,151,0.35))] opacity-75" style={{ height: `${value}%` }} />
              ))}
            </div>
          </div>

          <div className="mt-6">
            <div className="text-[9px] uppercase tracking-[0.09em] text-muted-2">Review markers</div>
            <div className="mt-2 divide-y divide-line border-y border-line">
              {selected.markers.map((marker) => (
                <div key={`${selected.id}-${marker.time}`} className="grid grid-cols-[52px_8px_1fr] items-center gap-3 py-3 text-[11px]">
                  <span className="tabular-nums text-muted">{marker.time}</span>
                  <span className={`h-2 w-2 rounded-full ${marker.severity === "good" ? "bg-mint" : marker.severity === "watch" ? "bg-gold" : "bg-danger"}`} />
                  <span className="text-text">{marker.label}</span>
                </div>
              ))}
            </div>
          </div>
        </section>
      ) : null}
    </div>
  );
}

function PerformanceWorkspace({ data }: { data: OsuWorkspaceData }) {
  const skills = [
    ["Aim", 84, "+3.8%"], ["Speed", 72, "+1.2%"], ["Reading", 68, "+5.1%"], ["Consistency", 79, "+2.4%"], ["Finger control", 65, "+0.7%"],
  ] as const;

  return (
    <div className="grid border-x border-b border-line lg:grid-cols-[minmax(0,0.6fr)_minmax(300px,0.4fr)]">
      <section className="border-b border-line p-5 lg:border-b-0 lg:border-r md:p-6">
        <div className="text-[9px] uppercase tracking-[0.09em] text-muted-2">Skill profile</div>
        <h2 className="mt-1 text-[22px] tracking-[-0.04em]">Last 30 days</h2>
        <div className="mt-6 grid gap-5">
          {skills.map(([label, value, delta]) => (
            <div key={label}>
              <div className="mb-2 flex items-center justify-between gap-4 text-[11px]"><span>{label}</span><span className="tabular-nums text-mint">{delta}</span></div>
              <div className="h-2 overflow-hidden rounded-full bg-white/5"><div className="h-full rounded-full bg-[linear-gradient(90deg,rgba(121,201,151,0.45),#ff78b4)]" style={{ width: `${value}%` }} /></div>
            </div>
          ))}
        </div>
      </section>
      <section className="p-5 md:p-6">
        <div className="text-[9px] uppercase tracking-[0.09em] text-muted-2">Player summary</div>
        <h2 className="mt-1 text-[22px] tracking-[-0.04em]">{data.player.username}</h2>
        <div className="mt-5 grid grid-cols-2 gap-x-5 gap-y-6 border-y border-line py-5">
          <Metric label="Global rank" value={`#${data.player.globalRank.toLocaleString()}`} accent />
          <Metric label="Performance" value={`${data.player.pp.toLocaleString()}pp`} />
          <Metric label="Accuracy" value={`${data.player.accuracy.toFixed(2)}%`} />
          <Metric label="Play count" value={data.player.playCount.toLocaleString()} />
        </div>
        <div className="mt-6">
          <div className="text-[9px] uppercase tracking-[0.09em] text-muted-2">Recent PP</div>
          <div className="mt-3 flex h-36 items-end gap-2 border-b border-line">
            {[61, 58, 70, 67, 73, 76, 71, 82, 78, 86, 89, 84].map((value, index) => <span key={index} className="flex-1 rounded-t bg-mint/55" style={{ height: `${value}%` }} />)}
          </div>
        </div>
      </section>
    </div>
  );
}

function CoachingWorkspace({ beatmaps }: { beatmaps: OsuBeatmap[] }) {
  const [saved, setSaved] = useState(false);
  const sessionMaps = [beatmaps[5], beatmaps[3], beatmaps[1]].filter(Boolean);

  return (
    <div className="grid border-x border-b border-line lg:grid-cols-[minmax(0,0.58fr)_minmax(300px,0.42fr)]">
      <section className="border-b border-line p-5 lg:border-b-0 lg:border-r md:p-6">
        <div className="text-[9px] uppercase tracking-[0.09em] text-[#ff9bc7]">Today's focus</div>
        <h2 className="mt-2 max-w-[18ch] text-[clamp(24px,4vw,42px)] leading-[0.98] tracking-[-0.05em]">Stabilise wide jumps after dense reading sections.</h2>
        <p className="mt-4 max-w-[700px] text-[13px] leading-relaxed text-muted">Your cursor control stays strong in isolated aim patterns, then drops after stacks and short streams. This session alternates readable aim maps with one denser transition map so the correction is practiced without turning the whole block into a speed session.</p>

        <div className="mt-6 grid grid-cols-3 gap-3 border-y border-line py-4 max-sm:grid-cols-1">
          <Metric label="Session length" value="28 min" />
          <Metric label="Target difficulty" value="5.1–5.9★" accent />
          <Metric label="Confidence" value="High" />
        </div>
      </section>
      <section className="p-5 md:p-6">
        <div className="flex items-center justify-between gap-4">
          <div><div className="text-[9px] uppercase tracking-[0.09em] text-muted-2">Practice session</div><h2 className="mt-1 text-[16px]">Three-map block</h2></div>
          <Button type="button" variant={saved ? "secondary" : "primary"} onClick={() => setSaved((value) => !value)}>{saved ? "Saved" : "Save session"}</Button>
        </div>
        <ol className="mt-5 divide-y divide-line border-y border-line">
          {sessionMaps.map((beatmap, index) => (
            <li key={beatmap.id} className="grid grid-cols-[30px_minmax(0,1fr)_auto] items-center gap-3 py-4">
              <span className="text-[10px] text-[#ff9bc7]">0{index + 1}</span>
              <span className="min-w-0"><strong className="block truncate text-[11px] font-medium">{beatmap.title}</strong><span className="mt-1 block truncate text-[9px] text-muted">{beatmap.difficulty} · {index === 0 ? "Warm up" : index === 1 ? "Build control" : "Apply under load"}</span></span>
              <span className="text-[10px] tabular-nums text-muted">{beatmap.stars.toFixed(2)}★</span>
            </li>
          ))}
        </ol>
        {saved ? <p className="mt-3 text-[10px] text-mint">Session saved to your osu!lazer workspace.</p> : null}
      </section>
    </div>
  );
}

export function OsuPage() {
  const [data, setData] = useState<OsuWorkspaceData | null>(null);
  const [view, setView] = useState<WorkspaceView>("beatmaps");

  useEffect(() => {
    let cancelled = false;
    void osuHubClient.getWorkspace().then((next) => {
      if (!cancelled) setData(next);
    });
    return () => { cancelled = true; };
  }, []);

  return (
    <PageStack className="gap-0">
      <Helmet>
        <title>osu!lazer workspace · AimMod Hub</title>
        <meta name="description" content="Search beatmaps, manage replays, inspect performance, and build practice sessions in the AimMod osu!lazer workspace." />
      </Helmet>

      <PageSection className="rounded-b-none border-[#ff66aa]/20 bg-[radial-gradient(circle_at_10%_0%,rgba(255,102,170,0.15),transparent_26%),linear-gradient(120deg,rgba(18,8,14,0.98),rgba(4,13,9,0.98)_52%)] px-4 py-5 md:px-6">
        <div className="flex flex-wrap items-end justify-between gap-5">
          <div>
            <div className="text-[10px] uppercase tracking-[0.11em] text-[#ff9bc7]">AimMod · osu!lazer</div>
            <h1 className="mt-1 text-[clamp(25px,4vw,42px)] leading-none tracking-[-0.05em]">{data?.player.username ?? "osu! workspace"}</h1>
            <p className="mt-2 text-[11px] text-muted">Beatmaps, replays, performance, and practice in one client.</p>
          </div>
          {data ? (
            <div className="grid grid-cols-4 gap-x-6 gap-y-3 max-sm:grid-cols-2">
              <Metric label="Global" value={`#${data.player.globalRank.toLocaleString()}`} />
              <Metric label="PP" value={data.player.pp.toLocaleString()} accent />
              <Metric label="Accuracy" value={`${data.player.accuracy.toFixed(2)}%`} />
              <Metric label="Plays" value={data.player.playCount.toLocaleString()} />
            </div>
          ) : null}
        </div>
      </PageSection>

      <nav className="hub-scroll flex overflow-x-auto border-x border-b border-line bg-panel-strong px-2" aria-label="osu! workspace">
        {VIEWS.map((item) => (
          <button key={item.id} type="button" onClick={() => setView(item.id)} className={`shrink-0 border-b-2 px-4 py-3 text-[11px] transition-colors ${view === item.id ? "border-[#ff66aa] text-text" : "border-transparent text-muted hover:text-text"}`}>
            {item.label}
          </button>
        ))}
        <Button to="/community" className="ml-auto my-1.5 shrink-0">KovaaK's Hub</Button>
      </nav>

      {!data ? (
        <div className="border-x border-b border-line px-5 py-16 text-center text-[12px] text-muted">Loading osu! workspace...</div>
      ) : view === "beatmaps" ? (
        <BeatmapExplorer beatmaps={data.beatmaps} />
      ) : view === "replays" ? (
        <ReplayWorkspace replays={data.replays} beatmaps={data.beatmaps} />
      ) : view === "performance" ? (
        <PerformanceWorkspace data={data} />
      ) : (
        <CoachingWorkspace beatmaps={data.beatmaps} />
      )}
    </PageStack>
  );
}
