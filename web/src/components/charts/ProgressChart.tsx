import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { RunPreview } from "../../gen/aimmod/hub/v1/hub_pb";

const C = {
  score: "#ffd700",
  acc: "#00b4ff",
  muted: "#a7c2b3",
  grid: "rgba(130,201,157,0.10)",
  tooltipBg: "rgba(8,18,14,0.97)",
  tooltipBorder: "rgba(130,201,157,0.22)",
};

type DataPoint = {
  date: string;
  score: number;
  accuracy: number;
  label: string;
};

type TooltipProps = {
  active?: boolean;
  payload?: { color: string; name: string; value: number }[];
  label?: string;
};

function CustomTooltip({ active, payload, label }: TooltipProps) {
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
      }}
    >
      <p style={{ color: C.muted, marginBottom: 6, fontSize: 11 }}>{label}</p>
      {payload.map((p) => (
        <p key={p.name} style={{ color: p.color, marginBottom: 2 }}>
          {p.name === "Score"
            ? Math.round(p.value).toLocaleString()
            : `${p.value.toFixed(1)}%`}
          <span style={{ color: C.muted, fontSize: 10, marginLeft: 6 }}>{p.name}</span>
        </p>
      ))}
    </div>
  );
}

type Props = {
  runs: RunPreview[];
  showScore?: boolean;
  showAccuracy?: boolean;
};

export function ProgressChart({ runs, showScore = true, showAccuracy = true }: Props) {
  if (runs.length < 2) return null;

  const data: DataPoint[] = runs.map((r) => ({
    date: new Date(r.playedAtIso).toLocaleDateString(undefined, { month: "short", day: "numeric" }),
    score: r.score,
    accuracy: parseFloat(r.accuracy.toFixed(1)),
    label: r.playedAtIso,
  }));

  const avgScore = data.reduce((s, d) => s + d.score, 0) / data.length;
  const avgAcc = data.reduce((s, d) => s + d.accuracy, 0) / data.length;

  const minScore = Math.min(...data.map((d) => d.score));
  const maxScore = Math.max(...data.map((d) => d.score));
  const scorePad = (maxScore - minScore) * 0.1 || maxScore * 0.05;

  return (
    <ResponsiveContainer width="100%" height={220}>
      <LineChart data={data} margin={{ top: 12, right: 16, bottom: 0, left: 4 }}>
        <CartesianGrid stroke={C.grid} strokeDasharray="3 4" vertical={false} />

        <XAxis
          dataKey="date"
          tick={{ fill: C.muted, fontSize: 10, fontFamily: "JetBrains Mono, monospace" }}
          tickLine={false}
          axisLine={false}
          interval="preserveStartEnd"
        />

        {showScore && (
          <YAxis
            yAxisId="score"
            orientation="left"
            tick={{ fill: C.score, fontSize: 10, fontFamily: "JetBrains Mono, monospace" }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v: number) => Math.round(v).toLocaleString()}
            domain={[Math.max(0, minScore - scorePad), maxScore + scorePad]}
            width={52}
          />
        )}

        {showAccuracy && (
          <YAxis
            yAxisId="acc"
            orientation="right"
            tick={{ fill: C.acc, fontSize: 10, fontFamily: "JetBrains Mono, monospace" }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v: number) => `${v}%`}
            domain={[(min: number) => Math.max(0, Math.floor(min * 0.94)), 100]}
            width={36}
          />
        )}

        <Tooltip content={<CustomTooltip />} />

        {showScore && (
          <ReferenceLine
            yAxisId="score"
            y={avgScore}
            stroke="rgba(255,215,0,0.2)"
            strokeDasharray="5 3"
          />
        )}
        {showAccuracy && (
          <ReferenceLine
            yAxisId="acc"
            y={avgAcc}
            stroke="rgba(0,180,255,0.2)"
            strokeDasharray="5 3"
          />
        )}

        {showScore && (
          <Line
            yAxisId="score"
            type="monotone"
            dataKey="score"
            stroke={C.score}
            strokeWidth={2}
            dot={{ fill: C.score, r: 2, strokeWidth: 0 }}
            activeDot={{ fill: C.score, r: 4, strokeWidth: 0 }}
            name="Score"
          />
        )}

        {showAccuracy && (
          <Line
            yAxisId="acc"
            type="monotone"
            dataKey="accuracy"
            stroke={C.acc}
            strokeWidth={2}
            dot={{ fill: C.acc, r: 2, strokeWidth: 0 }}
            activeDot={{ fill: C.acc, r: 4, strokeWidth: 0 }}
            name="Accuracy"
          />
        )}
      </LineChart>
    </ResponsiveContainer>
  );
}
