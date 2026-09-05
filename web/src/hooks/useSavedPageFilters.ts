import { useEffect, useRef } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { loadPageFilters, savePageFilters, supportsSavedFilters } from "../lib/savedPageFilters";

export function useSavedPageFilters() {
  const location = useLocation();
  const navigate = useNavigate();
  const previousPath = useRef<string | null>(null);
  const pendingRestore = useRef<{ path: string; search: string } | null>(null);
  const path = location.pathname.replace(/\/$/, "") || "/";
  // BrowserRouter subscribes to history in a parent layout effect. Restoring
  // from a child layout effect can change the URL before that subscription,
  // leaving controls on the original empty search. Wait until passive effects.
  useEffect(() => {
    if (previousPath.current !== path) {
      previousPath.current = path;
      pendingRestore.current = null;
      if (!location.search && supportsSavedFilters(path)) {
        let restore = "";
        try { restore = loadPageFilters(window.localStorage, path); } catch { /* Storage disabled. */ }
        if (restore) {
          pendingRestore.current = { path, search: `?${restore}` };
          navigate({ pathname: location.pathname, search: `?${restore}`, hash: location.hash }, { replace: true });
          return;
        }
      }
    }
    if (pendingRestore.current?.path === path) {
      // StrictMode may repeat this effect before navigation is reflected in
      // router state. Never persist that intermediate empty query.
      if (location.search !== pendingRestore.current.search) return;
      pendingRestore.current = null;
    }
    try { savePageFilters(window.localStorage, path, location.search); } catch { /* Storage disabled. */ }
  }, [path, location.pathname, location.search, location.hash, navigate]);
}
