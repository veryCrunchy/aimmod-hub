/** Flag emoji for a two-letter country code, or an empty string. */
export function countryFlag(code?: string | null): string {
  const c = code?.trim().toUpperCase() ?? "";
  if (!/^[A-Z]{2}$/.test(c)) return "";
  return String.fromCodePoint(...[...c].map((ch) => 0x1f1e6 + ch.charCodeAt(0) - 65));
}

/** Country name for a code, falling back to the code itself. */
export function countryName(code?: string | null): string {
  const c = code?.trim().toUpperCase() ?? "";
  if (!c) return "";
  try {
    return new Intl.DisplayNames(["en"], { type: "region" }).of(c) ?? c;
  } catch {
    return c;
  }
}
