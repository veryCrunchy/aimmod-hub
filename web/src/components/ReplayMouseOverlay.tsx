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

  const count = (() => {
    if (playbackMs <= points[0].timestampMs) return 2;
    let lo = 0;
    let hi = points.length - 1;
    while (lo < hi) {
      const mid = Math.floor((lo + hi + 1) / 2);
      if (points[mid].timestampMs <= playbackMs) lo = mid;
      else hi = mid - 1;
    }
    return Math.max(2, Math.min(points.length, lo + 1));
  })();

  const pad = 28;
  const width = CANVAS_W - pad * 2;
  const height = CANVAS_H - pad * 2;
  const scale = (CANVAS_W * 0.65) / VIEWPORT_PX;
  const cam = points[Math.min(count - 1, points.length - 1)];
  const toX = (x: number) => CANVAS_W / 2 + (x - cam.x) * scale;
  const toY = (y: number) => CANVAS_H / 2 + (y - cam.y) * scale;

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

  for (let i = 0; i < Math.min(count - 1, metrics.speeds.length); i += 1) {
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

  const cutoff = points[Math.min(count - 1, points.length - 1)]?.timestampMs ?? Infinity;
  for (const marker of overshoots) {
    if (marker.timestampMs > cutoff) continue;
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

  for (let i = 0; i < Math.min(count, points.length); i += 1) {
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

  const activeHit = hitTimestampsMs.find((ts) => Math.abs(playbackMs - ts) <= 90);
  if (activeHit != null) {
    const anchor = points[Math.max(0, Math.min(count - 1, points.length - 1))];
    const pulse = Math.max(0, 1 - Math.abs(playbackMs - activeHit) / 90);
    const px = toX(anchor.x);
    const py = toY(anchor.y);
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
  const denseHitStream = useMemo(
    () => shouldClusterHitIndicators(hitTimestampsMs, points[points.length - 1]?.timestampMs ?? 0),
    [hitTimestampsMs, points],
  );

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || videoRef?.current) return;
    drawPath(canvas, points, overshoots, renderMetrics, playbackMs, hitTimestampsMs);
  }, [hitTimestampsMs, overshoots, playbackMs, points, renderMetrics, videoRef]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const video = videoRef?.current;
    if (!canvas || !video) return;

    let raf = 0;
    let cancelled = false;

    const drawFromVideo = () => {
      if (cancelled) return;
      drawPath(canvas, points, overshoots, renderMetrics, video.currentTime * 1000, hitTimestampsMs);
    };

    const tick = () => {
      drawFromVideo();
      if (!cancelled && !video.paused && !video.ended) {
        raf = window.requestAnimationFrame(tick);
      }
    };

    const stop = () => {
      window.cancelAnimationFrame(raf);
    };

    const start = () => {
      stop();
      raf = window.requestAnimationFrame(tick);
    };

    const onPlay = () => start();
    const onPause = () => {
      stop();
      drawFromVideo();
    };
    const onSeek = () => drawFromVideo();

    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);
    video.addEventListener("seeked", onSeek);
    video.addEventListener("timeupdate", onSeek);

    drawFromVideo();
    if (!video.paused && !video.ended) {
      start();
    }

    return () => {
      cancelled = true;
      stop();
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("seeked", onSeek);
      video.removeEventListener("timeupdate", onSeek);
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
