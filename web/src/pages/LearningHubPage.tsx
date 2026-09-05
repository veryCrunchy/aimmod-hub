import { useEffect, useMemo, useState } from "react";
import { PageSeo } from "../components/PageSeo";
import { Link, useSearchParams } from "react-router-dom";
import { updateFilterQuery } from "../lib/savedPageFilters";
import { SectionHeader } from "../components/SectionHeader";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchLearningIndex, type LearnEntryPreview } from "../lib/api";
import { getPrerenderLearningIndex } from "../lib/prerender";

function humanizeToken(value: string) {
  return value
    .replace(/[_-]+/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

const PRIORITY_STYLES = {
  high: {
    badge: "border-mint/30 bg-mint/[0.06] text-mint",
    label: "High signal",
    bar: "bg-mint/70",
  },
  medium: {
    badge: "border-cyan/30 bg-cyan/[0.04] text-cyan",
    label: "Useful",
    bar: "bg-cyan/50",
  },
  low: {
    badge: "border-line bg-white/[0.02] text-muted",
    label: "Reference",
    bar: "bg-line-strong/60",
  },
} as const;

function priorityStyle(priority: string) {
  return PRIORITY_STYLES[priority as keyof typeof PRIORITY_STYLES] ?? PRIORITY_STYLES.low;
}

function EntryCard({ entry, featured = false }: { entry: LearnEntryPreview; featured?: boolean }) {
  const ps = priorityStyle(entry.priority);
  const tags = [
    ...entry.contextTags.slice(0, featured ? 3 : 2),
    ...entry.scenarioTypes.slice(0, featured ? 2 : 1),
  ];

  return (
    <Link
      to={`/learn/${entry.id}`}
      className={`group relative rounded-[18px] border border-line bg-white/[0.02] overflow-hidden transition-all hover:border-cyan/30 hover:bg-white/[0.035] hover:-translate-y-px ${featured ? "p-5" : "p-4"}`}
    >
      <div className={`absolute left-0 top-0 bottom-0 w-[3px] ${ps.bar}`} />

      <div className="pl-3">
        <div className="flex flex-wrap items-center gap-2">
          <span className={`rounded-full border px-2 py-0.5 text-[11px] uppercase tracking-[0.08em] ${ps.badge}`}>
            {ps.label}
          </span>
          {(entry.sourceCount > 0 || entry.drillCount > 0) && (
            <span className="text-[11px] text-muted-2">
              {entry.sourceCount > 0 ? `${entry.sourceCount} src` : ""}
              {entry.sourceCount > 0 && entry.drillCount > 0 ? " · " : ""}
              {entry.drillCount > 0 ? `${entry.drillCount} drill${entry.drillCount !== 1 ? "s" : ""}` : ""}
            </span>
          )}
        </div>
        <h3
          className={`mt-2.5 font-medium leading-tight tracking-[-0.035em] text-text transition-colors group-hover:text-cyan ${featured ? "text-[19px]" : "text-[17px]"}`}
        >
          {entry.title}
        </h3>
        <p className={`mt-2 leading-6 text-muted line-clamp-3 ${featured ? "text-[14px]" : "text-[13px]"}`}>
          {entry.summary}
        </p>
        {tags.length ? (
          <div className="mt-3.5 flex flex-wrap gap-1.5">
            {tags.map((tag) => (
              <Link
                key={tag}
                to={`/learn/topics/${tag}`}
                className="rounded-full border border-line bg-white/[0.02] px-2 py-0.5 text-[11px] text-muted-2 transition-colors hover:border-cyan/30 hover:text-cyan"
              >
                {humanizeToken(tag)}
              </Link>
            ))}
          </div>
        ) : null}
      </div>
    </Link>
  );
}

function HubPageSkeleton() {
  return (
    <PageStack>
      <PageSection className="overflow-hidden border-cyan/18">
        <Skeleton className="h-3.5 w-20 mb-4" />
        <Skeleton className="h-12 w-2/3 mb-3" />
        <Skeleton className="h-5 w-full max-w-lg mb-2" />
        <Skeleton className="h-5 w-4/5 max-w-md mb-6" />
        <div className="flex gap-2">
          <Skeleton className="h-9 w-36 rounded-full" />
          <Skeleton className="h-9 w-32 rounded-full" />
        </div>
      </PageSection>
      <Grid className="grid-cols-[repeat(auto-fit,minmax(150px,1fr))]">
        {[0, 1, 2, 3].map((i) => (
          <PageSection key={i}>
            <Skeleton className="h-3 w-14 mb-3" />
            <Skeleton className="h-8 w-10 mb-2" />
            <Skeleton className="h-4 w-full" />
          </PageSection>
        ))}
      </Grid>
      <Grid className="grid-cols-[1.45fr_0.85fr] max-[1080px]:grid-cols-1">
        <PageSection>
          <Skeleton className="h-3.5 w-28 mb-3" />
          <Skeleton className="h-7 w-3/4 mb-2" />
          <Skeleton className="h-4 w-full mb-6" />
          <div className="grid gap-3 md:grid-cols-2">
            {[0, 1, 2, 3, 4, 5, 6, 7].map((i) => (
              <Skeleton key={i} className="h-40" />
            ))}
          </div>
        </PageSection>
        <PageSection>
          <Skeleton className="h-3.5 w-20 mb-3" />
          <Skeleton className="h-7 w-4/5 mb-2" />
          <Skeleton className="h-4 w-full mb-5" />
          <div className="flex flex-wrap gap-2 mb-4">
            {[0, 1, 2, 3, 4].map((i) => (
              <Skeleton key={i} className="h-7 w-20 rounded-full" />
            ))}
          </div>
          <div className="flex flex-wrap gap-2">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-7 w-24 rounded-full" />
            ))}
          </div>
        </PageSection>
      </Grid>
    </PageStack>
  );
}

