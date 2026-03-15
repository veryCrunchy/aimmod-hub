import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { cn } from "../lib/cn";
import { lookupExternalUser, searchPlayers, type PlayerSearchResult } from "../lib/api";

function PlayerAvatar({ url, name }: { url: string; name: string }) {
  if (!url) {
    return (
      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-white/8 text-[10px] text-muted">
        {name.slice(0, 1).toUpperCase()}
      </span>
    );
  }
  return (
    <img
      src={url}
      alt=""
      className="h-7 w-7 shrink-0 rounded-full border border-white/10 object-cover"
    />
  );
}

function AimModBadge() {
  return (
    <span className="shrink-0 rounded-full border border-cyan/25 bg-cyan/8 px-1.5 py-0.5 text-[9px] font-medium uppercase tracking-[0.08em] text-cyan">
      AimMod
    </span>
  );
}

function CountryFlag({ country }: { country: string }) {
  if (!country) return null;
  return (
    <span className="shrink-0 text-[11px] text-muted/50" title={country.toUpperCase()}>
      {country.toUpperCase()}
    </span>
  );
}

export function PlayerLookup({ placeholder }: { placeholder?: string }) {
  const [value, setValue] = useState("");
  const [results, setResults] = useState<PlayerSearchResult[]>([]);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const navigate = useNavigate();

  // Debounced search
  useEffect(() => {
    const q = value.trim();
    if (!q) {
      setResults([]);
      setOpen(false);
      setError(null);
      return;
    }
    const tid = setTimeout(() => {
      void searchPlayers(q).then((r) => {
        setResults(r);
        setOpen(r.length > 0);
        setActiveIndex(0);
      });
    }, 250);
    return () => clearTimeout(tid);
  }, [value]);

  // Close on outside click
  useEffect(() => {
    function handler(e: MouseEvent) {
      if (!containerRef.current?.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  // Scroll active item into view
  useEffect(() => {
    itemRefs.current[activeIndex]?.scrollIntoView({ block: "nearest" });
  }, [activeIndex]);

  function selectResult(result: PlayerSearchResult) {
    setOpen(false);
    setValue("");
    setResults([]);
    if (result.type === "aimmod" && result.handle) {
      void navigate(`/profiles/${result.handle}`);
    } else if (result.steamId) {
      void navigate(`/u/${encodeURIComponent(result.steamId)}`);
    } else {
      void navigate(`/u/kovaaks/${encodeURIComponent(result.username)}`);
    }
  }

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const q = value.trim();
    if (!q) return;

    // If an item is highlighted, select it
    if (open && results[activeIndex]) {
      selectResult(results[activeIndex]);
      return;
    }

    // Otherwise do a direct resolution lookup
    setLoading(true);
    setError(null);
    try {
      const result = await lookupExternalUser(q);
      if (result.resolvedSteamId) {
        void navigate(`/u/${encodeURIComponent(result.resolvedSteamId)}`);
      } else if (result.kovaaksUsername) {
        void navigate(`/u/kovaaks/${encodeURIComponent(result.kovaaksUsername)}`);
      } else {
        setError("No player found for that query.");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Lookup failed.");
    } finally {
      setLoading(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (!open || results.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => (i + 1) % results.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => (i - 1 + results.length) % results.length);
    } else if (e.key === "Escape") {
      setOpen(false);
      setActiveIndex(0);
    }
  }

  return (
    <div ref={containerRef} className="relative w-full">
      <form onSubmit={(e) => { void handleSubmit(e); }} className="flex items-center gap-2">
        <input
          ref={inputRef}
          value={value}
          onChange={(e) => { setValue(e.target.value); setError(null); }}
          onFocus={() => { if (results.length > 0) setOpen(true); }}
          onKeyDown={handleKeyDown}
          placeholder={placeholder ?? "Search by KovaaK's username, Steam name, or Steam ID…"}
          autoComplete="off"
          className="min-w-0 flex-1 rounded-xl border border-line bg-white/3 px-3.5 py-2 text-[13px] text-text placeholder:text-muted/50 outline-none focus:border-cyan/40 focus:bg-white/[0.045] transition-colors"
        />
        <button
          type="submit"
          disabled={!value.trim() || loading}
          className="shrink-0 rounded-xl border border-line bg-white/4 px-4 py-2 text-[12px] text-text transition-colors hover:border-cyan/30 hover:bg-white/6 disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {loading ? "…" : "Go"}
        </button>
      </form>

      {error && (
        <p className="mt-1.5 text-[11px] text-red-400/80">{error}</p>
      )}

      {open && results.length > 0 && (
        <div className="absolute left-0 right-0 top-[calc(100%+6px)] z-50 max-h-[min(400px,60vh)] overflow-y-auto overscroll-contain rounded-[14px] border border-line bg-[rgba(6,16,12,0.98)] shadow-[0_16px_48px_rgba(0,0,0,0.55)] backdrop-blur-xl">
          {results.map((result, i) => {
            const name = result.displayName || result.username;
            const sub = result.type === "aimmod"
              ? `@${result.handle}`
              : result.username !== result.displayName && result.username
                ? result.username
                : null;

            return (
              <button
                key={result.steamId || result.username}
                ref={(el) => { itemRefs.current[i] = el; }}
                type="button"
                onMouseDown={(e) => { e.preventDefault(); selectResult(result); }}
                onMouseEnter={() => setActiveIndex(i)}
                className={cn(
                  "flex w-full items-center gap-3 border-b border-white/5 px-4 py-2.5 text-left transition-colors last:border-b-0",
                  i === activeIndex
                    ? "bg-[rgba(121,201,151,0.09)]"
                    : "hover:bg-white/[0.03]",
                )}
              >
                <PlayerAvatar url={result.avatarUrl} name={name} />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="truncate text-[13px] font-medium text-text">{name}</span>
                    {result.type === "aimmod" && <AimModBadge />}
                  </div>
                  {sub && (
                    <div className="text-[11px] text-muted/60 truncate">{sub}</div>
                  )}
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {result.runCount > 0 && (
                    <span className="text-[11px] text-muted/50 tabular-nums">
                      {result.runCount.toLocaleString()} runs
                    </span>
                  )}
                  <CountryFlag country={result.country} />
                </div>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
