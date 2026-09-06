import { useEffect, useId, useRef, useState } from "react";
import { configureWorkers, createReplaySession, type CoreSession, type SkinAssets } from "replayviewer-js";
import { Play, Pause, RotateCcw, Maximize, Settings2, Volume2 } from "lucide-react";
import { decodeOsuPlayback, fetchPlaybackBytes, osuPlaybackAudioUrl, osuPlaybackBeatmapUrl, playbackTimeLabel } from "../lib/osuPlayback";
import { createAimModPlaybackSkin, disposeAimModPlaybackSkin, playbackSkins, savedPlaybackSkin, loadPlaybackSkin, composePlaybackSkin } from "../lib/osuPlaybackSkin";
import { playbackAnalysis } from "../lib/osuPlaybackAnalysis";
import type { OsuReplayAnalysis } from "../lib/osuCommunity";
import type { ParsedOsuPlayback } from "../lib/osuPlayback";
import "./OsuReplayPlayer.css";
import stretchWorkerUrl from "replayviewer-js/stretch-worker.js?url";

configureWorkers({ stretch: stretchWorkerUrl });

export interface OsuReplayPlayerProps {
  replayUrl: string;
  beatmapId: number;
  beatmapsetId?: number;
  beatmapUrl?: string;
  title?: string;
  audioUrl?: string;
  backgroundUrl?: string;
  seekToMs?: number;
  onTimeChange?: (beatmapMs: number) => void;
  onAnalysis?: (analysis: OsuReplayAnalysis) => void;
  onPlaybackError?: (message: string) => void;
  onVerifiedReplay?: (playback: ParsedOsuPlayback) => void;
}

export function OsuReplayPlayer(props: OsuReplayPlayerProps) {
  // A different play must not inherit a locally-selected beatmap file.
  return <OsuReplayPlayerSession key={`${props.replayUrl}|${props.beatmapId}`} {...props} />;
}