export function LearningHubPage() {
  const [data, setData] = useState<Awaited<ReturnType<typeof fetchLearningIndex>> | null>(() => getPrerenderLearningIndex());
  const [error, setError] = useState<string | null>(null);
  const [params, setParams] = useSearchParams();
  const search = params.get("q") ?? "";
  const setSearch = (q: string) => setParams(current => updateFilterQuery(current, { q }), { replace: true });

  useEffect(() => {
    if (data) return;
    let cancelled = false;
    void fetchLearningIndex()
      .then((response) => {
        if (cancelled) return;
        setData(response);
        setError(null);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Could not load learning guides.");
      });
    return () => {
      cancelled = true;
    };
  }, [data]);

  const filteredEntries = useMemo(() => {
    if (!data) return [];
    const needle = search.trim().toLowerCase();
    if (!needle) return data.entries;
    return data.entries.filter((entry) => {
      const haystack = [
        entry.title,
        entry.summary,
        ...entry.contextTags,
        ...entry.signalKeys,
        ...entry.scenarioTypes,
        ...entry.focusAreas,
      ]
        .join(" ")
        .toLowerCase();
      return haystack.includes(needle);
    });
  }, [data, search]);

  if (!data && !error) return <HubPageSkeleton />;

  return (
    <PageStack>
      <PageSeo title="KovaaK's Aim Training Knowledge Base · AimMod Hub"
        description="Read aim training guides on mechanics, practice routines, common mistakes and scenario selection." noindex={Boolean(error)} />
      <Link className="text-sm text-cyan" to="/osu/learn">Looking for osu! guides? Visit the osu! knowledge base.</Link>

      <PageSection className="relative overflow-hidden border-cyan/18 bg-[radial-gradient(circle_at_top_left,rgba(94,233,255,0.12),transparent_24%),radial-gradient(circle_at_82%_12%,rgba(121,201,151,0.12),transparent_22%),linear-gradient(135deg,rgba(8,18,20,0.98),rgba(7,13,16,0.96)_52%,rgba(5,8,11,0.98))] shadow-[0_24px_80px_rgba(0,0,0,0.42)]">
        <div className="text-[11px] uppercase tracking-[0.1em] text-cyan/80">AimMod Learn</div>
        <h1 className="my-2.5 max-w-[16ch] break-words text-[clamp(28px,5vw,56px)] leading-[0.94] tracking-[-0.05em]">
          Evidence-backed aim training guides.
        </h1>
        <p className="max-w-[760px] text-[14px] leading-6 text-[#cbe4d7] md:text-[16px] md:leading-7">
          Research-backed guides covering aim improvement, flaws, mechanics, scenario training, and sensitivity — the same advice AimMod uses when coaching you directly.
        </p>
        <div className="mt-5 flex flex-wrap gap-2">
          <Button
            to={data?.featuredEntries[0] ? `/learn/${data.featuredEntries[0].id}` : "/learn"}
            variant="primary"
          >
            Open a featured guide
          </Button>
          <Button to="/app">Get the desktop coach</Button>
        </div>
      </PageSection>

      <Grid className="grid-cols-[repeat(auto-fit,minmax(150px,1fr))]">
        <PageSection className="border-mint/18 bg-[radial-gradient(circle_at_bottom_right,rgba(121,201,151,0.07),transparent_60%)]">
          <div className="text-[10px] uppercase tracking-[0.1em] text-mint/60">Guides</div>
          <div className="mt-2 text-[clamp(24px,3vw,32px)] font-medium text-text tabular-nums">
            {data?.entryCount ?? "—"}
          </div>
          <div className="mt-1 text-[12px] text-muted">Evidence-backed aim training guides</div>
        </PageSection>
        <PageSection className="border-cyan/14 bg-[radial-gradient(circle_at_bottom_right,rgba(94,233,255,0.05),transparent_60%)]">
          <div className="text-[10px] uppercase tracking-[0.1em] text-cyan/60">Sources</div>
          <div className="mt-2 text-[clamp(24px,3vw,32px)] font-medium text-text tabular-nums">
            {data?.sourceCount ?? "—"}
          </div>
          <div className="mt-1 text-[12px] text-muted">Videos, articles, transcripts</div>
        </PageSection>
        <PageSection>
          <div className="text-[10px] uppercase tracking-[0.1em] text-muted-2">Signals</div>
          <div className="mt-2 text-[clamp(24px,3vw,32px)] font-medium text-text tabular-nums">
            {data?.signalKeyCount ?? "—"}
          </div>
          <div className="mt-1 text-[12px] text-muted">Aim patterns covered</div>
        </PageSection>
        <PageSection>
          <div className="text-[10px] uppercase tracking-[0.1em] text-muted-2">Topics</div>
          <div className="mt-2 text-[clamp(24px,3vw,32px)] font-medium text-text tabular-nums">
            {data?.contextTagCount ?? "—"}
          </div>
          <div className="mt-1 text-[12px] text-muted">Searchable learning tags</div>
        </PageSection>
      </Grid>

      <Grid className="grid-cols-[1.45fr_0.85fr] max-[1080px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Featured guides"
            title="High-signal learning pages"
            body="Comprehensive guides with clear action steps, cited research, and related topics to explore."
          />
          {data?.featuredEntries.length ? (
            <div className="grid gap-3 md:grid-cols-2">
              {data.featuredEntries.map((entry) => (
                <EntryCard key={entry.id} entry={entry} featured />
              ))}
            </div>
          ) : (
            <EmptyState
              title="No learning guides yet"
              body={error || "No guides available right now. Check back soon."}
            />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Topic map"
            title="Browse by topic"
            body="Click a tag to filter the guide list below."
          />
          <div className="grid gap-4">
            <div>
              <div className="mb-2 text-[11px] uppercase tracking-[0.1em] text-muted-2">Top contexts</div>
              <div className="flex flex-wrap gap-2">
                {data?.topContextTags.map((tag) => (
                  <Link
                    key={tag}
                    to={`/learn/topics/${tag}`}
                    className="rounded-full border border-line bg-white/[0.03] px-2.5 py-1 text-[12px] text-text transition-colors hover:border-line-strong hover:bg-white/[0.05] hover:text-cyan"
                  >
                    {humanizeToken(tag)}
                  </Link>
                ))}
              </div>
            </div>
            <div>
              <div className="mb-2 text-[11px] uppercase tracking-[0.1em] text-muted-2">Scenario families</div>
              <div className="flex flex-wrap gap-2">
                {data?.topScenarioTypes.map((tag) => (
                  <Link
                    key={tag}
                    to={`/learn/topics/${tag}`}
                    className="rounded-full border border-cyan/20 bg-cyan/[0.04] px-2.5 py-1 text-[12px] text-cyan transition-colors hover:border-cyan/35 hover:bg-cyan/[0.08]"
                  >
                    {humanizeToken(tag)}
                  </Link>
                ))}
              </div>
            </div>
            <div className="rounded-[14px] border border-line bg-white/[0.02] p-4 text-[13px] leading-6 text-muted">
              Every guide is backed by the same research AimMod uses to coach you. New guides appear as the research grows.
            </div>
          </div>
        </PageSection>
      </Grid>

      <PageSection>
        <SectionHeader
          eyebrow="Browse guides"
          title="All guides"
          aside={
            data?.updatedAtIso ? (
              <span className="text-muted-2">Updated {data.updatedAtIso.slice(0, 10)}</span>
            ) : undefined
          }
        />
        <div className="relative mb-4">
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter by title, topic, signal, or scenario family…"
            className="w-full rounded-[14px] border border-line bg-white/[0.03] px-4 py-3 pr-28 text-[14px] text-text outline-none transition-colors placeholder:text-muted-2 focus:border-cyan/35"
          />
          <div className="absolute right-3 top-1/2 flex -translate-y-1/2 items-center gap-1.5">
            {search ? (
              <button
                onClick={() => setSearch("")}
                className="rounded-full px-2 py-0.5 text-[12px] text-muted-2 transition-colors hover:text-text"
              >
                clear
              </button>
            ) : null}
            <span className="rounded-full border border-line bg-white/[0.03] px-2 py-0.5 text-[12px] text-muted-2 tabular-nums">
              {filteredEntries.length}
            </span>
          </div>
        </div>
        {filteredEntries.length ? (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {filteredEntries.map((entry) => (
              <EntryCard key={entry.id} entry={entry} />
            ))}
          </div>
        ) : (
          <EmptyState
            title="No matching guides"
            body="Try a broader search term like sensitivity, transfer, plateau, or tracking."
          />
        )}
      </PageSection>
    </PageStack>
  );
}

export default LearningHubPage;
