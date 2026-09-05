import { useEffect, useId, useRef, useState } from "react";
import { createReplaySession, type CoreSession, type SkinAssets } from "replayviewer-js";
import { Play, Pause, RotateCcw, Maximize, Settings2, Volume2 } from "lucide-react";
import { decodeOsuPlayback, fetchPlaybackBytes, osuPlaybackAudioUrl, osuPlaybackBeatmapUrl, playbackTimeLabel } from "../lib/osuPlayback";
import { createAimModPlaybackSkin, disposeAimModPlaybackSkin } from "../lib/osuPlaybackSkin";
import { playbackAnalysis } from "../lib/osuPlaybackAnalysis";
import type { OsuReplayAnalysis } from "../lib/osuCommunity";
import type { ParsedOsuPlayback } from "../lib/osuPlayback";
import "./OsuReplayPlayer.css";

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
  const generation = useRef(0);

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
    let background: ImageBitmap | null = null;
    let frame = 0;
    let audio: AudioContext | null = null;
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
      setStage("Preparing playback");
      audio = new AudioContext(); context.current = audio;
      skin = await createAimModPlaybackSkin();
      let song: AudioBuffer | null = null;
      const songUrl = audioUrl || osuPlaybackAudioUrl(beatmapId, beatmapsetId, parsed.replay.beatmapHash);
      if (songUrl) {
        setStage("Loading song audio");
        try {
          const bytes = await fetchPlaybackBytes(songUrl, 64 * 1024 * 1024, AbortSignal.any([abort.signal, AbortSignal.timeout(60000)]));
          song = await audio.decodeAudioData(bytes);
        } catch { if (alive()) setSongError("The matching song is unavailable. Replay hitsounds remain enabled."); }
      }
      if (backgroundUrl) {
        try {
          const bytes = await fetchPlaybackBytes(backgroundUrl, 8 * 1024 * 1024, AbortSignal.any([abort.signal, AbortSignal.timeout(10000)]));
          background = await createImageBitmap(new Blob([bytes]));
        } catch { /* The play remains watchable without artwork. */ }
      }
      if (!alive()) return;
      owned = await createReplaySession({ canvas: canvas.current!, audioContext: audio, replay: parsed.replay,
        beatmapSet: { beatmap: parsed.beatmap, songBuffer: song, background, beatmapSounds: new Map() }, skin,
        lazerDefaultsUrl: "/playback/aimmod-sounds", userRate: 1 });
      if (!alive()) { owned.audioSync.pause(); owned.destroy(); return; }
      session.current = owned;
      analysisCallback.current?.(playbackAnalysis(owned.renderer.hitResults, parsed.beatmap));
      owned.renderer.options.backgroundDim = .65;
      owned.renderer.options.showURBar = true;
      owned.renderer.options.showKeyOverlay = true;
      owned.player.setClockFn(owned.audioSync.clockFn);
      owned.audioSync.setSongVolume(.65); owned.audioSync.setEffectsVolume(.35);
      owned.renderer.start();
      setRate(1); setSongVolume(.65); setHitsoundVolume(.35); setDim(.65); setKeysVisible(true); setTimingVisible(true); setDuration(owned.player.durationMs); setAudioAvailable(song !== null); setState("ready");
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
      if (!alive()) { owned?.audioSync.pause(); owned?.destroy(); if (skin) disposeAimModPlaybackSkin(skin); background?.close(); }
    });
    return () => {
      abort.abort(); generation.current++; cancelAnimationFrame(frame); document.removeEventListener("visibilitychange", hidden);
      owned?.player.pause(); owned?.audioSync.pause(); owned?.destroy();
      if (skin) disposeAimModPlaybackSkin(skin);
      background?.close();
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
    if (state === "ready" && current && seekToMs != null && Number.isFinite(seekToMs)) {
      void seek(Math.max(0, Math.min(current.player.durationMs, (seekToMs - current.introOffsetMs) / current.speed)))
        .catch(() => setControlError("This replay could not seek to that moment."));
    }
  }, [seekToMs, state]);

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
        <button type="button" className="osu-replay-player__icon" disabled={state !== "ready"} onClick={() => void toggle()} aria-label={playing ? "Pause replay" : "Play replay"} title={playing ? "Pause replay" : "Play replay"}>{playing ? <Pause size={18} /> : <Play size={18} />}</button>
        <button type="button" className="osu-replay-player__icon" aria-label="Restart replay" disabled={state !== "ready"} onClick={() => { pause(); void seek(0).catch(() => setControlError("Could not restart replay.")); }} title="Restart replay"><RotateCcw size={17} /></button>
        <span className="osu-replay-player__time"><output aria-label="Replay position">{playbackTimeLabel(position)}</output> / {playbackTimeLabel(duration)}</span>
        <label className="osu-replay-player__speed"><span>Speed</span><select value={rate} disabled={state !== "ready"} onChange={event => {
          const value = Number(event.target.value); setRate(value);
          const current = session.current;
          if (current) { const time = current.player.currentTimeMs; current.audioSync.setUserRate(value); void seek(time).catch(() => setControlError("Could not change playback speed.")); }
        }}>{[.25, .5, .75, 1, 1.25, 1.5, 2].map(value => <option key={value} value={value}>{value}x</option>)}</select></label>
        <button type="button" className="osu-replay-player__fullscreen osu-replay-player__icon" aria-label="Fullscreen replay" title="Fullscreen replay" onClick={() => void (document.fullscreenElement ? document.exitFullscreen() : root.current?.requestFullscreen())?.catch(() => setControlError("Fullscreen is unavailable in this browser."))}><Maximize size={17} /></button>
      </div>
      <input aria-label="Replay timeline" className="osu-replay-player__timeline" type="range" min="0" max={duration} step="10" value={Math.min(position, duration)} disabled={state !== "ready"}
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
      {showSettings ? <div id={settingsId} className="osu-replay-player__settings">
        <label>Background dim <output>{Math.round(dim * 100)}%</output><input aria-label="Background dim" type="range" min="0" max="1" step="0.05" value={dim} onChange={event => { const value = Number(event.target.value); setDim(value); if (session.current) session.current.renderer.options.backgroundDim = value; }} /></label>
        <label><input type="checkbox" checked={keysVisible} onChange={event => { setKeysVisible(event.target.checked); if (session.current) session.current.renderer.options.showKeyOverlay = event.target.checked; }} /> Key presses</label>
        <label><input type="checkbox" checked={timingVisible} onChange={event => { setTimingVisible(event.target.checked); if (session.current) session.current.renderer.options.showURBar = event.target.checked; }} /> Hit timing</label>
      </div> : null}
      {controlError ? <p role="status" className="osu-replay-player__notice">{controlError}</p> : null}
      {songError ? <p role="status" className="osu-replay-player__notice">{songError}</p> : null}
    </div>
  </section>;
}
