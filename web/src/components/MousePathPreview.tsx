import type { MousePathPoint } from "../lib/api";

type Props = {
  points: MousePathPoint[];
};

function buildPath(points: MousePathPoint[]) {
  if (points.length === 0) return "";
  const xs = points.map((point) => point.x);
  const ys = points.map((point) => point.y);
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const minY = Math.min(...ys);
  const maxY = Math.max(...ys);
  const width = Math.max(1, maxX - minX);
  const height = Math.max(1, maxY - minY);
  const pad = 24;
  const viewportWidth = 960;
  const viewportHeight = 420;
  const scale = Math.min(
    (viewportWidth - pad * 2) / width,
    (viewportHeight - pad * 2) / height,
  );
  const offsetX = (viewportWidth - width * scale) / 2;
  const offsetY = (viewportHeight - height * scale) / 2;

  const transformed = points.map((point) => ({
    x: offsetX + (point.x - minX) * scale,
    y: offsetY + (point.y - minY) * scale,
    isClick: point.isClick,
  }));

  const path = transformed
    .map((point, index) => `${index === 0 ? "M" : "L"}${point.x.toFixed(1)},${point.y.toFixed(1)}`)
    .join(" ");

  return { path, transformed, viewportWidth, viewportHeight };
}

export function MousePathPreview({ points }: Props) {
  const built = buildPath(points);
  if (!built) return null;

  return (
    <div className="overflow-hidden rounded-2xl border border-white/8 bg-black/30">
      <svg
        viewBox={`0 0 ${built.viewportWidth} ${built.viewportHeight}`}
        className="block h-auto w-full"
        role="img"
        aria-label="Mouse path preview"
      >
        <defs>
          <linearGradient id="mouse-path-gradient" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="#7fe0ad" />
            <stop offset="50%" stopColor="#6dd2ff" />
            <stop offset="100%" stopColor="#d8d84f" />
          </linearGradient>
        </defs>
        <rect width={built.viewportWidth} height={built.viewportHeight} fill="rgba(2,8,10,0.96)" />
        <path
          d={built.path}
          fill="none"
          stroke="url(#mouse-path-gradient)"
          strokeWidth="3"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        {built.transformed.filter((point) => point.isClick).map((point, index) => (
          <circle
            key={`${point.x}-${point.y}-${index}`}
            cx={point.x}
            cy={point.y}
            r="4.5"
            fill="#d8d84f"
            stroke="rgba(2, 8, 10, 0.9)"
            strokeWidth="1.5"
          />
        ))}
      </svg>
    </div>
  );
}
