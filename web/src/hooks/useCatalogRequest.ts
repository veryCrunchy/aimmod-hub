import { useEffect, useRef, useState } from "react";
import { catalogCache, catalogRequest } from "../lib/osuCatalog";

export function useCatalogRequest<T>(key: string | null, request: (signal: AbortSignal) => Promise<T>, delay = 300) {
  const loader = useRef(request);
  loader.current = request;
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<{ key: string; attempt: number; data?: T; error?: boolean }>();
  useEffect(() => {
    if (key === null) return;
    const controller = new AbortController();
    const run = loader.current;
    const timer = setTimeout(() => {
      catalogRequest(key, controller.signal, run).then(data => {
        if (!controller.signal.aborted) setState({ key, attempt, data });
      }).catch(() => {
        if (!controller.signal.aborted) setState({ key, attempt, error: true });
      });
    }, catalogCache.get(key) !== undefined ? 0 : delay);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [key, attempt, delay]);
  const current = state?.key === key && state.attempt === attempt ? state : undefined;
  return { data: current?.data, error: current?.error, loading: key !== null && !current,
    retry: () => { if (key) catalogCache.delete(key); setAttempt(value => value + 1); } };
}
