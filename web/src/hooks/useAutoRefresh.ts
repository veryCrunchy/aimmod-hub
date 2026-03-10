import { useEffect, useRef } from "react";

/**
 * Calls `refresh` on a fixed interval without triggering a loading state.
 * The ref pattern ensures we always call the latest version of the function
 * without needing it in the dependency array.
 */
export function useAutoRefresh(refresh: () => void, intervalMs = 30_000) {
  const ref = useRef(refresh);
  ref.current = refresh;
  useEffect(() => {
    const id = setInterval(() => ref.current(), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
}
