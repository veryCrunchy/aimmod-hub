import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { ContextWindow, TimelineSecond } from "../../gen/aimmod/hub/v1/hub_pb";

const C = {
  mint: "#79c997",
  cyan: "#b8ffe1",
  violet: "#c2a9ff",
  muted: "#a7c2b3",
  grid: "rgba(130,201,157,0.10)",
  tooltipBg: "rgba(8,18,14,0.97)",
  tooltipBorder: "rgba(130,201,157,0.22)",
};

type Props = {
  timeline: TimelineSecond[];
  contextWindows?: ContextWindow[];
};

type TooltipPayload = {
  name: string;
  value: number;
  color: string;
};

function CustomTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean;
  payload?: TooltipPayload[];
  label?: number;
}) {
  if (!active || !payload?.length) return null;
  return (
    <div
      style={{
        background: C.tooltipBg,
        border: `1px solid ${C.tooltipBorder}`,
        borderRadius: 12,
        padding: "10px 14px",
        fontFamily: "JetBrains Mono, monospace",
        fontSize: 12,
        minWidth: 140,
      }}
    >
      <p style={{ color: C.muted, marginBottom: 6 }}>{label}s</p>
      {payload.map((p) => (
        <p key={p.name} style={{ color: p.color, margin: "2px 0" }}>
          <span style={{ color: C.muted }}>{p.name}: </span>
          {p.name === "Accuracy" || p.name === "Dmg Eff" ? `${p.value}%` : p.value.toLocaleString()}
        </p>
      ))}
    </div>
  );
}

export function TimelineChart({ timeline, contextWindows = [] }: Props) {
  if (timeline.length === 0) return null;

  const data = timeline.map((pt) => ({
    t: pt.tSec,
    spm: Math.round(pt.spm),
    accuracy: parseFloat(pt.accuracy.toFixed(1)),
    damageEff: pt.damageEff > 0 ? parseFloat(pt.damageEff.toFixed(1)) : undefined,
  }));

  const hasDamageEff = data.some((d) => d.damageEff !== undefined && d.damageEff > 0);
  const maxSpm = Math.max(...data.map((d) => d.spm));

  const windowMarkers = contextWindows
    .map((cw) => ({
      x: Math.round(Number(cw.startMs) / 1000),
      label: cw.label || cw.windowType || "Moment",
    }))
    .filter((m) => m.x > 0);

  return (
    <ResponsiveContainer width="100%" height={260}>
      <ComposedChart data={data} margin={{ top: 16, right: 48, bottom: 0, left: 4 }}>
        <defs>
          <linearGradient id="scoreGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor={C.mint} stopOpacity={0.2} />
            <stop offset="95%" stopColor={C.mint} stopOpacity={0} />
          </linearGradient>
        </defs>

        <CartesianGrid
          stroke={C.grid}
          strokeDasharray="3 4"
          vertical={false}
        />
        <XAxis
          dataKey="t"
          tick={{ fill: C.muted, fontSize: 10, fontFamily: "JetBrains Mono, monospace" }}
          tickLine={false}
          axisLine={false}
          tickFormatter={(v: number) => `${v}s`}
        />
        <YAxis
          yAxisId="spm"
          tick={{ fill: C.muted, fontSize: 10, fontFamily: "JetBrains Mono, monospace" }}
          tickLine={false}
          axisLine={false}
          tickFormatter={(v: number) => (v >= 1000 ? `${Math.round(v / 1000)}k` : String(v))}
          width={44}
          domain={[0, Math.ceil((maxSpm * 1.15) / 1000) * 1000]}
        />
        <YAxis
          yAxisId="acc"
          orientation="right"
          tick={{ fill: C.muted, fontSize: 10, fontFamily: "JetBrains Mono, monospace" }}
          tickLine={false}
          axisLine={false}
          tickFormatter={(v: number) => `${v}%`}
          domain={[0, 100]}
          width={36}
        />

        <Tooltip content={<CustomTooltip />} />

        {windowMarkers.map((m, i) => (
          <ReferenceLine
            key={i}
            x={m.x}
            yAxisId="spm"
            stroke="rgba(194,169,255,0.35)"
            strokeDasharray="4 3"
            label={{
              value: "●",
              position: "top",
              fill: C.violet,
              fontSize: 8,
              fontFamily: "JetBrains Mono, monospace",
            }}
          />
        ))}

        <Area
          yAxisId="spm"
          type="monotone"
          dataKey="spm"
          stroke={C.mint}
          strokeWidth={2}
          fill="url(#scoreGrad)"
          dot={false}
          name="SPM"
        />
        <Line
          yAxisId="acc"
          type="monotone"
          dataKey="accuracy"
          stroke={C.cyan}
          strokeWidth={1.5}
          dot={false}
          name="Accuracy"
        />
        {hasDamageEff && (
          <Line
            yAxisId="acc"
            type="monotone"
            dataKey="damageEff"
            stroke={C.violet}
            strokeWidth={1}
            strokeDasharray="3 3"
            dot={false}
            name="Dmg Eff"
          />
        )}
      </ComposedChart>
    </ResponsiveContainer>
  );
}
