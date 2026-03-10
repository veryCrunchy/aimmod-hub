import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { RunPreview } from "../../gen/aimmod/hub/v1/hub_pb";

const C = {
  muted: "#a7c2b3",
  grid: "rgba(130,201,157,0.10)",
  tooltipBg: "rgba(8,18,14,0.97)",
  tooltipBorder: "rgba(130,201,157,0.22)",
  // matches kovaaks exactly
  floor: "#a78bfa80",  // bottom 10% — violet, faint
  mid: "#00b4ff50",   // middle — blue, faint
  peak: "#00f5a0",    // top 10% — green, solid
};

type Props = {
  runs: RunPreview[];
};

function percentile(sorted: number[], p: number): number {
  return sorted[Math.floor(sorted.length * p)] ?? sorted[sorted.length - 1];
}

type Bin = { label: string; count: number; lo: number; hi: number };

type TooltipProps = {
  active?: boolean;
  payload?: { payload: Bin }[];
};

function CustomTooltip({ active, payload }: TooltipProps) {
  if (!active || !payload?.length) return null;
  const d = payload[0].payload;
  return (
    <div
      style={{
        background: C.tooltipBg,
        border: `1px solid ${C.tooltipBorder}`,
        borderRadius: 12,
        padding: "8px 14px",
        fontFamily: "JetBrains Mono, monospace",
        fontSize: 12,
      }}
    >
      <p style={{ color: C.muted, marginBottom: 4, fontSize: 11 }}>
        {Math.round(d.lo).toLocaleString()} – {Math.round(d.hi).toLocaleString()}
      </p>
      <p style={{ color: "#f3fff8", fontWeight: 700 }}>{d.count} sessions</p>
    </div>
  );
}

export function ScoreDistributionChart({ runs }: Props) {
  if (runs.length < 4) return null;

  const sorted = [...runs].map((r) => r.score).sort((a, b) => a - b);
  const min = sorted[0];
  const max = sorted[sorted.length - 1];
  const range = max - min;
  if (range === 0) return null;

  const BINS = 10;
  const binSize = range / BINS;

  const bins: Bin[] = Array.from({ length: BINS }, (_, i) => {
    const lo = min + i * binSize;
    const hi = lo + binSize;
    const count = sorted.filter((s) =>
      i === BINS - 1 ? s >= lo && s <= hi : s >= lo && s < hi,
    ).length;
    return { label: Math.round(lo).toLocaleString(), count, lo, hi };
  });

  const p10 = percentile(sorted, 0.1);
  const p50 = percentile(sorted, 0.5);
  const p90 = percentile(sorted, 0.9);

  const legend = [
    { color: "#a78bfa", label: `Floor (p10): ${Math.round(p10).toLocaleString()}` },
    { color: "#ffd700", label: `Median: ${Math.round(p50).toLocaleString()}` },
    { color: "#00f5a0", label: `Peak (p90): ${Math.round(p90).toLocaleString()}` },
  ];

  return (
    <div>
      <div className="mb-3 flex flex-wrap gap-x-5 gap-y-1.5">
        {legend.map((l) => (
          <div key={l.label} className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full shrink-0" style={{ background: l.color }} />
            <span className="text-[11px] text-muted">{l.label}</span>
          </div>
        ))}
      </div>

      <ResponsiveContainer width="100%" height={180}>
        <BarChart data={bins} margin={{ top: 4, right: 8, bottom: 0, left: 0 }} barCategoryGap="14%">
          <CartesianGrid stroke={C.grid} strokeDasharray="3 4" vertical={false} />
          <XAxis
            dataKey="label"
            tick={{ fill: C.muted, fontSize: 9, fontFamily: "JetBrains Mono, monospace" }}
            tickLine={false}
            axisLine={false}
          />
          <YAxis
            allowDecimals={false}
            tick={{ fill: C.muted, fontSize: 10, fontFamily: "JetBrains Mono, monospace" }}
            tickLine={false}
            axisLine={false}
            width={24}
          />
          <Tooltip content={<CustomTooltip />} cursor={{ fill: "rgba(255,255,255,0.04)" }} />
          <Bar dataKey="count" name="Runs" radius={[3, 3, 0, 0]}>
            {bins.map((bin, i) => (
              <Cell
                key={i}
                fill={bin.lo >= p90 ? C.peak : bin.hi <= p10 ? C.floor : C.mid}
              />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
