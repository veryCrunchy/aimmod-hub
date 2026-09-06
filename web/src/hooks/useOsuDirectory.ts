import { useEffect, useState } from "react";
import { fetchOsuCommunity, type OsuSharedReplay } from "../lib/osuCommunity";

const caches = new Map<boolean, { expires: number; items: OsuSharedReplay[] }>();
const pendingQueries = new Map<boolean, Promise<OsuSharedReplay[]>>();

function loadDirectory(replaysOnly: boolean) {
  const cached = caches.get(replaysOnly);
  let pending = pendingQueries.get(replaysOnly);
  if (cached && cached.expires > Date.now()) return Promise.resolve(cached.items);
  if (!pending) {
    pending = fetchOsuCommunity(100, replaysOnly).then(items => {
      caches.set(replaysOnly, { items, expires: Date.now() + 60_000 });
      return items;
    }).finally(() => { pendingQueries.delete(replaysOnly); });
    pendingQueries.set(replaysOnly, pending);
  }
  return pending;
}

export function useOsuDirectory(replaysOnly = false) {
  const cached = caches.get(replaysOnly);
  const [items, setItems] = useState<OsuSharedReplay[] | null>(cached && cached.expires > Date.now() ? cached.items : null);
  const [error, setError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let cancelled = false;
    setError(false);
    void loadDirectory(replaysOnly).then(value => { if (!cancelled) setItems(value); })
      .catch(() => { if (!cancelled) setError(true); });
    return () => { cancelled = true; };
  }, [attempt, replaysOnly]);
  return { items, error, retry: () => { caches.delete(replaysOnly); setItems(null); setAttempt(value => value + 1); } };
}
