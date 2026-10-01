import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";

export type UrlStateSpec = Record<string, string>;

/**
 * Reads filters from a query string. Each key falls back to its default;
 * when `choices` lists the allowed values, anything else uses the default.
 */
export function readUrlState<T extends UrlStateSpec>(params: URLSearchParams, defaults: T, choices: Partial<Record<keyof T, readonly string[]>> = {}): T {
  const out = { ...defaults };
  for (const key of Object.keys(defaults) as (keyof T & string)[]) {
    const raw = params.get(key);
    if (raw === null) continue;
    const allowed = choices[key];
    if (allowed && !allowed.includes(raw)) continue;
    (out as Record<string, string>)[key] = raw.slice(0, 200);
  }
  return out;
}

/**
 * Applies changes to a query string. Values equal to their default are
 * removed so shared links stay short; other keys are kept untouched.
 */
export function writeUrlState<T extends UrlStateSpec>(current: URLSearchParams, defaults: T, changes: Partial<T>): URLSearchParams {
  const next = new URLSearchParams(current);
  for (const [key, value] of Object.entries(changes)) {
    if (value === undefined) continue;
    if (value === "" || value === defaults[key]) next.delete(key);
    else next.set(key, value);
  }
  return next;
}

/** Filter state kept in the URL so views can be shared and bookmarked. */
export function useUrlState<T extends UrlStateSpec>(defaults: T, choices: Partial<Record<keyof T, readonly string[]>> = {}) {
  const [params, setParams] = useSearchParams();
  // Defaults and choices are expected to be stable literals per page.
  const key = params.toString();
  const state = useMemo(() => readUrlState(new URLSearchParams(key), defaults, choices), [key]); // eslint-disable-line react-hooks/exhaustive-deps
  const update = useCallback((changes: Partial<T>, options: { push?: boolean } = {}) => {
    setParams((current) => writeUrlState(current, defaults, changes), { replace: !options.push });
  }, [setParams]); // eslint-disable-line react-hooks/exhaustive-deps
  return [state, update] as const;
}

export function intParam(value: string, fallback = 0, min = 0, max = Number.MAX_SAFE_INTEGER): number {
  const n = Number.parseInt(value, 10);
  if (!Number.isFinite(n)) return fallback;
  return Math.min(max, Math.max(min, n));
}
