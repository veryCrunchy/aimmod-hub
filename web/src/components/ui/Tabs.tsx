import { cn } from "../../lib/cn";

type TabsProps<T extends string> = {
  tabs: readonly (readonly [value: T, label: string])[];
  value: T;
  onChange: (value: T) => void;
  label: string;
  className?: string;
};

export function Tabs<T extends string>({ tabs, value, onChange, label, className }: TabsProps<T>) {
  return (
    <div role="tablist" aria-label={label} className={cn("flex gap-1 overflow-x-auto border-b border-line", className)}>
      {tabs.map(([tab, text]) => (
        <button
          key={tab}
          type="button"
          role="tab"
          aria-selected={value === tab}
          onClick={() => onChange(tab)}
          className={cn(
            "-mb-px min-h-10 whitespace-nowrap border-b-2 px-3 text-sm transition-colors",
            value === tab ? "border-mint font-medium text-text" : "border-transparent text-muted hover:text-text",
          )}
        >
          {text}
        </button>
      ))}
    </div>
  );
}
