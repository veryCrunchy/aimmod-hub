import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Helmet } from "../lib/helmet";
import { useSearchParams } from "react-router-dom";
import { filterChoice, updateFilterQuery } from "../lib/savedPageFilters";
import { ReplayResultCard } from "../components/ReplayResultCard";
import { PageHeader } from "../components/ui/PageHeader";
import { EmptyState } from "../components/ui/EmptyState";
import { Skeleton } from "../components/ui/Skeleton";
import { Button } from "../components/ui/Button";
import { PageStack } from "../components/ui/Stack";
import { fetchReplayHub } from "../lib/api";

type ReplayFilter = "all" | "video" | "mouse";

const FILTERS: ReplayFilter[] = ["all", "video", "mouse"];

export function ReplayHubPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const query = searchParams.get("q") ?? "";
  const [draftQuery, setDraftQuery] = useState(query);
  const filter = filterChoice<ReplayFilter>(searchParams.get("replay"), FILTERS, "all");
  const setFilter = (replay: ReplayFilter) => setSearchParams(current => updateFilterQuery(current, { replay }), { replace: true });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [items, setItems] = useState<import("../lib/api").HubSearchRun[]>([]);

  useEffect(() => {
    setDraftQuery(query);
  }, [query]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    void fetchReplayHub({ query, limit: 80 })
      .then((response) => {
        if (!cancelled) {
          setItems(response.items);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Could not load replays.");
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [query, attempt]);

  const filteredItems = useMemo(() => {
    return items.filter((item) => {
      if (filter === "video") return item.hasVideo;
      if (filter === "mouse") return item.hasMousePath;
      return true;
    });
  }, [items, filter]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = new URLSearchParams(searchParams);
    const trimmed = draftQuery.trim();
    if (trimmed) next.set("q", trimmed);
    else next.delete("q");
    setSearchParams(next);
  }

  const videoCount = items.filter((item) => item.hasVideo).length;
  const mouseCount = items.filter((item) => item.hasMousePath).length;
  const counts: Record<ReplayFilter, number> = { all: items.length, video: videoCount, mouse: mouseCount };
  const labels: Record<ReplayFilter, string> = { all: "All", video: "Video", mouse: "Mouse path" };

  return (
    <PageStack>
      <Helmet>
        <title>Replays · AimMod Hub</title>
        <meta name="description" content="KovaaK's runs with replay video or mouse paths, shared through AimMod." />
      </Helmet>

      <PageHeader
        title="Replays"
        meta={loading ? "Loading…" : query ? `${filteredItems.length.toLocaleString()} matching “${query}”` : `${items.length.toLocaleString()} recent runs with video or a mouse path`}
      />

      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <form onSubmit={handleSubmit} role="search" className="flex min-w-0 flex-1 gap-2">
          <input
            aria-label="Search replays"
            type="search"
            value={draftQuery}
            onChange={(event) => setDraftQuery(event.target.value)}
            placeholder="Scenario, player or run id"
            className="min-h-10 min-w-0 flex-1 rounded-md border border-line bg-panel px-3 text-sm text-text placeholder:text-muted-2"
          />
          <Button type="submit">Search</Button>
        </form>
        <div className="flex flex-wrap gap-2" role="group" aria-label="Replay type">
          {FILTERS.map((item) => (
            <button
              key={item}
              type="button"
              aria-pressed={filter === item}
              onClick={() => setFilter(item)}
              className={[
                "min-h-8 rounded-full border px-3 text-xs transition-colors",
                filter === item ? "border-cyan/40 bg-cyan/10 text-cyan" : "border-line text-muted hover:text-text",
              ].join(" ")}
            >
              {labels[item]} <span className="tabular-nums opacity-70">{counts[item]}</span>
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div role="status" aria-label="Loading replays" className="grid gap-3 lg:grid-cols-2">
          {[0, 1, 2, 3].map((index) => <Skeleton key={index} className="h-[110px]" />)}
        </div>
      ) : error ? (
        <EmptyState title="Replays could not be loaded."><Button onClick={() => setAttempt(value => value + 1)}>Try again</Button></EmptyState>
      ) : filteredItems.length === 0 ? (
        query || filter !== "all"
          ? <EmptyState title="No replays match."><Button onClick={() => setSearchParams(new URLSearchParams())}>Clear search</Button></EmptyState>
          : <EmptyState title="No replays shared yet."><Button to="/kovaaks">See recent runs</Button></EmptyState>
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {filteredItems.map((item) => (
            <ReplayResultCard key={`${item.publicRunID || item.sessionID}:${item.replayQuality}:${item.hasMousePath}`} run={item} />
          ))}
        </div>
      )}
    </PageStack>
  );
}