function OsuReplayPlayerSession({ replayUrl, beatmapId, beatmapsetId, beatmapUrl, title = "Replay", audioUrl, backgroundUrl,
  seekToMs, onTimeChange, onAnalysis, onPlaybackError, onVerifiedReplay }: OsuReplayPlayerProps) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const settingsId = useId();
  const root = useRef<HTMLElement>(null);
  const session = useRef<CoreSession | null>(null);
  const context = useRef<AudioContext | null>(null);
  const callback = useRef(onTimeChange);
  callback.current = onTimeChange;
  const analysisCallback = useRef(onAnalysis);
  analysisCallback.current = onAnalysis;
  const errorCallback = useRef(onPlaybackError);
  errorCallback.current = onPlaybackError;
  const verifiedCallback = useRef(onVerifiedReplay);
  verifiedCallback.current = onVerifiedReplay;
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [stage, setStage] = useState("Loading replay and beatmap");
  const [error, setError] = useState("");
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(1);
  const [rate, setRate] = useState(1);
  const [songVolume, setSongVolume] = useState(.65);
  const [hitsoundVolume, setHitsoundVolume] = useState(.35);
  const [audioAvailable, setAudioAvailable] = useState(false);
  const [localMap, setLocalMap] = useState<File | null>(null);
  const [controlError, setControlError] = useState("");
  const [songError, setSongError] = useState("");
  const [showSettings, setShowSettings] = useState(false);
  const [dim, setDim] = useState(.65);
  const [keysVisible, setKeysVisible] = useState(true);
  const [timingVisible, setTimingVisible] = useState(true);
  const [followpointsVisible, setFollowpointsVisible] = useState(true);
  const [judgementsVisible, setJudgementsVisible] = useState(true);
  const [skinId, setSkinId] = useState(savedPlaybackSkin);
  const [skinBusy, setSkinBusy] = useState(false);
  const [skinError, setSkinError] = useState("");
  const [customSkinName, setCustomSkinName] = useState("");
  const [renderQuality, setRenderQuality] = useState(0);
  const renderQualityRef = useRef(renderQuality);
  renderQualityRef.current = renderQuality;
  const swapSkin = useRef<((id: string, file?: File, quality?: number) => Promise<void>) | null>(null);
  const preferences = useRef({ rate, songVolume, hitsoundVolume });
  preferences.current = { rate, songVolume, hitsoundVolume };
  const generation = useRef(0);
  const appliedSeek = useRef<number | undefined>(undefined);

  const pause = () => {
    generation.current++;
    session.current?.player.pause();
    session.current?.audioSync.pause();
    setPlaying(false);
  };

  useEffect(() => {
    const abort = new AbortController();
    let owned: CoreSession | null = null;
    let skin: SkinAssets | null = null;
    let overlay: SkinAssets | null = null;
    let background: ImageBitmap | null = null;
    let frame = 0;
    let switching = false;
    let audio: AudioContext | null = null;
    appliedSeek.current = undefined;
    setSkinBusy(false); setSkinError(""); setSkinId(savedPlaybackSkin());
    setState("loading"); setError(""); setPlaying(false); setPosition(0); setControlError(""); setSongError("");
    setStage("Loading replay and beatmap");
    const alive = () => !abort.signal.aborted;
    const run = async () => {
      const mapSource = localMap
        ? localMap.size <= 4 * 1024 * 1024 ? localMap.arrayBuffer() : Promise.reject(new Error("The beatmap file exceeds the size limit."))
        : fetchPlaybackBytes(beatmapUrl || osuPlaybackBeatmapUrl(beatmapId), 4 * 1024 * 1024, abort.signal);
      const [replayBytes, mapBytes] = await Promise.all([
        fetchPlaybackBytes(replayUrl, 64 * 1024 * 1024, abort.signal), mapSource,
      ]);
      if (!alive()) return;
      setStage("Decoding replay inputs");
      const parsed = await decodeOsuPlayback(replayBytes, mapBytes, abort.signal);
      if (!alive()) return;
      verifiedCallback.current?.(parsed);
      setStage("Loading replay skin");
      audio = new AudioContext(); context.current = audio;
      skin = await createAimModPlaybackSkin();
      if (!alive()) return;
      let song: AudioBuffer | null = null;
      const songUrl = audioUrl || osuPlaybackAudioUrl(beatmapId, beatmapsetId, parsed.replay.beatmapHash);
      setStage("Loading song and skin");
      await Promise.all([
        (async () => {
          try { overlay = await loadPlaybackSkin(savedPlaybackSkin(), audio!, AbortSignal.any([abort.signal, AbortSignal.timeout(12000)])); }
          catch { if (alive()) { setSkinId("classic"); setSkinError("The selected skin is unavailable. AimMod Classic is ready to use."); } }
        })(),
        (async () => {
          if (!songUrl) return;
          try {
            const bytes = await fetchPlaybackBytes(songUrl, 64 * 1024 * 1024, AbortSignal.any([abort.signal, AbortSignal.timeout(60000)]));
            song = await audio!.decodeAudioData(bytes);
          } catch { if (alive()) setSongError("The matching song is unavailable. Replay hitsounds remain enabled."); }
        })(),
        (async () => {
          if (!backgroundUrl) return;
          try {
            const bytes = await fetchPlaybackBytes(backgroundUrl, 8 * 1024 * 1024, AbortSignal.any([abort.signal, AbortSignal.timeout(10000)]));
            background = await createImageBitmap(new Blob([bytes]));
          } catch { /* The play remains watchable without artwork. */ }
        })(),
      ]);
      if (!alive()) return;
      owned = await createReplaySession({ canvas: canvas.current!, audioContext: audio, replay: parsed.replay,
        beatmapSet: { beatmap: parsed.beatmap, songBuffer: song, background, beatmapSounds: new Map() }, skin: composePlaybackSkin(skin, overlay),
        lazerDefaultsUrl: "/playback/aimmod-sounds", userRate: 1 });
      if (!alive()) return;
      session.current = owned;
      analysisCallback.current?.(playbackAnalysis(owned.renderer.hitResults, parsed.beatmap));
      owned.renderer.options.backgroundDim = .65;
      owned.renderer.options.showURBar = true;
      owned.renderer.options.showKeyOverlay = true;
      owned.player.setClockFn(owned.audioSync.clockFn);
      owned.audioSync.setSongVolume(.65); owned.audioSync.setEffectsVolume(.35);
      owned.renderer.start();
      swapSkin.current = async (id, file, quality) => {
        if (!owned || !audio || !skin || switching) return;
        switching = true;
        setSkinBusy(true); setSkinError("");
        const previous = owned;
        const wasPlaying = previous.player.isPlaying;
        const switchToken = ++generation.current;
        previous.player.pause(); previous.audioSync.pause(); setPlaying(false);
        const time = previous.player.currentTimeMs;
        const reuseSkin = quality !== undefined;
        const nextQuality = quality ?? renderQualityRef.current;
        let nextOverlay: SkinAssets | null = null;
        let next: CoreSession | null = null;
        try {
          nextOverlay = reuseSkin ? overlay : await loadPlaybackSkin(id, audio, abort.signal, file);
          if (!alive()) return;
          previous.renderer.stop();
          next = await createReplaySession({ canvas: canvas.current!, audioContext: audio, replay: previous.replay,
            beatmapSet: previous.assets, skin: composePlaybackSkin(skin, nextOverlay),
            lazerDefaultsUrl: "/playback/aimmod-sounds", userRate: preferences.current.rate,
            pageZoom: nextQuality ? nextQuality / Math.max(1, window.devicePixelRatio || 1) : 1 });
          if (!alive()) return;
          Object.assign(next.renderer.options, previous.renderer.options);
          next.player.setClockFn(next.audioSync.clockFn);
          next.audioSync.setSongVolume(preferences.current.songVolume);
          next.audioSync.setEffectsVolume(preferences.current.hitsoundVolume);
          next.player.seek(time);
          previous.destroy();
          if (overlay && !reuseSkin) disposeAimModPlaybackSkin(overlay);
          overlay = nextOverlay; nextOverlay = null;
          owned = next; session.current = next; next = null;
          owned.renderer.start();
          setRenderQuality(nextQuality);
          if (!reuseSkin) { setSkinId(id); setCustomSkinName(file?.name.replace(/\.osk$/i, "") ?? ""); }
          if (id !== "custom") { try { localStorage.setItem("osu-replay-skin", id); } catch { /* Optional preference. */ } }
          if (wasPlaying && !document.hidden && switchToken === generation.current) {
            await owned.audioSync.playFrom(time);
            if (alive() && !document.hidden && switchToken === generation.current) { owned.player.play(); setPlaying(true); } else { owned.audioSync.pause(); }
          }
        } catch {
          if (alive()) { owned?.renderer.start(); setSkinError("This skin could not be opened. Your current skin is still available."); }
        } finally {
          next?.destroy();
          if (nextOverlay && !reuseSkin) disposeAimModPlaybackSkin(nextOverlay);
          switching = false;
          if (alive()) setSkinBusy(false);
        }
      };
      setRenderQuality(0); setRate(1); setSongVolume(.65); setHitsoundVolume(.35); setDim(.65); setKeysVisible(true); setTimingVisible(true); setFollowpointsVisible(true); setJudgementsVisible(true); setDuration(owned.player.durationMs); setAudioAvailable(song !== null); setState("ready");
      let last = 0;
      const tick = (time: number) => {
        if (!alive() || !owned) return;
        const current = owned.player.currentTimeMs;
        if (current >= owned.player.durationMs) { owned.player.pause(); owned.audioSync.pause(); setPlaying(false); }
        if (time - last > 80) { setPosition(current); callback.current?.(owned.timeMapper.toMapTime(current)); last = time; }
        frame = requestAnimationFrame(tick);
      };
      frame = requestAnimationFrame(tick);
    };
    const hidden = () => {
      if (document.hidden) {
        generation.current++; owned?.player.pause(); owned?.audioSync.pause(); setPlaying(false);
        void audio?.suspend().catch(() => undefined);
      }
    };
    document.addEventListener("visibilitychange", hidden);
    void run().catch(reason => {
      if (alive()) { const message = reason instanceof Error ? reason.message : "This replay could not be opened."; setError(message); setState("error"); errorCallback.current?.(message); }
    }).finally(() => {
      if (!alive()) { owned?.audioSync.pause(); owned?.destroy(); if (skin) { disposeAimModPlaybackSkin(skin); skin = null; } if (overlay) { disposeAimModPlaybackSkin(overlay); overlay = null; } background?.close(); background = null; }
    });
    return () => {
      abort.abort(); generation.current++; cancelAnimationFrame(frame); document.removeEventListener("visibilitychange", hidden);
      swapSkin.current = null;
      owned?.player.pause(); owned?.audioSync.pause(); owned?.destroy();
      if (skin) { disposeAimModPlaybackSkin(skin); skin = null; }
      if (overlay) { disposeAimModPlaybackSkin(overlay); overlay = null; }
      background?.close(); background = null;
      if (audio && audio.state !== "closed") void audio.close().catch(() => undefined);
      if (session.current === owned) session.current = null;
      if (context.current === audio) context.current = null;
    };
  }, [replayUrl, beatmapId, beatmapsetId, beatmapUrl, audioUrl, backgroundUrl, localMap, attempt]);

  const seek = async (time: number) => {
    const current = session.current;
    if (!current) return;
    const token = ++generation.current;
    current.player.seek(time); setPosition(time);
    await current.audioSync.seekTo(time);
    if (token !== generation.current || session.current !== current) return;
    current.player.seek(time);
  };

  useEffect(() => {
    const current = session.current;
    if (state === "ready" && !skinBusy && current && seekToMs != null && seekToMs !== appliedSeek.current && Number.isFinite(seekToMs)) {
      appliedSeek.current = seekToMs;
      void seek(Math.max(0, Math.min(current.player.durationMs, (seekToMs - current.introOffsetMs) / current.speed)))
        .catch(() => setControlError("This replay could not seek to that moment."));
    }
  }, [seekToMs, state, skinBusy]);

  const toggle = async () => {
    const current = session.current;
    if (!current) return;
    if (current.player.isPlaying) { pause(); return; }
    const token = ++generation.current;
    try {
      const start = current.player.currentTimeMs >= current.player.durationMs ? 0 : current.player.currentTimeMs;
      await context.current?.resume();
      await current.audioSync.playFrom(start);
      if (token !== generation.current || session.current !== current) return;
      current.player.seek(start); current.player.play(); setPlaying(true); setControlError("");
    } catch { if (session.current === current) setControlError("Playback could not start. Try Play again."); }
  };

  return <section className="osu-replay-player" ref={root} aria-label={`${title} playback`} data-state={state}>
    <div className="osu-replay-player__stage">
      <canvas ref={canvas} aria-label="osu! replay playfield" />
      {state !== "ready" ? <div className="osu-replay-player__overlay" role={state === "error" ? "alert" : "status"}>
        <strong>{state === "error" ? "Playback unavailable" : stage}</strong>
        {state === "loading" ? <progress aria-label={stage} /> : <><p>{error}</p><button onClick={() => setAttempt(value => value + 1)}>Try again</button>
          <label className="osu-replay-player__file">Choose original .osu<input type="file" accept=".osu" onChange={event => setLocalMap(event.target.files?.[0] ?? null)} /></label></>}
      </div> : null}
    </div>
    <div className="osu-replay-player__controls">
      <div className="osu-replay-player__transport">
        <button type="button" className="osu-replay-player__icon" disabled={state !== "ready" || skinBusy} onClick={() => void toggle()} aria-label={playing ? "Pause replay" : "Play replay"} title={playing ? "Pause replay" : "Play replay"}>{playing ? <Pause size={18} /> : <Play size={18} />}</button>
        <button type="button" className="osu-replay-player__icon" aria-label="Restart replay" disabled={state !== "ready" || skinBusy} onClick={() => { pause(); void seek(0).catch(() => setControlError("Could not restart replay.")); }} title="Restart replay"><RotateCcw size={17} /></button>
        <span className="osu-replay-player__time"><output aria-label="Replay position">{playbackTimeLabel(position)}</output> / {playbackTimeLabel(duration)}</span>
        <label className="osu-replay-player__speed"><span>Speed</span><select value={rate} disabled={state !== "ready" || skinBusy} onChange={event => {
          const value = Number(event.target.value); setRate(value);
          const current = session.current;
          if (current) { const time = current.player.currentTimeMs; current.audioSync.setUserRate(value); void seek(time).catch(() => setControlError("Could not change playback speed.")); }
        }}>{[.25, .5, .75, 1, 1.25, 1.5, 2].map(value => <option key={value} value={value}>{value}x</option>)}</select></label>
        <button type="button" className="osu-replay-player__fullscreen osu-replay-player__icon" aria-label="Fullscreen replay" title="Fullscreen replay" onClick={() => void (document.fullscreenElement ? document.exitFullscreen() : root.current?.requestFullscreen())?.catch(() => setControlError("Fullscreen is unavailable in this browser."))}><Maximize size={17} /></button>
      </div>
      <input aria-label="Replay timeline" className="osu-replay-player__timeline" type="range" min="0" max={duration} step="10" value={Math.min(position, duration)} disabled={state !== "ready" || skinBusy}
        onChange={event => void seek(Number(event.target.value)).catch(() => setControlError("Could not seek replay."))} />
      <div className="osu-replay-player__sound">
        <span>{audioAvailable ? "Song + hitsounds" : "Hitsounds only"}</span>
        <label><Volume2 size={16} aria-hidden="true" />Song<input aria-label="Song volume" type="range" min="0" max="1" step="0.05" value={songVolume} onChange={event => {
          const value = Number(event.target.value); setSongVolume(value); session.current?.audioSync.setSongVolume(value);
        }} /></label>
        <label>Hitsounds<input aria-label="Hitsound volume" type="range" min="0" max="1" step="0.05" value={hitsoundVolume} onChange={event => {
          const value = Number(event.target.value); setHitsoundVolume(value); session.current?.audioSync.setEffectsVolume(value);
        }} /></label>
        <button type="button" className="osu-replay-player__settings-button osu-replay-player__icon" aria-label="Replay display settings" title="Replay display settings" aria-expanded={showSettings} aria-controls={settingsId} onClick={() => setShowSettings(value => !value)}><Settings2 size={17} /></button>
      </div>
      <div className="osu-replay-player__skins">
        <label>Skin <select aria-label="Replay skin" value={skinId} disabled={state !== "ready" || skinBusy} onChange={event => void swapSkin.current?.(event.target.value)}>
          {playbackSkins.map(skin => <option key={skin.id} value={skin.id}>{skin.name}</option>)}
          {skinId === "custom" ? <option value="custom">{customSkinName || "Custom skin"}</option> : null}
        </select></label>
        {playbackSkins.find(skin => skin.id === skinId)?.source ? <a href={playbackSkins.find(skin => skin.id === skinId)!.source} target="_blank" rel="noreferrer">by {playbackSkins.find(skin => skin.id === skinId)!.creator}</a> : null}
        <label className="osu-replay-player__file">Choose .osk<input type="file" accept=".osk,.zip" aria-label="Choose custom skin" disabled={state !== "ready" || skinBusy} onChange={event => { const file = event.target.files?.[0]; event.target.value = ""; if (file) void swapSkin.current?.("custom", file); }} /></label>
        {skinBusy ? <span role="status">Loading skin…</span> : null}
      </div>
      {skinError ? <p role="status" className="osu-replay-player__notice">{skinError}</p> : null}
      {showSettings ? <div id={settingsId} className="osu-replay-player__settings">
        <label>Render detail <select aria-label="Render detail" value={renderQuality} disabled={state !== "ready" || skinBusy} onChange={event => void swapSkin.current?.(skinId, undefined, Number(event.target.value))}>
          <option value={0}>Automatic</option><option value={1}>720p</option><option value={1.5}>1080p</option><option value={2}>1440p</option>
        </select></label>
        <label>Background dim <output>{Math.round(dim * 100)}%</output><input aria-label="Background dim" type="range" min="0" max="1" step="0.05" value={dim} onChange={event => { const value = Number(event.target.value); setDim(value); if (session.current) session.current.renderer.options.backgroundDim = value; }} /></label>
        <label><input type="checkbox" checked={followpointsVisible} onChange={event => { setFollowpointsVisible(event.target.checked); if (session.current) session.current.renderer.options.showFollowpoints = event.target.checked; }} /> Follow points</label>
        <label><input type="checkbox" checked={judgementsVisible} onChange={event => { setJudgementsVisible(event.target.checked); if (session.current) session.current.renderer.options.showJudgement = event.target.checked; }} /> Hit results</label>
        <label><input type="checkbox" checked={keysVisible} onChange={event => { setKeysVisible(event.target.checked); if (session.current) session.current.renderer.options.showKeyOverlay = event.target.checked; }} /> Key presses</label>
        <label><input type="checkbox" checked={timingVisible} onChange={event => { setTimingVisible(event.target.checked); if (session.current) session.current.renderer.options.showURBar = event.target.checked; }} /> Hit timing</label>
      </div> : null}
      {controlError ? <p role="status" className="osu-replay-player__notice">{controlError}</p> : null}
      {songError ? <p role="status" className="osu-replay-player__notice">{songError}</p> : null}
    </div>
  </section>;
}
