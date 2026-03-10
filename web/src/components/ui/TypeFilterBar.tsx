import { cn } from "../../lib/cn";
import { displayScenarioType } from "../../lib/api";

interface Props {
  types: string[];
  active: string | null;
  onChange: (t: string | null) => void;
  className?: string;
}

export function TypeFilterBar({ types, active, onChange, className = "" }: Props) {
  if (types.length === 0) return null;
  return (
    <div className={cn("mb-4 flex flex-wrap gap-2", className)}>
      <FilterChip active={active === null} onClick={() => onChange(null)}>All types</FilterChip>
      {types.map((type) => (
        <FilterChip key={type} active={active === type} onClick={() => onChange(type)}>
          {displayScenarioType(type) ?? type}
        </FilterChip>
      ))}
    </div>
  );
}

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "rounded-full border px-3 py-1 text-[11px] uppercase tracking-wider transition-colors",
        active
          ? "border-cyan/40 bg-cyan/10 text-cyan"
          : "border-line text-muted hover:border-line hover:text-text"
      )}
    >
      {children}
    </button>
  );
}
