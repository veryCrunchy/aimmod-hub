import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import type { TopScenario } from "../../gen/aimmod/hub/v1/hub_pb";

const PALETTE = ["#79c997", "#b8ffe1", "#ffd956", "#c2a9ff", "#ff8787", "#6f9480"];

const C = {
  muted: "#a7c2b3",
  tooltipBg: "rgba(8,18,14,0.97)",
  tooltipBorder: "rgba(130,201,157,0.22)",
};

type Props = {
  topScenarios: TopScenario[];
};

type TooltipPayload = {
  name: string;
  value: number;
  payload: { color: string };
};

function CustomTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: TooltipPayload[];
}) {
  if (!active || !payload?.length) return null;
  const p = payload[0];
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
      <p style={{ color: p.payload.color, margin: 0 }}>{p.name}</p>
      <p style={{ color: C.muted, margin: "2px 0 0" }}>{p.value.toLocaleString()} runs</p>
    </div>
  );
}

export function ScenarioTypeChart({ topScenarios }: Props) {
  // Aggregate run counts by scenario type
  const typeMap = new Map<string, number>();
  for (const s of topScenarios) {
    const type = s.scenarioType?.trim() || "Other";
    typeMap.set(type, (typeMap.get(type) ?? 0) + s.runCount);
  }

  const data = [...typeMap.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([name, value], i) => ({ name, value, color: PALETTE[i % PALETTE.length] }));

  if (data.length === 0) return null;

  const total = data.reduce((s, d) => s + d.value, 0);

  return (
    <div className="flex items-center gap-6">
      <div style={{ width: 120, height: 120, flexShrink: 0 }}>
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={data}
              cx="50%"
              cy="50%"
              innerRadius={34}
              outerRadius={54}
              paddingAngle={2}
              dataKey="value"
              strokeWidth={0}
            >
              {data.map((entry, i) => (
                <Cell key={i} fill={entry.color} />
              ))}
            </Pie>
            <Tooltip content={<CustomTooltip />} />
          </PieChart>
        </ResponsiveContainer>
      </div>

      <div className="grid gap-2 flex-1 min-w-0">
        {data.slice(0, 5).map((entry) => {
          const pct = Math.round((entry.value / total) * 100);
          return (
            <div key={entry.name} className="flex items-center gap-2 min-w-0">
              <span
                className="shrink-0 h-2 w-2 rounded-full"
                style={{ background: entry.color }}
              />
              <span className="text-[11px] text-muted truncate flex-1">{entry.name}</span>
              <span className="text-[11px] shrink-0" style={{ color: entry.color }}>
                {pct}%
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
