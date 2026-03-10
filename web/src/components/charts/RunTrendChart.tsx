import {
  Area,
  AreaChart,
  CartesianGrid,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { RunPreview } from "../../gen/aimmod/hub/v1/hub_pb";

const C = {
  cyan: "#b8ffe1",
  muted: "#a7c2b3",
  grid: "rgba(130,201,157,0.10)",
  tooltipBg: "rgba(8,18,14,0.97)",
  tooltipBorder: "rgba(130,201,157,0.22)",
};

type Props = {
  runs: RunPreview[];
};

type TooltipPayload = {
  value: number;
};

function CustomTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean;
  payload?: TooltipPayload[];
  label?: string;
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
        maxWidth: 200,
      }}
    >
      <p style={{ color: C.muted, marginBottom: 4, fontSize: 11 }}>{label}</p>
      <p style={{ color: C.cyan }}>{payload[0].value.toFixed(1)}%</p>
    </div>
  );
}

export function RunTrendChart({ runs }: Props) {
  if (runs.length < 2) return null;

  // Oldest first for chronological display
  const data = [...runs]
    .reverse()
    .map((run) => ({
      accuracy: parseFloat(run.accuracy.toFixed(1)),
      scenario: run.scenarioName,
      date: new Date(run.playedAtIso).toLocaleDateString(undefined, { month: "short", day: "numeric" }),
    }));

  const avg = parseFloat((data.reduce((sum, d) => sum + d.accuracy, 0) / data.length).toFixed(1));

  return (
    <ResponsiveContainer width="100%" height={200}>
      <AreaChart data={data} margin={{ top: 12, right: 16, bottom: 0, left: 4 }}>
        <defs>
          <linearGradient id="accTrendGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor={C.cyan} stopOpacity={0.16} />
            <stop offset="95%" stopColor={C.cyan} stopOpacity={0} />
          </linearGradient>
        </defs>

        <CartesianGrid stroke={C.grid} strokeDasharray="3 4" vertical={false} />
        <XAxis
          dataKey="date"
          tick={{ fill: C.muted, fontSize: 10, fontFamily: "JetBrains Mono, monospace" }}
          tickLine={false}
          axisLine={false}
          interval="preserveStartEnd"
        />
        <YAxis
          tick={{ fill: C.muted, fontSize: 10, fontFamily: "JetBrains Mono, monospace" }}
          tickLine={false}
          axisLine={false}
          tickFormatter={(v: number) => `${v}%`}
          domain={[
            (min: number) => Math.max(0, Math.floor(min * 0.92)),
            100,
          ]}
          width={38}
        />

        <Tooltip content={<CustomTooltip />} />

        <ReferenceLine
          y={avg}
          stroke="rgba(184,255,225,0.28)"
          strokeDasharray="5 3"
          label={{
            value: `avg ${avg}%`,
            position: "insideTopRight",
            fill: "rgba(184,255,225,0.5)",
            fontSize: 10,
            fontFamily: "JetBrains Mono, monospace",
          }}
        />

        <Area
          type="monotone"
          dataKey="accuracy"
          stroke={C.cyan}
          strokeWidth={2}
          fill="url(#accTrendGrad)"
          dot={{ fill: C.cyan, r: 2, strokeWidth: 0 }}
          activeDot={{ fill: C.cyan, r: 4, strokeWidth: 0 }}
          name="Accuracy"
        />
      </AreaChart>
    </ResponsiveContainer>
  );
}
