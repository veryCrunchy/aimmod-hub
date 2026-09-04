import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Helmet } from "../lib/helmet";
import { useSearchParams } from "react-router-dom";
import { ReplayResultCard } from "../components/ReplayResultCard";
import { SectionHeader } from "../components/SectionHeader";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Skeleton } from "../components/ui/Skeleton";
import { Button } from "../components/ui/Button";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchReplayHub } from "../lib/api";

type ReplayFilter = "all" | "video" | "mouse";

const FILTERS: ReplayFilter[] = ["all", "video", "mouse"];

export function ReplayHubPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const query = searchParams.get("q") ?? "";
  const [draftQuery, setDraftQuery] = useState(query);
  const [filter, setFilter] = useState<ReplayFilter>("all");
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

  return (
    <PageStack>
      <Helmet>
        <title>Replay Hub · AimMod Hub</title>
        <meta
          name="description"
          content="Browse uploaded AimMod replays, find watchable runs, and jump straight into replay-rich scenarios and player pages."
        />
      </Helmet>

      <PageSection>
        <SectionHeader
          eyebrow="Replay hub"
          level={1}
          title="Replay library"
          body="Recorded KovaaK's runs and mouse paths."
        />

        <form onSubmit={handleSubmit} className="mt-4 flex flex-col gap-3 md:flex-row md:items-center">
          <input
            aria-label="Search KovaaK's replays"
            type="search"
            value={draftQuery}
            onChange={(event) => setDraftQuery(event.target.value)}
            placeholder="Search replays by scenario, player, or run id"
            className="min-w-0 flex-1 rounded-full border border-line bg-[rgba(255,255,255,0.03)] px-4 py-2.5 text-sm text-text outline-none transition-colors placeholder:text-muted focus:border-mint/60"
          />
          <button
            type="submit"
            className="inline-flex min-h-10 shrink-0 items-center justify-center rounded-full bg-mint px-4 text-sm font-medium text-[#041009] transition-transform hover:scale-[1.02]"
          >
            Search
          </button>
        </form>

        <div className="mt-4 flex flex-wrap items-center gap-2">
          {FILTERS.map((item) => (
            <button
              key={item}
              type="button"
              aria-pressed={filter === item}
              onClick={() => setFilter(item)}
              className={[
                "rounded-full border px-3 py-1.5 text-[12px] transition-colors",
                filter === item
                  ? "border-mint/40 bg-mint/10 text-mint"
                  : "border-line text-muted hover:border-line-strong hover:text-text",
              ].join(" ")}
            >
              {item === "all" ? "All replays" : item === "video" ? "Video only" : "Mouse path only"}
            </button>
          ))}
        </div>
      </PageSection>

      <Grid className="grid-cols-[repeat(auto-fit,minmax(180px,1fr))]">
        <div className="rounded-md border border-line bg-white/2 p-4">
          <p className="text-[10px] uppercase tracking-normal text-muted-2">Replay runs</p>
          <p className="mt-2 text-3xl font-medium text-text">{items.length.toLocaleString()}</p>
          <p className="mt-1 text-[12px] text-muted">Runs with replay data in the current query window.</p>
        </div>
        <div className="rounded-md border border-line bg-white/2 p-4">
          <p className="text-[10px] uppercase tracking-normal text-muted-2">Video clips</p>
          <p className="mt-2 text-3xl font-medium text-mint">{items.filter((item) => item.hasVideo).length.toLocaleString()}</p>
          <p className="mt-1 text-[12px] text-muted">Runs that already have watchable replay video on the hub.</p>
        </div>
        <div className="rounded-md border border-line bg-white/2 p-4">
          <p className="text-[10px] uppercase tracking-normal text-muted-2">Mouse path captures</p>
          <p className="mt-2 text-3xl font-medium text-violet">{items.filter((item) => item.hasMousePath).length.toLocaleString()}</p>
          <p className="mt-1 text-[12px] text-muted">Runs with uploaded mouse-path replay data.</p>
        </div>
      </Grid>

      <PageSection>
        <SectionHeader
          eyebrow="Replay results"
          title={query ? `Results for “${query}”` : "Latest replay-ready runs"}
          body={
            query
              ? "These are the replay-ready runs that match your search."
              : "The newest runs that already have replay data available on the hub."
          }
        />

        {loading ? (
          <div role="status" aria-label="Loading replays" className="grid gap-3">
            <span className="text-sm text-muted">Loading replays...</span>
            {[0, 1, 2, 3, 4].map((index) => (
              <Skeleton key={index} className="h-[124px] rounded-md" />
            ))}
          </div>
        ) : error ? (
          <EmptyState title="Replay library unavailable" body="Please try again in a moment."><Button onClick={() => setAttempt(value => value + 1)}>Try again</Button></EmptyState>
        ) : filteredItems.length === 0 ? (
          <EmptyState
            title="No replay matches yet"
            body="Try a broader search, or wait for more replay-enabled runs to upload."
          />
        ) : (
          <ScrollArea className="max-h-[min(72vh,980px)] pr-2">
            <div className="grid gap-3">
              {filteredItems.map((item) => (
                <ReplayResultCard key={`${item.publicRunID || item.sessionID}:${item.replayQuality}:${item.hasMousePath}`} run={item} />
              ))}
            </div>
          </ScrollArea>
        )}
      </PageSection>
    </PageStack>
  );
}
