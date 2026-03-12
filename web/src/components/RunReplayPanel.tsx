import { useEffect, useMemo, useRef, useState } from "react";
import type { GetRunResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "./SectionHeader";
import { PageSection } from "./ui/PageSection";
import { ReplayMouseOverlay } from "./ReplayMouseOverlay";

type MousePathPoint = {
  x: number;
  y: number;
  timestampMs: number;
  isClick: boolean;
};

type Props = {
  run: GetRunResponse;
  runId: string;
  replayMediaUrl: string | null;
  mousePath: MousePathPoint[];
  hitTimestampsMs: number[];
  playbackOffsetMs: number;
  videoOffsetMs: number;
  mousePathLoaded: boolean;
  canDeleteReplayMedia: boolean;
  deletingReplay: boolean;
  onDeleteReplay: () => void;
};

function fmtReplayClock(valueMs: number): string {
  const totalSeconds = Math.max(0, Math.floor(valueMs / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

function shouldClusterHitIndicators(hitTimestampsMs: number[], durationMs: number): boolean {
  if (hitTimestampsMs.length < 12 || durationMs <= 0) return false;
  const hitsPerSecond = hitTimestampsMs.length / Math.max(durationMs / 1000, 1);
  return hitsPerSecond > 2;
}

function clusterHitWindows(
  hitTimestampsMs: number[],
  mergeGapMs = 220,
): Array<{ startMs: number; endMs: number }> {
  if (hitTimestampsMs.length === 0) return [];
  const sorted = [...hitTimestampsMs].sort((a, b) => a - b);
  const windows: Array<{ startMs: number; endMs: number }> = [];
  let startMs = sorted[0];
  let endMs = sorted[0];
  for (let i = 1; i < sorted.length; i += 1) {
    const timestampMs = sorted[i];
    if (timestampMs - endMs <= mergeGapMs) {
      endMs = timestampMs;
      continue;
    }
    windows.push({ startMs, endMs });
    startMs = timestampMs;
    endMs = timestampMs;
  }
  windows.push({ startMs, endMs });
  return windows;
}

export function RunReplayPanel({
  run,
  runId,
  replayMediaUrl,
  mousePath,
  hitTimestampsMs,
  playbackOffsetMs,
  videoOffsetMs,
  mousePathLoaded,
  canDeleteReplayMedia,
  deletingReplay,
  onDeleteReplay,
}: Props) {
  const [playbackMs, setPlaybackMs] = useState(0);
  const [replayDurationMs, setReplayDurationMs] = useState(0);
  const [videoDurationMs, setVideoDurationMs] = useState(0);
  const [replayPlaying, setReplayPlaying] = useState(false);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const lastPlaybackUiSyncRef = useRef(0);
  const lastOverlaySyncRef = useRef(0);
  const hasReplayVideo = Boolean(replayMediaUrl);
  const effectivePlaybackMs = Math.max(0, playbackMs - Math.max(0, playbackOffsetMs));

  const replayDurationMsResolved = useMemo(
    () =>
      Math.max(
        replayDurationMs,
        (mousePath[mousePath.length - 1]?.timestampMs ?? 0) + Math.max(0, playbackOffsetMs),
        Number(run?.durationMs || 0),
      ),
    [mousePath, playbackOffsetMs, replayDurationMs, run?.durationMs],
  );
  const clickTimestampsMs = useMemo(
    () =>
      mousePath
        .filter((point) => point.isClick)
        .map((point) => point.timestampMs + Math.max(0, playbackOffsetMs)),
    [mousePath, playbackOffsetMs],
  );
  const denseHitStream = useMemo(
    () => shouldClusterHitIndicators(hitTimestampsMs, replayDurationMsResolved),
    [hitTimestampsMs, replayDurationMsResolved],
  );
  const hitTimingWindows = useMemo(
    () =>
      denseHitStream
        ? clusterHitWindows(hitTimestampsMs.map((timestampMs) => timestampMs + Math.max(0, playbackOffsetMs)))
        : [],
    [denseHitStream, hitTimestampsMs, playbackOffsetMs],
  );

  useEffect(() => {
    setPlaybackMs(0);
    setReplayDurationMs(mousePath[mousePath.length - 1]?.timestampMs ?? 0);
    setVideoDurationMs(0);
    setReplayPlaying(false);
    lastPlaybackUiSyncRef.current = 0;
    lastOverlaySyncRef.current = 0;
  }, [mousePath, replayMediaUrl, runId]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    const sync = (force = false) => {
      const nextMs = video.currentTime * 1000 + Math.max(0, videoOffsetMs);
      if (force || Math.abs(nextMs - lastPlaybackUiSyncRef.current) >= 33) {
        lastPlaybackUiSyncRef.current = nextMs;
        setPlaybackMs(nextMs);
      }
    };
    const syncDuration = () => {
      if (Number.isFinite(video.duration) && video.duration > 0) {
        setVideoDurationMs(video.duration * 1000);
      }
    };
    const onPlay = () => {
      setReplayPlaying(true);
      sync(true);
    };
    const onPause = () => {
      const videoTimelineEndMs = videoDurationMs + Math.max(0, videoOffsetMs);
      if (video.ended && replayDurationMsResolved > videoTimelineEndMs + 80) {
        sync(true);
        return;
      }
      setReplayPlaying(false);
      sync(true);
    };
    const onEnded = () => {
      sync(true);
      const videoTimelineEndMs = videoDurationMs + Math.max(0, videoOffsetMs);
      if (replayDurationMsResolved > videoTimelineEndMs + 80) {
        setReplayPlaying(true);
        return;
      }
      setReplayPlaying(false);
    };
    const onTimeUpdate = () => sync(false);
    const onSeeked = () => sync(true);
    video.addEventListener("timeupdate", onTimeUpdate);
    video.addEventListener("seeked", onSeeked);
    video.addEventListener("loadedmetadata", syncDuration);
    video.addEventListener("durationchange", syncDuration);
    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);
    video.addEventListener("ended", onEnded);
    sync(true);
    syncDuration();
    return () => {
      video.removeEventListener("timeupdate", onTimeUpdate);
      video.removeEventListener("seeked", onSeeked);
      video.removeEventListener("loadedmetadata", syncDuration);
      video.removeEventListener("durationchange", syncDuration);
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("ended", onEnded);
    };
  }, [replayDurationMsResolved, replayMediaUrl, videoDurationMs, videoOffsetMs]);

  useEffect(() => {
    if (mousePath.length < 2 || !replayPlaying) return;
    let raf = 0;
    let last = 0;
    const tick = (timestamp: number) => {
      if (!last) last = timestamp;
      const delta = timestamp - last;
      last = timestamp;
      setPlaybackMs((current) => {
        let base = current;
        if (hasReplayVideo) {
          const video = videoRef.current;
          const currentVideoMs = video
            ? Math.min(video.currentTime * 1000, videoDurationMs) + Math.max(0, videoOffsetMs)
            : 0;
          const videoTimelineEndMs = videoDurationMs + Math.max(0, videoOffsetMs);
          if (currentVideoMs + 40 < videoTimelineEndMs) {
            return current;
          }
          base = Math.max(base, currentVideoMs);
        }
        const next = Math.min(base + delta, replayDurationMsResolved);
        if (next >= replayDurationMsResolved) {
          setReplayPlaying(false);
        }
        return next;
      });
      raf = window.requestAnimationFrame(tick);
    };
    raf = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(raf);
  }, [hasReplayVideo, mousePath.length, replayDurationMsResolved, replayPlaying, videoDurationMs, videoOffsetMs]);

  const handleReplaySeek = (nextMs: number) => {
    const clamped = Math.max(0, Math.min(nextMs, replayDurationMsResolved));
    setPlaybackMs(clamped);
    const video = hasReplayVideo ? videoRef.current : null;
    if (video) {
      video.currentTime = Math.max(0, Math.min(clamped - Math.max(0, videoOffsetMs), videoDurationMs)) / 1000;
    }
  };

  const toggleReplayPlayback = () => {
    const video = hasReplayVideo ? videoRef.current : null;
    if (video) {
      const videoTimelineEndMs = videoDurationMs + Math.max(0, videoOffsetMs);
      if (videoDurationMs > 0 && video.paused && playbackMs >= videoTimelineEndMs && replayDurationMsResolved > videoTimelineEndMs + 80) {
        if (playbackMs >= replayDurationMsResolved) {
          setPlaybackMs(videoTimelineEndMs);
        }
        setReplayPlaying(true);
        return;
      }
      if (video.paused) {
        void video.play();
      } else {
        video.pause();
      }
      return;
    }
    setReplayPlaying((current) => {
      if (current) return false;
      if (playbackMs >= replayDurationMsResolved) {
        setPlaybackMs(0);
      }
      return true;
    });
  };

  const resetReplayPlayback = () => {
    setReplayPlaying(false);
    handleReplaySeek(0);
    const video = hasReplayVideo ? videoRef.current : null;
    if (video) video.pause();
  };

  return (
    <PageSection>
      <SectionHeader
        eyebrow="Replay"
        title="Replay view"
        body={
          mousePath.length > 0
            ? "Video, movement path, hits, clicks, and saved moments all follow one shared replay timeline."
            : "Review the saved replay for this run."
        }
        aside={
          canDeleteReplayMedia ? (
            <button
              type="button"
              className="text-sm text-danger underline underline-offset-3 disabled:opacity-50"
              disabled={deletingReplay}
              onClick={onDeleteReplay}
            >
              {deletingReplay ? "Removing..." : "Remove replay"}
            </button>
          ) : null
        }
      />
      <div className="relative overflow-hidden rounded-2xl border border-white/8 bg-black/40">
        <div className="relative mx-auto aspect-[900/520] w-full bg-[rgba(2,8,10,0.96)]">
          {replayMediaUrl ? (
            <video
              ref={videoRef}
              preload="metadata"
              className="absolute left-[17.5%] top-[18.37%] z-0 h-[63.27%] w-[65%] object-fill"
              src={replayMediaUrl}
            />
          ) : (
            <div className="absolute left-[17.5%] top-[18.37%] flex h-[63.27%] w-[65%] items-center justify-center border border-white/6 bg-[linear-gradient(180deg,rgba(15,23,19,0.96),rgba(4,8,10,0.98))] px-4 text-center text-[12px] text-muted">
              No video was saved for this run.
              <br />
              The replay still uses the same playback timeline and movement view.
            </div>
          )}
          {mousePath.length > 0 ? (
            <ReplayMouseOverlay
              points={mousePath}
              hitTimestampsMs={hitTimestampsMs}
              playbackMs={effectivePlaybackMs}
              playbackOffsetMs={playbackOffsetMs}
              videoOffsetMs={videoOffsetMs}
              videoRef={hasReplayVideo ? videoRef : undefined}
              onPlaybackMs={(nextMs) => {
                if (Math.abs(nextMs - lastOverlaySyncRef.current) < 33) return;
                lastOverlaySyncRef.current = nextMs;
                lastPlaybackUiSyncRef.current = nextMs;
                setPlaybackMs(nextMs);
              }}
            />
          ) : null}
        </div>
      </div>
      <div className="mt-3 rounded-xl border border-white/8 bg-black/20 px-3 py-3">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3 text-[11px] uppercase tracking-[0.08em] text-muted-2">
          <div className="flex flex-wrap items-center gap-3">
            <span>
              {mousePath.length > 0
                ? `${mousePath.length.toLocaleString()} samples`
                : mousePathLoaded
                  ? "Video only"
                  : "Loading movement path"}
            </span>
            {mousePath.length > 0 ? <span>{clickTimestampsMs.length} clicks</span> : null}
            {mousePath.length > 0 ? (
              <span>{denseHitStream ? `${hitTimingWindows.length} contact windows` : `${hitTimestampsMs.length} hits`}</span>
            ) : null}
            {run.contextWindows.length > 0 ? <span>{run.contextWindows.length} saved moments</span> : null}
          </div>
          <div className="text-text">
            {fmtReplayClock(playbackMs)} / {fmtReplayClock(replayDurationMsResolved)}
          </div>
        </div>
        <div
          className="relative h-4 cursor-pointer overflow-hidden rounded-full bg-white/6"
          onClick={(event) => {
            const rect = event.currentTarget.getBoundingClientRect();
            const fraction = Math.max(0, Math.min(1, (event.clientX - rect.left) / Math.max(rect.width, 1)));
            handleReplaySeek(fraction * replayDurationMsResolved);
          }}
        >
          {run.contextWindows.map((window, index) => {
            const left = (Number(window.startMs) / Math.max(replayDurationMsResolved, 1)) * 100;
            const width = Math.max(
              0.5,
              ((Number(window.endMs) - Number(window.startMs)) / Math.max(replayDurationMsResolved, 1)) * 100,
            );
            return (
              <div
                key={`moment-${window.startMs}-${index}`}
                className="absolute top-0 bottom-0 border-l border-violet/80 bg-violet/18"
                style={{ left: `${left}%`, width: `${width}%` }}
                title={window.label || "Saved moment"}
              />
            );
          })}
          {clickTimestampsMs.map((timestampMs, index) => (
            <div
              key={`click-${timestampMs}-${index}`}
              className="absolute top-0 bottom-0 w-[2px] bg-gold/90"
              style={{ left: `${(timestampMs / Math.max(replayDurationMsResolved, 1)) * 100}%` }}
              title={`Click at ${(timestampMs / 1000).toFixed(2)}s`}
            />
          ))}
          {denseHitStream
            ? hitTimingWindows.map((window, index) => {
                const totalDurationMs = Math.max(replayDurationMsResolved, 1);
                const left = (window.startMs / totalDurationMs) * 100;
                const width = Math.max(0.8, ((Math.max(window.endMs, window.startMs + 120) - window.startMs) / totalDurationMs) * 100);
                return (
                  <div
                    key={`hit-window-${window.startMs}-${index}`}
                    className="absolute top-0 bottom-0 bg-mint/35 border-l border-mint border-r border-mint/30"
                    style={{ left: `${left}%`, width: `${width}%` }}
                    title={`Contact window around ${(window.startMs / 1000).toFixed(2)}s`}
                  />
                );
              })
            : hitTimestampsMs.map((timestampMs, index) => {
                const adjustedTimestampMs = timestampMs + Math.max(0, playbackOffsetMs);
                return (
                <div
                  key={`hit-${timestampMs}-${index}`}
                  className="absolute top-0 bottom-0 w-[2px] bg-mint"
                  style={{ left: `${(adjustedTimestampMs / Math.max(replayDurationMsResolved, 1)) * 100}%` }}
                  title={`Hit at ${(adjustedTimestampMs / 1000).toFixed(2)}s`}
                />
                );
              })}
          <div
            className="absolute top-0 bottom-0 w-[2px] bg-cyan shadow-[0_0_0_1px_rgba(184,255,225,0.35)]"
            style={{ left: `${((playbackMs || 0) / Math.max(replayDurationMsResolved, 1)) * 100}%` }}
          />
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <button
            type="button"
            className="rounded-full border border-white/10 bg-white/6 px-3 py-1.5 text-sm text-text transition-colors hover:border-cyan/40 hover:text-cyan"
            onClick={toggleReplayPlayback}
          >
            {replayPlaying ? "Pause" : playbackMs >= replayDurationMsResolved ? "Replay" : "Play"}
          </button>
          <button
            type="button"
            className="rounded-full border border-white/10 bg-transparent px-3 py-1.5 text-sm text-muted transition-colors hover:border-white/20 hover:text-text"
            onClick={resetReplayPlayback}
          >
            Reset
          </button>
        </div>
      </div>
    </PageSection>
  );
}
