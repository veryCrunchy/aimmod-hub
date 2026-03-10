import { cn } from "../../lib/cn";

interface Props {
  label: string;
  field: string;
  sortField: string;
  sortDir: "asc" | "desc";
  onSort: (f: string) => void;
  className?: string;
}

export function SortableTh({ label, field, sortField, sortDir, onSort, className = "" }: Props) {
  const active = sortField === field;
  return (
    <th
      className={cn(
        "px-4 py-3 cursor-pointer select-none whitespace-nowrap transition-colors",
        active ? "text-text" : "text-muted hover:text-text",
        className
      )}
      onClick={() => onSort(field)}
    >
      {label}
      <span className={cn("ml-1 text-[10px]", active ? "" : "opacity-30")}>
        {active ? (sortDir === "asc" ? "↑" : "↓") : "↕"}
      </span>
    </th>
  );
}
