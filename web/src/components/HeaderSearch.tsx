import { forwardRef, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import type { QuickSearchResult } from "../gen/aimmod/hub/v1/hub_pb";
import { cn } from "../lib/cn";
import { quickSearch } from "../lib/api";
import { groupQuickResults, loadRecentSearches, quickResultHref, rememberSearch, type RecentSearch } from "../lib/quickSearch";
import { CountryTag } from "./CountryTag";
import { ScenarioTypeBadge } from "./ScenarioTypeBadge";

const quickNavItems = [
  { to: "/community", label: "Browse scenarios & players", sub: "All uploaded data, sorted by activity" },
  { to: "/benchmarks", label: "Benchmarks", sub: "Rank thresholds and leaderboards" },
  { to: "/leaderboard", label: "Global leaderboard", sub: "All-time records and top 100 scores" },
  { to: "/replays", label: "Replay library", sub: "Watch replays and mouse paths" },
  { to: "/learn", label: "Aim training guides", sub: "Mechanics, sensitivity, and practice" },
  { to: "/app/kovaaks", label: "AimMod for KovaaK's", sub: "The in-game mod for Windows" },
];

type DropdownItem = {
  kind: string;
  group: string;
  title: string;
  subtitle: string;
  to: string;
  meta: string;
  badge: string;
  imageUrl: string;
  country: string;
};

function resultMeta(result: Pick<QuickSearchResult, "kind" | "count">): string {
  const count = Number(result.count);
  if (!count) return "";
  switch (result.kind) {
    case "player":
    case "scenario":
      return `${count.toLocaleString()} runs`;
    case "kovaaks_scenario":
    case "benchmark":
      return `${count.toLocaleString()} players`;
    default:
      return "";
  }
}

/** Flattens grouped results into keyboard-navigable rows. */
export function buildItems(results: readonly QuickSearchResult[]): DropdownItem[] {
  return groupQuickResults(results).flatMap((group) =>
    group.items.slice(0, group.kind.startsWith("kovaaks") ? 4 : 5).map((result) => ({
      kind: result.kind,
      group: group.label,
      title: result.title,
      subtitle: result.subtitle,
      to: quickResultHref(result),
      meta: resultMeta(result),
      badge: result.badge,
      imageUrl: result.imageUrl,
      country: result.country,
    })),
  );
}

function kindLabel(kind: string) {
  return kind === "kovaaks_player" ? "KovaaK's player" : kind === "kovaaks_scenario" ? "KovaaK's scenario" : kind;
}

export const HeaderSearch = forwardRef<HTMLInputElement, { game?: "osu" | "kovaaks" }>(function HeaderSearch({ game = "kovaaks" }, ref) {
  const navigate = useNavigate();
  const [value, setValue] = useState("");
  const [results, setResults] = useState<QuickSearchResult[] | null>(null);
  const [recent, setRecent] = useState<RecentSearch[]>(() => (typeof window === "undefined" ? [] : loadRecentSearches(window.localStorage)));
  const [open, setOpen] = useState(false);
  const [focused, setFocused] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const [searchState, setSearchState] = useState<"idle" | "loading" | "error">("idle");
  const containerRef = useRef<HTMLDivElement>(null);
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);

  // Two steps: AimMod results almost at once, then KovaaK's players and
  // scenarios once typing pauses.
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
    setSearchState("loading");
    const local = setTimeout(() => {
      void quickSearch(q, false)
        .then((r) => {
          if (cancelled) return;
          setResults(r.results);
          setOpen(true);
          setSearchState("idle");
        })
        .catch(() => { if (!cancelled) setSearchState("error"); });
    }, 90);
    const external = q.length >= 3 ? setTimeout(() => {
      void quickSearch(q, true)
        .then((r) => {
          if (cancelled || !r.kovaaksIncluded) return;
          setResults(r.results);
          setOpen(true);
          setSearchState("idle");
        })
        .catch(() => {});
    }, 450) : undefined;
    return () => {
      cancelled = true;
      clearTimeout(local);
      if (external) clearTimeout(external);
    };
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

  useEffect(() => {
    itemRefs.current[activeIndex]?.scrollIntoView({ block: "nearest" });
  }, [activeIndex]);

  const items = results ? buildItems(results) : [];
  const showQuickNav = focused && !value.trim() && !open;

  function go(to: string, item?: { title: string; kind: string }) {
    if (item && typeof window !== "undefined") setRecent(rememberSearch(window.localStorage, { title: item.title, href: to, kind: item.kind }));
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
      (e.target as HTMLInputElement).blur();
    } else if (e.key === "Enter") {
      if (open && items[activeIndex]) {
        e.preventDefault();
        go(items[activeIndex].to, items[activeIndex]);
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

  const menu = "absolute left-0 right-0 top-[calc(100%+6px)] z-50 overflow-y-auto overscroll-contain rounded-md border border-line bg-panel-strong shadow-[0_16px_48px_rgba(0,0,0,0.6)]";

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
          placeholder="Search players, scenarios, benchmarks"
          className="min-w-0 flex-1 rounded-md border border-line bg-panel px-3 py-2.5 text-sm text-text transition-colors placeholder:text-muted focus:border-cyan"
        />
        <button type="submit" className="inline-flex min-h-10 shrink-0 items-center justify-center rounded-md border border-line bg-panel px-3 text-sm text-text hover:border-line-strong">
          Search
        </button>
      </form>

      {showQuickNav && (
        <div className={cn(menu, "max-h-[60vh]")}>
          {recent.length ? (
            <>
              <div className="border-b border-line/50 px-4 py-2"><span className="text-[10px] uppercase tracking-normal text-muted">Recent</span></div>
              {recent.map((item) => (
                <button key={item.href} type="button" onClick={(e) => { e.preventDefault(); go(item.href, item); }}
                  className="flex w-full items-center justify-between gap-3 border-b border-line/40 px-4 py-2.5 text-left text-[13px] transition-colors hover:bg-[rgba(121,201,151,0.08)]">
                  <span className="truncate">{item.title}</span>
                  <span className="shrink-0 text-[11px] capitalize text-muted-2">{kindLabel(item.kind)}</span>
                </button>
              ))}
            </>
          ) : null}
          <div className="border-b border-line/50 px-4 py-2"><span className="text-[10px] uppercase tracking-normal text-muted">Quick navigation</span></div>
          {quickNavItems.map((item) => (
            <button key={item.to} type="button" onClick={(e) => { e.preventDefault(); go(item.to); }}
              className="flex w-full items-center justify-between gap-3 border-b border-line/40 px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-[rgba(121,201,151,0.08)]">
              <div className="min-w-0">
                <div className="text-[13px] text-text">{item.label}</div>
                <div className="mt-0.5 text-[11px] text-muted">{item.sub}</div>
              </div>
              <span className="shrink-0 text-[11px] text-muted-2">→</span>
            </button>
          ))}
          <div className="border-t border-line/50 px-4 py-2 text-[11px] text-muted-2">
            Players, scenarios and benchmarks, including KovaaK's players not on AimMod · Ctrl+K or / to focus
          </div>
        </div>
      )}

      {focused && value.trim() && (searchState !== "idle" || (results && items.length === 0)) && !items.length && (
        <div role="status" className="absolute left-0 right-0 top-full z-50 mt-2 rounded-md border border-line bg-panel-strong p-4 text-sm text-muted">
          {searchState === "loading" ? "Searching..." : searchState === "error" ? "Search unavailable. Try again shortly." : "No matches found."}
        </div>
      )}
      {focused && open && items.length > 0 && (
        <div id="hub-search-results" role="listbox" aria-label="Search results" className={cn(menu, "max-h-[min(480px,65vh)]")}>
          {items.map((item, i) => (
            <div key={item.to} role="presentation">
              {i === 0 || items[i - 1].group !== item.group ? (
                <div role="presentation" className="sticky top-0 z-10 border-b border-line/50 bg-panel-strong px-4 py-1.5 text-[10px] uppercase tracking-normal text-muted">{item.group}</div>
              ) : null}
              <button
                ref={(el) => { itemRefs.current[i] = el; }}
                type="button"
                role="option"
                id={`hub-result-${i}`}
                aria-selected={i === activeIndex}
                onClick={(e) => { e.preventDefault(); go(item.to, item); }}
                onMouseEnter={() => setActiveIndex(i)}
                className={cn(
                  "flex w-full items-center justify-between gap-3 border-b border-line/40 px-4 py-2.5 text-left transition-colors",
                  i === activeIndex ? "bg-[rgba(121,201,151,0.1)]" : "hover:bg-[rgba(255,255,255,0.03)]",
                )}
              >
                <div className="flex min-w-0 items-center gap-2.5">
                  {item.imageUrl ? <img src={item.imageUrl} alt="" className="h-7 w-7 shrink-0 rounded border border-line object-cover" loading="lazy" /> : (
                    <span aria-hidden className="grid h-7 w-7 shrink-0 place-items-center rounded border border-line text-[11px] text-muted">{item.title.slice(0, 1).toUpperCase()}</span>
                  )}
                  <div className="min-w-0">
                    <div className="truncate text-[13px] text-text">{item.title}</div>
                    {item.subtitle ? <div className="truncate text-[11px] text-muted-2">{item.subtitle}</div> : null}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <CountryTag code={item.country} />
                  {item.badge ? <ScenarioTypeBadge type={item.badge} /> : null}
                  {item.meta ? <span className="text-[11px] text-muted max-sm:hidden">{item.meta}</span> : null}
                </div>
              </button>
            </div>
          ))}
          <button
            type="button"
            onClick={(e) => { e.preventDefault(); navigate(`/search?q=${encodeURIComponent(value.trim())}`); setOpen(false); }}
            className="flex w-full items-center justify-center gap-2 px-4 py-2.5 text-[12px] text-muted transition-colors hover:bg-[rgba(255,255,255,0.03)] hover:text-text"
          >
            See all results for "{value}"
          </button>
        </div>
      )}
    </div>
  );
});
