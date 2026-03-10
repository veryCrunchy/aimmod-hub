import {
  PolarAngleAxis,
  PolarGrid,
  PolarRadiusAxis,
  Radar,
  RadarChart,
  ResponsiveContainer,
  Tooltip,
} from "recharts";
import type { TypeProfileBand } from "../../gen/aimmod/hub/v1/hub_pb";
import { displayScenarioType } from "../../lib/api";

const C = {
  fill: "rgba(0,181,255,0.18)",
  stroke: "#00b4ff",
  grid: "rgba(130,201,157,0.12)",
  axis: "#a7c2b3",
  tooltipBg: "rgba(8,18,14,0.97)",
  tooltipBorder: "rgba(130,201,157,0.22)",
  communityFill: "rgba(255,215,0,0.06)",
  communityStroke: "rgba(255,215,0,0.3)",
};

type RadarPoint = {
  type: string;
  label: string;
  percentile: number;
  community: number;
  avgAccuracy: number;
  communityAvgAccuracy: number;
  runCount: number;
};

type TooltipProps = {
  active?: boolean;
  payload?: { payload: RadarPoint }[];
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
        padding: "10px 14px",
        fontFamily: "JetBrains Mono, monospace",
        fontSize: 12,
        minWidth: 160,
      }}
    >
      <p style={{ color: "#f3fff8", fontWeight: 600, marginBottom: 6 }}>{d.label}</p>
      <p style={{ color: C.stroke, marginBottom: 2 }}>
        {Math.round(d.percentile)}th percentile
      </p>
      <p style={{ color: C.axis, fontSize: 11, marginBottom: 2 }}>
        Your avg: {d.avgAccuracy.toFixed(1)}%
      </p>
      <p style={{ color: C.axis, fontSize: 11, marginBottom: 2 }}>
        Community: {d.communityAvgAccuracy.toFixed(1)}%
      </p>
      <p style={{ color: "rgba(167,194,179,0.5)", fontSize: 10 }}>
        {d.runCount.toLocaleString()} runs
      </p>
    </div>
  );
}

type Props = {
  bands: TypeProfileBand[];
};

export function AimProfileRadar({ bands }: Props) {
  if (bands.length < 2) return null;

  const data: RadarPoint[] = bands.map((b) => ({
    type: b.scenarioType,
    label: displayScenarioType(b.scenarioType) ?? b.scenarioType,
    percentile: Math.round(b.accuracyPercentile),
    community: 50,
    avgAccuracy: b.avgAccuracy,
    communityAvgAccuracy: b.communityAvgAccuracy,
    runCount: b.runCount,
  }));

  return (
    <ResponsiveContainer width="100%" height={280}>
      <RadarChart data={data} margin={{ top: 16, right: 32, bottom: 16, left: 32 }}>
        <PolarGrid stroke={C.grid} />
        <PolarAngleAxis
          dataKey="label"
          tick={{ fill: C.axis, fontSize: 11, fontFamily: "JetBrains Mono, monospace" }}
        />
        <PolarRadiusAxis
          angle={90}
          domain={[0, 100]}
          tickCount={5}
          tick={{ fill: "rgba(167,194,179,0.4)", fontSize: 9, fontFamily: "JetBrains Mono, monospace" }}
          tickFormatter={(v: number) => `${v}`}
        />
        <Tooltip content={<CustomTooltip />} />
        {/* community 50th pct reference */}
        <Radar
          name="Community avg"
          dataKey="community"
          stroke={C.communityStroke}
          fill={C.communityFill}
          strokeDasharray="4 3"
          dot={false}
        />
        {/* player */}
        <Radar
          name="You"
          dataKey="percentile"
          stroke={C.stroke}
          fill={C.fill}
          strokeWidth={2}
          dot={{ fill: C.stroke, r: 3, strokeWidth: 0 }}
          activeDot={{ fill: C.stroke, r: 5, strokeWidth: 0 }}
        />
      </RadarChart>
    </ResponsiveContainer>
  );
}
