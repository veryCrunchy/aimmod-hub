import { forwardRef, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { cn } from "../lib/cn";
import { searchHub, type HubSearchResponse } from "../lib/api";
import { ScenarioTypeBadge } from "./ScenarioTypeBadge";

const quickNavItems = [
  { to: "/community", label: "Browse scenarios & players", sub: "All uploaded data, sorted by activity" },
  { to: "/learn", label: "Aim training guides", sub: "Mechanics, sensitivity, and practice" },
  { to: "/leaderboard", label: "Global leaderboard", sub: "All-time records and top 100 scores" },
  { to: "/replays", label: "Replay library", sub: "Watch replays and mouse paths" },
  { to: "/app/osu", label: "AimMod for osu!", sub: "Native analysis and coaching for Windows and Linux" },
  { to: "/app/kovaaks", label: "AimMod for KovaaK's", sub: "Live HUDs, replays, and coaching for Windows" },
];

type DropdownItem = {
  kind: "scenario" | "player" | "run" | "benchmark";
  title: string;
  subtitle: string;
  to: string;
  meta: string;
  badge: string;
};

function buildItems(results: HubSearchResponse): DropdownItem[] {
  return [
    ...results.scenarios.slice(0, 3).map((s) => ({
      kind: "scenario" as const,
      title: s.scenarioName,
      subtitle: "scenario",
      to: `/scenarios/${s.scenarioSlug}`,
      meta: `${s.runCount.toLocaleString()} runs`,
      badge: s.scenarioType,
    })),
    ...results.profiles.slice(0, 3).map((p) => ({
      kind: "player" as const,
      title: p.userDisplayName || p.userHandle,
      subtitle: `@${p.userHandle}`,
      to: `/profiles/${p.userHandle}`,
      meta: `${p.runCount.toLocaleString()} runs`,
      badge: p.primaryScenarioType,
    })),
    ...(results.benchmarks ?? []).slice(0, 2).map((b) => ({
      kind: "benchmark" as const,
      title: b.benchmarkName,
      subtitle: b.benchmarkAuthor ? `by ${b.benchmarkAuthor}` : "benchmark",
      to: `/benchmarks/${b.benchmarkId}`,
      meta: `${b.playerCount} player${b.playerCount !== 1 ? "s" : ""}`,
      badge: b.benchmarkType,
    })),
    ...results.replays.slice(0, 2).map((r) => ({
      kind: "run" as const,
      title: r.scenarioName,
      subtitle: `${r.userDisplayName || r.userHandle} · replay`,
      to: `/runs/${r.publicRunID || r.sessionID}`,
      meta: r.hasVideo ? "video replay" : "mouse path",
      badge: r.scenarioType,
    })),
    ...results.runs.slice(0, 2).map((r) => ({
      kind: "run" as const,
      title: r.scenarioName,
      subtitle: `${r.userDisplayName || r.userHandle} · run`,
      to: `/runs/${r.publicRunID || r.sessionID}`,
      meta: `${Math.round(r.score).toLocaleString()} score`,
      badge: r.scenarioType,
    })),
  ].slice(0, 8);
}

export const HeaderSearch = forwardRef<HTMLInputElement, { game?: "osu" | "kovaaks" }>(function HeaderSearch({ game = "kovaaks" }, ref) {
  const navigate = useNavigate();
  const [value, setValue] = useState("");
  const [results, setResults] = useState<HubSearchResponse | null>(null);
  const [open, setOpen] = useState(false);
  const [focused, setFocused] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const [searchState, setSearchState] = useState<"idle" | "loading" | "error">("idle");
  const containerRef = useRef<HTMLDivElement>(null);
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);

  // Debounced search
  useEffect(() => {
    if (game === "osu") return;
    let cancelled = false;
    const q = value.trim();
    if (!q) {
      setResults(null);
      setOpen(false);
      setSearchState("idle");
      return;
    }
    setResults(null);
    setSearchState("loading");
    const tid = setTimeout(() => {
      void searchHub(q)
        .then((r) => {
          if (cancelled) return;
          setResults(r);
          setOpen(true);
          setSearchState("idle");
        })
        .catch(() => { if (!cancelled) setSearchState("error"); });
    }, 300);
    return () => { cancelled = true; clearTimeout(tid); };
  }, [value, game]);

  // Close on outside click
  useEffect(() => {
    function handler(e: MouseEvent) {
      if (!containerRef.current?.contains(e.target as Node)) {
        setOpen(false);
        setFocused(false);
      }
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  // Scroll active item into view
  useEffect(() => {
    itemRefs.current[activeIndex]?.scrollIntoView({ block: "nearest" });
  }, [activeIndex]);

  const items = results ? buildItems(results) : [];
  const showQuickNav = focused && !value.trim() && !open;

  function go(to: string) {
    navigate(to);
    setOpen(false);
    setFocused(false);
    setValue("");
    setResults(null);
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (!open && items.length) {
        setOpen(true);
        return;
      }
      if (items.length) setActiveIndex((i) => (i + 1) % items.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      if (items.length) setActiveIndex((i) => (i - 1 + items.length) % items.length);
    } else if (e.key === "Escape") {
      setOpen(false);
      setFocused(false);
      setActiveIndex(0);
    } else if (e.key === "Enter") {
      if (open && items[activeIndex]) {
        e.preventDefault();
        go(items[activeIndex].to);
      } else {
        const q = value.trim();
        if (q) {
          navigate(`/search?q=${encodeURIComponent(q)}`);
          setOpen(false);
        }
      }
    }
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const q = value.trim();
    navigate(q ? `/search?q=${encodeURIComponent(q)}` : "/search");
    setOpen(false);
  }

  if (game === "osu") {
    return <form role="search" className="flex min-w-0 gap-2" onSubmit={e => { e.preventDefault(); navigate(`/osu/community?q=${encodeURIComponent(value.trim())}`); }}>
      <input ref={ref} type="search" aria-label="Search osu! plays" placeholder="Search beatmaps, players, or mappers" value={value} onChange={e => setValue(e.target.value)} className="min-w-0 flex-1 rounded-md border border-line bg-panel px-3 py-2.5 text-sm text-text placeholder:text-muted" />
      <button type="submit" className="min-h-10 rounded-md border border-line bg-panel px-3 text-sm">Search</button>
    </form>;
  }

  return (
    <div ref={containerRef} onBlur={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node)) { setFocused(false); setOpen(false); } }} className="relative flex min-w-0 items-center gap-2 xl:min-w-[220px]">
      <form role="search" onSubmit={handleSubmit} className="flex min-w-0 flex-1 items-center gap-2">
        <input
          aria-label="Search AimMod Hub"
          role="combobox"
          aria-autocomplete="list"
          aria-expanded={focused && open && items.length > 0}
          aria-controls={focused && open && items.length > 0 ? "hub-search-results" : undefined}
          aria-activedescendant={focused && open && items[activeIndex] ? `hub-result-${activeIndex}` : undefined}
          ref={ref}
          value={value}
          onChange={(e) => {
            setValue(e.target.value);
            setActiveIndex(0);
          }}
          onFocus={() => {
            setFocused(true);
            if (items.length) setOpen(true);
          }}
          onKeyDown={handleKeyDown}
          placeholder="Search players, scenarios, replays"
          className="min-w-0 flex-1 rounded-md border border-line bg-panel px-3 py-2.5 text-sm text-text transition-colors placeholder:text-muted focus:border-cyan"
        />
        <button
          type="submit"
          className="inline-flex min-h-10 shrink-0 items-center justify-center rounded-md border border-line bg-panel px-3 text-sm text-text hover:border-line-strong"
        >
          Search
        </button>
      </form>

      {showQuickNav && (
        <div className="absolute left-0 right-0 top-[calc(100%+6px)] z-50 max-h-[60vh] overflow-auto rounded-md border border-line bg-panel-strong shadow-[0_16px_48px_rgba(0,0,0,0.6)] backdrop-blur-xl">
          <div className="border-b border-line/50 px-4 py-2.5">
            <span className="text-[10px] uppercase tracking-normal text-muted">Quick navigation</span>
          </div>
          {quickNavItems.map((item) => (
            <button
              key={item.to}
              type="button"
              onClick={(e) => {
                e.preventDefault();
                go(item.to);
              }}
              className="flex w-full items-center justify-between gap-3 border-b border-line/40 px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-[rgba(121,201,151,0.08)]"
            >
              <div className="min-w-0">
                <div className="text-[13px] text-text">{item.label}</div>
                <div className="mt-0.5 text-[11px] text-muted">{item.sub}</div>
              </div>
              <span className="shrink-0 text-[11px] text-muted-2">→</span>
            </button>
          ))}
          <div className="border-t border-line/50 px-4 py-2 text-[11px] text-muted-2">
            Type to search players, scenarios, and replays
          </div>
        </div>
      )}

      {focused && value.trim() && (searchState !== "idle" || (results && items.length === 0)) && (
        <div role="status" className="absolute left-0 right-0 top-full mt-2 rounded-md border border-line bg-panel-strong p-4 text-sm text-muted">
          {searchState === "loading" ? "Searching..." : searchState === "error" ? "Search unavailable. Try again shortly." : "No matches found."}
        </div>
      )}
      {focused && open && items.length > 0 && (
        <div id="hub-search-results" role="listbox" aria-label="Search results" className="absolute left-0 right-0 top-[calc(100%+6px)] z-50 max-h-[min(420px,60vh)] overflow-y-auto overscroll-contain rounded-md border border-line bg-panel-strong shadow-xl">
          {items.map((item, i) => (
            <button
              key={item.to}
              ref={(el) => {
                itemRefs.current[i] = el;
              }}
              type="button"
              role="option"
              id={`hub-result-${i}`}
              aria-selected={i === activeIndex}
              onClick={(e) => {
                e.preventDefault();
                go(item.to);
              }}
              onMouseEnter={() => setActiveIndex(i)}
              className={cn(
                "flex w-full items-center justify-between gap-3 border-b border-line/60 px-4 py-3 text-left transition-colors last:border-b-0",
                i === activeIndex
                  ? "bg-[rgba(121,201,151,0.1)]"
                  : "hover:bg-[rgba(255,255,255,0.03)]"
              )}
            >
              <div className="min-w-0">
                <div className="flex items-center gap-2 text-[10px] uppercase tracking-normal text-muted">
                  <span>{item.kind}</span>
                  {item.kind === "player" && (
                    <span className="text-muted-2">{item.subtitle}</span>
                  )}
                </div>
                <div className="truncate text-[13px] text-text">{item.title}</div>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <ScenarioTypeBadge type={item.badge} />
                <span className="text-[11px] text-muted">{item.meta}</span>
              </div>
            </button>
          ))}
          <button
            type="button"
            onClick={(e) => {
              e.preventDefault();
              navigate(`/search?q=${encodeURIComponent(value.trim())}`);
              setOpen(false);
            }}
            role="option"
            aria-selected={false}
            className="flex w-full items-center justify-center gap-2 border-t border-line px-4 py-2.5 text-[12px] text-muted transition-colors hover:bg-[rgba(255,255,255,0.03)] hover:text-text"
          >
            See all results for "{value}"
          </button>
        </div>
      )}
    </div>
  );
});
