import { useEffect, useMemo, useRef } from "react";
import type { MousePathPoint } from "../lib/api";

const CANVAS_W = 900;
const CANVAS_H = 520;
const TRAIL_MS = 2000;
const VIEWPORT_PX = 960;

type Props = {
  points: MousePathPoint[];
  hitTimestampsMs: number[];
  playbackMs: number;
  videoRef?: React.RefObject<HTMLVideoElement | null>;
};

interface OvershootMarker {
  x: number;
  y: number;
  timestampMs: number;
}

interface RenderMetrics {
  speeds: number[];
  p5: number;
  p95: number;
}

interface PlaybackSample {
  index: number;
  x: number;
  y: number;
  timestampMs: number;
}

function lowerBoundByTimestamp<T extends { timestampMs: number }>(items: T[], timestampMs: number): number {
  let lo = 0;
  let hi = items.length;
  while (lo < hi) {
    const mid = Math.floor((lo + hi) / 2);
    if (items[mid].timestampMs < timestampMs) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

function lowerBoundNumbers(values: number[], target: number): number {
  let lo = 0;
  let hi = values.length;
  while (lo < hi) {
    const mid = Math.floor((lo + hi) / 2);
    if (values[mid] < target) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

function hitWindowIndex(hitTimestampsMs: number[], playbackMs: number, windowMs: number): number {
  if (hitTimestampsMs.length === 0) return -1;
  const start = lowerBoundNumbers(hitTimestampsMs, playbackMs - windowMs);
  for (let i = start; i < hitTimestampsMs.length; i += 1) {
    const delta = hitTimestampsMs[i] - playbackMs;
    if (delta > windowMs) break;
    if (Math.abs(delta) <= windowMs) return i;
  }
  return -1;
}

function lerpColor(
  a: [number, number, number],
  b: [number, number, number],
  t: number,
  alpha = 1,
): string {
  const r = Math.round(a[0] + (b[0] - a[0]) * t);
  const g = Math.round(a[1] + (b[1] - a[1]) * t);
  const bl = Math.round(a[2] + (b[2] - a[2]) * t);
  return `rgba(${r},${g},${bl},${alpha})`;
}

function speedColor(t: number, alpha = 1): string {
  const stops: [number, [number, number, number]][] = [
    [0.0, [30, 100, 255]],
    [0.25, [0, 200, 220]],
    [0.5, [0, 220, 80]],
    [0.75, [255, 220, 0]],
    [1.0, [255, 50, 30]],
  ];

  for (let i = 0; i < stops.length - 1; i += 1) {
    const [t0, c0] = stops[i];
    const [t1, c1] = stops[i + 1];
    if (t >= t0 && t <= t1) {
      return lerpColor(c0, c1, (t - t0) / (t1 - t0), alpha);
    }
  }

  return `rgba(255,50,30,${alpha})`;
}

function detectOvershoots(points: MousePathPoint[]): OvershootMarker[] {
  if (points.length < 4) return [];

  const vx: number[] = new Array(points.length - 1);
  const vy: number[] = new Array(points.length - 1);
  const spd: number[] = new Array(points.length - 1);
  let speedSum = 0;

  for (let i = 0; i < points.length - 1; i += 1) {
    const dt = Math.max((points[i + 1].timestampMs - points[i].timestampMs) / 1000, 0.001);
    vx[i] = (points[i + 1].x - points[i].x) / dt;
    vy[i] = (points[i + 1].y - points[i].y) / dt;
    spd[i] = Math.hypot(vx[i], vy[i]);
    speedSum += spd[i];
  }

  const meanSpeed = speedSum / Math.max(points.length - 1, 1);
  const speedThreshold = Math.max(meanSpeed * 0.3, 100);
  const markers: OvershootMarker[] = [];

  for (let i = 0; i < points.length - 2; i += 1) {
    if (spd[i] < speedThreshold || spd[i + 1] < speedThreshold) continue;
    if (points[i].isClick || points[i + 1].isClick || points[i + 2].isClick) continue;

    const dot = (vx[i] * vx[i + 1] + vy[i] * vy[i + 1]) / (spd[i] * spd[i + 1]);
    if (dot < -0.5) {
      markers.push({
        x: points[i + 1].x,
        y: points[i + 1].y,
        timestampMs: points[i + 1].timestampMs,
      });
      i += 2;
    }
  }

  return markers;
}

function shouldClusterHitIndicators(hitTimestampsMs: number[], durationMs: number): boolean {
  if (hitTimestampsMs.length < 12 || durationMs <= 0) return false;
  const hitsPerSecond = hitTimestampsMs.length / Math.max(durationMs / 1000, 1);
  return hitsPerSecond > 2;
}

function buildRenderMetrics(points: MousePathPoint[]): RenderMetrics {
  const speeds: number[] = [];
  for (let i = 0; i < points.length - 1; i += 1) {
    const p = points[i];
    const q = points[i + 1];
    const dt = Math.max((q.timestampMs - p.timestampMs) / 1000, 0.001);
    speeds.push(Math.hypot(q.x - p.x, q.y - p.y) / dt);
  }
  const sorted = [...speeds].sort((a, b) => a - b);
  return {
    speeds,
    p5: sorted[Math.floor(sorted.length * 0.05)] ?? 0,
    p95: sorted[Math.floor(sorted.length * 0.95)] ?? 1,
  };
}

function sampleAtPlayback(points: MousePathPoint[], playbackMs: number): PlaybackSample {
  if (points.length === 0) {
    return { index: 0, x: 0, y: 0, timestampMs: 0 };
  }
  if (points.length === 1 || playbackMs <= points[0].timestampMs) {
    return {
      index: 0,
      x: points[0].x,
      y: points[0].y,
      timestampMs: points[0].timestampMs,
    };
  }

  let lo = 0;
  let hi = points.length - 1;
  while (lo < hi) {
    const mid = Math.floor((lo + hi + 1) / 2);
    if (points[mid].timestampMs <= playbackMs) lo = mid;
    else hi = mid - 1;
  }

  const index = Math.max(0, Math.min(points.length - 1, lo));
  const current = points[index];
  const next = points[Math.min(points.length - 1, index + 1)];
  const span = Math.max(1, next.timestampMs - current.timestampMs);
  const t = index < points.length - 1
    ? Math.max(0, Math.min(1, (playbackMs - current.timestampMs) / span))
    : 0;

  return {
    index,
    x: current.x + (next.x - current.x) * t,
    y: current.y + (next.y - current.y) * t,
    timestampMs: playbackMs,
  };
}

function drawPath(
  canvas: HTMLCanvasElement,
  points: MousePathPoint[],
  overshoots: OvershootMarker[],
  metrics: RenderMetrics,
  playbackMs: number,
  hitTimestampsMs: number[],
) {
  const ctx = canvas.getContext("2d");
  if (!ctx) return;

  ctx.clearRect(0, 0, CANVAS_W, CANVAS_H);
  if (points.length < 2) return;
  const denseHitStream = shouldClusterHitIndicators(
    hitTimestampsMs,
    points[points.length - 1]?.timestampMs ?? 0,
  );

  const playbackSample = sampleAtPlayback(points, playbackMs);
  const count = Math.max(2, Math.min(points.length, playbackSample.index + 1));
  const trailStartMs = Math.max(0, playbackMs - TRAIL_MS);
  const trailStartIndex = Math.max(0, lowerBoundByTimestamp(points, trailStartMs) - 1);
  const trailEndIndex = Math.min(points.length - 1, Math.max(trailStartIndex + 1, playbackSample.index));
  const clickStartIndex = lowerBoundByTimestamp(points, trailStartMs);
  const overshootStartIndex = lowerBoundByTimestamp(overshoots, trailStartMs);

  const pad = 28;
  const width = CANVAS_W - pad * 2;
  const height = CANVAS_H - pad * 2;
  const scale = (CANVAS_W * 0.65) / VIEWPORT_PX;
  const toX = (x: number) => CANVAS_W / 2 + (x - playbackSample.x) * scale;
  const toY = (y: number) => CANVAS_H / 2 + (y - playbackSample.y) * scale;

  const norm = (s: number) => Math.max(0, Math.min(1, (s - metrics.p5) / (metrics.p95 - metrics.p5 || 1)));

  ctx.strokeStyle = "rgba(255,255,255,0.04)";
  ctx.lineWidth = 1;
  for (let gx = 0; gx <= 4; gx += 1) {
    const px = pad + (width / 4) * gx;
    ctx.beginPath();
    ctx.moveTo(px, pad);
    ctx.lineTo(px, CANVAS_H - pad);
    ctx.stroke();
  }
  for (let gy = 0; gy <= 4; gy += 1) {
    const py = pad + (height / 4) * gy;
    ctx.beginPath();
    ctx.moveTo(pad, py);
    ctx.lineTo(CANVAS_W - pad, py);
    ctx.stroke();
  }

  const vpW = VIEWPORT_PX * scale;
  const vpH = vpW * (9 / 16);
  const vpX = CANVAS_W / 2 - vpW / 2;
  const vpY = CANVAS_H / 2 - vpH / 2;

  ctx.save();
  ctx.fillStyle = "rgba(0,0,0,0.35)";
  ctx.fillRect(0, 0, CANVAS_W, vpY);
  ctx.fillRect(0, vpY + vpH, CANVAS_W, CANVAS_H - (vpY + vpH));
  ctx.fillRect(0, vpY, vpX, vpH);
  ctx.fillRect(vpX + vpW, vpY, CANVAS_W - (vpX + vpW), vpH);
  ctx.restore();

  ctx.save();
  ctx.strokeStyle = "rgba(0,210,255,0.55)";
  ctx.lineWidth = 1.5;
  ctx.setLineDash([6, 4]);
  ctx.strokeRect(vpX, vpY, vpW, vpH);
  ctx.setLineDash([]);
  ctx.restore();

  ctx.save();
  ctx.strokeStyle = "rgba(0,210,255,0.35)";
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(CANVAS_W / 2 - 8, CANVAS_H / 2);
  ctx.lineTo(CANVAS_W / 2 + 8, CANVAS_H / 2);
  ctx.stroke();
  ctx.beginPath();
  ctx.moveTo(CANVAS_W / 2, CANVAS_H / 2 - 8);
  ctx.lineTo(CANVAS_W / 2, CANVAS_H / 2 + 8);
  ctx.stroke();
  ctx.restore();

  for (let i = trailStartIndex; i < Math.min(trailEndIndex, metrics.speeds.length); i += 1) {
    const age = playbackMs - points[i + 1].timestampMs;
    if (age > TRAIL_MS) continue;
    const alpha = Math.max(0, 1 - age / TRAIL_MS);
    ctx.beginPath();
    ctx.moveTo(toX(points[i].x), toY(points[i].y));
    ctx.lineTo(toX(points[i + 1].x), toY(points[i + 1].y));
    ctx.strokeStyle = speedColor(norm(metrics.speeds[i]), alpha);
    ctx.lineWidth = 1.8;
    ctx.lineCap = "round";
    ctx.stroke();
  }

  if (count < points.length) {
    const prev = points[Math.max(0, count - 1)];
    const age = playbackMs - playbackSample.timestampMs;
    const alpha = Math.max(0, 1 - age / TRAIL_MS);
    ctx.beginPath();
    ctx.moveTo(toX(prev.x), toY(prev.y));
    ctx.lineTo(toX(playbackSample.x), toY(playbackSample.y));
    ctx.strokeStyle = speedColor(
      norm(metrics.speeds[Math.max(0, Math.min(metrics.speeds.length - 1, count - 1))] ?? 0),
      alpha,
    );
    ctx.lineWidth = 1.8;
    ctx.lineCap = "round";
    ctx.stroke();
  }

  const cutoff = playbackSample.timestampMs;
  for (let i = overshootStartIndex; i < overshoots.length; i += 1) {
    const marker = overshoots[i];
    if (marker.timestampMs > cutoff) break;
    const age = playbackMs - marker.timestampMs;
    if (age > TRAIL_MS) continue;
    const alpha = Math.max(0, 0.85 * (1 - age / TRAIL_MS));
    const px = toX(marker.x);
    const py = toY(marker.y);
    ctx.beginPath();
    ctx.moveTo(px, py - 10);
    ctx.lineTo(px + 7, py + 4);
    ctx.lineTo(px - 7, py + 4);
    ctx.closePath();
    ctx.fillStyle = `rgba(255,80,30,${alpha})`;
    ctx.strokeStyle = `rgba(255,160,80,${Math.min(1, alpha * 1.05)})`;
    ctx.lineWidth = 1;
    ctx.fill();
    ctx.stroke();
  }

  for (let i = clickStartIndex; i < Math.min(count, points.length); i += 1) {
    const point = points[i];
    if (!point.isClick) continue;
    const age = playbackMs - point.timestampMs;
    if (age > TRAIL_MS) continue;
    const alpha = Math.max(0, 0.9 * (1 - age / TRAIL_MS));
    ctx.beginPath();
    ctx.arc(toX(point.x), toY(point.y), 5, 0, Math.PI * 2);
    ctx.fillStyle = `rgba(255,220,30,${alpha})`;
    ctx.strokeStyle = `rgba(255,255,255,${alpha * 0.55})`;
    ctx.lineWidth = 1;
    ctx.fill();
    ctx.stroke();
  }

  const activeHitIndex = hitWindowIndex(hitTimestampsMs, playbackMs, 90);
  if (activeHitIndex >= 0) {
    const activeHit = hitTimestampsMs[activeHitIndex];
    const pulse = Math.max(0, 1 - Math.abs(playbackMs - activeHit) / 90);
    const px = toX(playbackSample.x);
    const py = toY(playbackSample.y);
    const size = denseHitStream ? 8 : 10;
    const gap = denseHitStream ? 3 : 4;
    const alpha = denseHitStream ? 0.4 + pulse * 0.3 : 0.55 + pulse * 0.35;
    ctx.save();
    ctx.strokeStyle = `rgba(255,255,255,${alpha})`;
    ctx.lineWidth = 2;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(px - size, py - size);
    ctx.lineTo(px - gap, py - gap);
    ctx.moveTo(px + gap, py + gap);
    ctx.lineTo(px + size, py + size);
    ctx.moveTo(px + size, py - size);
    ctx.lineTo(px + gap, py - gap);
    ctx.moveTo(px - gap, py + gap);
    ctx.lineTo(px - size, py + size);
    ctx.stroke();
    ctx.strokeStyle = `rgba(0,245,160,${alpha * 0.7})`;
    ctx.lineWidth = 1;
    ctx.stroke();
    ctx.restore();
  }

  ctx.beginPath();
  ctx.arc(toX(points[0].x), toY(points[0].y), 5, 0, Math.PI * 2);
  ctx.fillStyle = "rgba(0,245,160,0.9)";
  ctx.fill();
}

export function ReplayMouseOverlay({ points, hitTimestampsMs, playbackMs, videoRef }: Props) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const overshoots = useMemo(() => detectOvershoots(points), [points]);
  const renderMetrics = useMemo(() => buildRenderMetrics(points), [points]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || videoRef?.current) return;
    drawPath(canvas, points, overshoots, renderMetrics, playbackMs, hitTimestampsMs);
  }, [hitTimestampsMs, overshoots, playbackMs, points, renderMetrics, videoRef]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const video = videoRef?.current;
    if (!canvas || !video) return;
    if (!video.paused && !video.ended && Math.abs(playbackMs - video.currentTime * 1000) < 24) return;
    drawPath(canvas, points, overshoots, renderMetrics, playbackMs, hitTimestampsMs);
  }, [hitTimestampsMs, overshoots, playbackMs, points, renderMetrics, videoRef]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const video = videoRef?.current;
    if (!canvas || !video) return;

    let raf = 0;
    let frameCallbackHandle = 0;
    let cancelled = false;
    const videoWithCallback = video as HTMLVideoElement & {
      requestVideoFrameCallback?: (
        callback: (now: number, metadata: { mediaTime: number }) => void,
      ) => number;
      cancelVideoFrameCallback?: (handle: number) => void;
    };

    const drawAtMs = (mediaMs: number) => {
      if (cancelled) return;
      drawPath(canvas, points, overshoots, renderMetrics, mediaMs, hitTimestampsMs);
    };

    const stop = () => {
      window.cancelAnimationFrame(raf);
      raf = 0;
      if (frameCallbackHandle && videoWithCallback.cancelVideoFrameCallback) {
        videoWithCallback.cancelVideoFrameCallback(frameCallbackHandle);
      }
      frameCallbackHandle = 0;
    };

    const tick = () => {
      drawAtMs(video.currentTime * 1000);
      if (!cancelled && !video.paused && !video.ended) {
        raf = window.requestAnimationFrame(tick);
      }
    };

    const start = () => {
      stop();
      if (videoWithCallback.requestVideoFrameCallback) {
        const handleFrame = (_now: number, metadata: { mediaTime: number }) => {
          drawAtMs(metadata.mediaTime * 1000);
          if (!cancelled && !video.paused && !video.ended) {
            frameCallbackHandle = videoWithCallback.requestVideoFrameCallback!(handleFrame);
          }
        };
        frameCallbackHandle = videoWithCallback.requestVideoFrameCallback(handleFrame);
        return;
      }
      raf = window.requestAnimationFrame(tick);
    };

    const onPlay = () => start();
    const onPause = () => {
      stop();
      drawAtMs(video.currentTime * 1000);
    };
    const onSeek = () => {
      drawAtMs(video.currentTime * 1000);
      if (!video.paused && !video.ended) {
        start();
      }
    };

    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);
    video.addEventListener("ended", onPause);
    video.addEventListener("seeked", onSeek);

    drawAtMs(video.currentTime * 1000);
    if (!video.paused && !video.ended) {
      start();
    }

    return () => {
      cancelled = true;
      stop();
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("ended", onPause);
      video.removeEventListener("seeked", onSeek);
    };
  }, [hitTimestampsMs, overshoots, points, renderMetrics, videoRef]);

  if (points.length < 2) return null;

  return (
    <div className="pointer-events-none absolute inset-0 z-10">
      <canvas
        ref={canvasRef}
        width={CANVAS_W}
        height={CANVAS_H}
        className="block h-full w-full"
        aria-label="Replay mouse path overlay"
      />
    </div>
  );
}
