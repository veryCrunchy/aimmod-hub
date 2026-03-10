import { displayScenarioType, scenarioTypeAccent } from "../lib/api";

type Props = {
  type?: string | null;
  className?: string;
};

export function ScenarioTypeBadge({ type: scenarioType, className = "" }: Props) {
  const label = displayScenarioType(scenarioType);
  if (!label) return null;
  const accent = scenarioTypeAccent(scenarioType);
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-[10px] uppercase tracking-wider ${accent} ${className}`}
    >
      {label}
    </span>
  );
}
