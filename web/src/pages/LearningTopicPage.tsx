import { useEffect, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useParams } from "react-router-dom";
import { SectionHeader } from "../components/SectionHeader";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchLearningTopic, type LearnEntryPreview } from "../lib/api";
import { getPrerenderLearningTopic } from "../lib/prerender";

function humanizeToken(value: string) {
  return value
    .replace(/[_-]+/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function TopicEntryCard({ entry }: { entry: LearnEntryPreview }) {
  return (
    <Link
      to={`/learn/${entry.id}`}
      className="group rounded-[16px] border border-line bg-white/[0.02] p-4 transition-all hover:border-cyan/30 hover:bg-white/[0.04] hover:-translate-y-px"
    >
      <div className="text-[11px] uppercase tracking-[0.1em] text-muted-2">{humanizeToken(entry.priority || "reference")}</div>
      <h3 className="mt-2 text-[17px] font-medium tracking-[-0.03em] text-text transition-colors group-hover:text-cyan">
        {entry.title}
      </h3>
      <p className="mt-2 text-[13px] leading-6 text-muted">{entry.summary}</p>
      <div className="mt-4 flex flex-wrap gap-2">
        {entry.scenarioTypes.slice(0, 2).map((type) => (
          <span key={type} className="rounded-full border border-cyan/20 bg-cyan/[0.05] px-2.5 py-1 text-[11px] text-cyan">
            {humanizeToken(type)}
          </span>
        ))}
      </div>
    </Link>
  );
}

function TopicPageSkeleton() {
  return (
    <PageStack>
      <PageSection>
        <Skeleton className="h-3 w-24 mb-4" />
        <Skeleton className="h-11 w-2/3 mb-3" />
        <Skeleton className="h-5 w-full max-w-2xl mb-2" />
        <Skeleton className="h-5 w-4/5 max-w-xl" />
      </PageSection>
      <Grid className="grid-cols-[0.8fr_1.2fr] max-[1080px]:grid-cols-1">
        <PageSection>
          <Skeleton className="h-7 w-28 mb-4" />
          <div className="flex flex-wrap gap-2">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-7 w-24 rounded-full" />
            ))}
          </div>
        </PageSection>
        <PageSection>
          <Skeleton className="h-7 w-40 mb-4" />
          <div className="grid gap-3 md:grid-cols-2">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-40" />
            ))}
          </div>
        </PageSection>
      </Grid>
    </PageStack>
  );
}

export function LearningTopicPage() {
  const { topic = "" } = useParams();
  const [data, setData] = useState<Awaited<ReturnType<typeof fetchLearningTopic>> | null>(() =>
    topic ? getPrerenderLearningTopic(topic) : null,
  );
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const initial = topic ? getPrerenderLearningTopic(topic) : null;
    if (initial) {
      setData(initial);
      setError(null);
      return;
    }
    let cancelled = false;
    setData(null);
    setError(null);
    void fetchLearningTopic(topic)
      .then((response) => {
        if (cancelled) return;
        setData(response);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Could not load this topic page.");
      });
    return () => {
      cancelled = true;
    };
  }, [topic]);

  if (!data && !error) return <TopicPageSkeleton />;

  const metaTitle = data ? `${data.title} Aim Training Guides · AimMod Learn` : "Aim Training Topic · AimMod Hub";
  const metaDescription = data?.description || "KB-backed aim training topic page from AimMod Learn.";

  return (
    <PageStack>
      <Helmet>
        <title>{metaTitle}</title>
        <meta name="description" content={metaDescription} />
        <meta property="og:title" content={metaTitle} />
        <meta property="og:description" content={metaDescription} />
      </Helmet>

      {error ? (
        <PageSection>
          <SectionHeader eyebrow="Learn topic" title="Could not load this topic" body={error} />
          <Button to="/learn">Back to learning pages</Button>
        </PageSection>
      ) : null}

      {data ? (
        <>
          <PageSection className="relative overflow-hidden border-cyan/18 bg-[radial-gradient(circle_at_top_left,rgba(94,233,255,0.10),transparent_22%),radial-gradient(circle_at_78%_18%,rgba(121,201,151,0.10),transparent_18%),linear-gradient(135deg,rgba(8,18,20,0.98),rgba(6,12,16,0.96)_52%,rgba(5,8,11,0.98))] shadow-[0_24px_80px_rgba(0,0,0,0.42)]">
            <Breadcrumb crumbs={[{ label: "Learn", to: "/learn" }, { label: data.title }]} />
            <div className="mt-3 max-w-[920px]">
              <div className="text-[11px] uppercase tracking-[0.1em] text-cyan">AimMod Learn Topic</div>
              <h1 className="mt-2 text-[clamp(28px,4.8vw,54px)] leading-[0.95] tracking-[-0.05em] text-text">
                {data.title}
              </h1>
              <p className="mt-3 max-w-[760px] text-[15px] leading-7 text-[#cbe4d7]">{data.description}</p>
            </div>
          </PageSection>

          <Grid className="grid-cols-[0.8fr_1.2fr] max-[1080px]:grid-cols-1">
            <PageSection>
              <SectionHeader eyebrow="Topic graph" title={`${data.entryCount} related guides`} />
              {data.relatedTopics.length ? (
                <div className="flex flex-wrap gap-2">
                  {data.relatedTopics.map((related) => (
                    <Link
                      key={related}
                      to={`/learn/topics/${related}`}
                      className="rounded-full border border-line bg-white/[0.03] px-3 py-1.5 text-[12px] text-text transition-colors hover:border-cyan/30 hover:text-cyan"
                    >
                      {humanizeToken(related)}
                    </Link>
                  ))}
                </div>
              ) : (
                <EmptyState title="No related topics yet" body="This topic is currently isolated in the KB graph." />
              )}
            </PageSection>

            <PageSection>
              <SectionHeader eyebrow="Featured guides" title={`Best pages for ${data.title.toLowerCase()}`} />
              <div className="grid gap-3 md:grid-cols-2">
                {data.featuredEntries.map((entry) => (
                  <TopicEntryCard key={entry.id} entry={entry} />
                ))}
              </div>
            </PageSection>
          </Grid>

          <PageSection>
            <SectionHeader eyebrow="All guides" title={`Everything currently filed under ${data.title.toLowerCase()}`} />
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {data.entries.map((entry) => (
                <TopicEntryCard key={entry.id} entry={entry} />
              ))}
            </div>
          </PageSection>
        </>
      ) : null}
    </PageStack>
  );
}

export default LearningTopicPage;
