import { countryName } from "../lib/country";

/** Two-letter country code as a small tag; flag emoji do not render on every system. */
export function CountryTag({ code }: { code?: string | null }) {
  const c = code?.trim().toUpperCase();
  if (!c || !/^[A-Z]{2}$/.test(c)) return null;
  return (
    <abbr title={countryName(c)} className="shrink-0 rounded border border-line px-1 text-[10px] font-medium tracking-wide text-muted no-underline">
      {c}
    </abbr>
  );
}
