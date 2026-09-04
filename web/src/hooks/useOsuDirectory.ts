import { useEffect, useState } from "react";
import { fetchOsuCommunity, type OsuSharedReplay } from "../lib/osuCommunity";

let cached: { expires: number; items: OsuSharedReplay[] } | undefined;
let pending: Promise<OsuSharedReplay[]> | undefined;

function loadDirectory() {
  if (cached && cached.expires > Date.now()) return Promise.resolve(cached.items);
  if (!pending) {
    pending = fetchOsuCommunity(100).then(items => {
      cached = { items, expires: Date.now() + 60_000 };
      return items;
    }).finally(() => { pending = undefined; });
  }
  return pending;
}

export function useOsuDirectory() {
  const [items, setItems] = useState<OsuSharedReplay[] | null>(cached && cached.expires > Date.now() ? cached.items : null);
  const [error, setError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let cancelled = false;
    setError(false);
    void loadDirectory().then(value => { if (!cancelled) setItems(value); })
      .catch(() => { if (!cancelled) setError(true); });
    return () => { cancelled = true; };
  }, [attempt]);
  return { items, error, retry: () => { cached = undefined; setItems(null); setAttempt(value => value + 1); } };
}
