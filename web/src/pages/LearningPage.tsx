import { useEffect, useMemo, useState } from "react";
import { PageSeo } from "../components/PageSeo";
import { Link, useParams } from "react-router-dom";
import { SectionHeader } from "../components/SectionHeader";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { Button } from "../components/ui/Button";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchLearningEntry, type LearnEntryPreview, type LearnMechanic, type LearnScenario } from "../lib/api";
import { getPrerenderLearningEntry } from "../lib/prerender";

function humanizeToken(value: string) {
  return value
    .replace(/[_-]+/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function priorityTone(priority: string) {
  switch (priority) {
    case "high":
      return "border-mint/30 bg-mint/[0.06] text-mint";
    case "medium":
      return "border-cyan/30 bg-cyan/[0.05] text-cyan";
    default:
      return "border-line bg-white/[0.03] text-muted";
  }
}

function BadgeRow({ title, values, tone = "neutral" }: { title: string; values: string[]; tone?: "neutral" | "cyan" }) {
  if (!values.length) return null;
  return (
    <div>
      <div className="mb-2 text-[11px] uppercase tracking-[0.1em] text-muted-2">{title}</div>
      <div className="flex flex-wrap gap-2">
        {values.map((value) => (
          <Link
            key={value}
            to={`/learn/topics/${value}`}
            className={
              tone === "cyan"
                ? "rounded-full border border-cyan/20 bg-cyan/[0.05] px-2.5 py-1 text-[12px] text-cyan transition-colors hover:border-cyan/40 hover:bg-cyan/[0.08]"
                : "rounded-full border border-line bg-white/[0.03] px-2.5 py-1 text-[12px] text-text transition-colors hover:border-cyan/30 hover:text-cyan"
            }
          >
            {humanizeToken(value)}
          </Link>
        ))}
      </div>
    </div>
  );
}

function WhyList({ items }: { items: string[] }) {
  if (!items.length) return null;
  return (
    <PageSection>
      <SectionHeader eyebrow="Why" title="Why this matters" />
      <div className="grid gap-2">
        {items.map((item, i) => (
          <div
            key={`why-${i}`}
            className="flex gap-3 rounded-[14px] border border-cyan/14 bg-cyan/[0.025] p-4"
          >
            <div className="mt-[3px] shrink-0 text-[14px] leading-none text-cyan/50 select-none">◆</div>
            <p className="text-[14px] leading-[1.7] text-text">{item}</p>
          </div>
        ))}
      </div>
    </PageSection>
  );
}

function ActionsList({ items }: { items: string[] }) {
  if (!items.length) return null;
  return (
    <PageSection>
      <SectionHeader eyebrow="Actions" title="What to do" />
      <div className="grid gap-2">
        {items.map((item, i) => (
          <div
            key={`action-${i}`}
            className="flex gap-4 rounded-[14px] border border-mint/14 bg-mint/[0.025] p-4"
          >
            <div className="w-6 shrink-0 text-right text-[clamp(18px,2vw,22px)] font-medium leading-none text-mint/40 tabular-nums mt-0.5">
              {i + 1}
            </div>
            <p className="text-[14px] leading-[1.7] text-text">{item}</p>
          </div>
        ))}
      </div>
    </PageSection>
  );
}

function AvoidList({ items }: { items: string[] }) {
  if (!items.length) return null;
  return (
    <PageSection>
      <SectionHeader eyebrow="Avoid" title="Common traps" />
      <div className="grid gap-2">
        {items.map((item, i) => (
          <div
            key={`avoid-${i}`}
            className="flex gap-3 rounded-[14px] border border-gold/14 bg-gold/[0.025] p-4"
          >
            <div className="mt-[3px] shrink-0 text-[14px] leading-none text-gold/60 select-none font-bold">!</div>
            <p className="text-[14px] leading-[1.7] text-[#f5edd4]">{item}</p>
          </div>
        ))}
      </div>
    </PageSection>
  );
}

function MechanicCard({ mechanic }: { mechanic: LearnMechanic }) {
  return (
    <div className="rounded-[16px] border border-cyan/14 bg-cyan/[0.025] p-4">
      <div className="text-[11px] uppercase tracking-[0.1em] text-cyan">Mechanic</div>
      <h3 className="mt-2 text-[17px] font-medium tracking-[-0.03em] text-text">{mechanic.title}</h3>
      <p className="mt-2 text-[13px] leading-6 text-muted">{mechanic.summary}</p>
      {mechanic.cues.length ? (
        <div className="mt-4">
          <div className="mb-2 text-[11px] uppercase tracking-[0.1em] text-muted-2">Cues</div>
          <div className="grid gap-2">
            {mechanic.cues.map((cue) => (
              <div key={cue} className="rounded-[10px] border border-line bg-white/[0.02] px-3 py-2 text-[13px] text-text">
                {cue}
              </div>
            ))}
          </div>
        </div>
      ) : null}
      <div className="mt-3">
        <BadgeRow title="Related signals" values={mechanic.relatedSignalKeys} />
      </div>
    </div>
  );
}

function ScenarioCard({ scenario }: { scenario: LearnScenario }) {
  return (
    <div className="rounded-[16px] border border-gold/14 bg-gold/[0.02] p-4">
      <div className="text-[11px] uppercase tracking-[0.1em] text-gold">Scenario</div>
      <h3 className="mt-2 text-[17px] font-medium tracking-[-0.03em] text-text">{scenario.name}</h3>
      <p className="mt-2 text-[13px] leading-6 text-muted">{scenario.summary}</p>
      <div className="mt-3 grid gap-3">
        <BadgeRow title="Scenario families" values={scenario.scenarioTypes} tone="cyan" />
        <BadgeRow title="What it trains" values={scenario.whatItTrains} />
      </div>
      {scenario.cautions.length ? (
        <div className="mt-4 rounded-[10px] border border-gold/20 bg-gold/[0.04] px-3 py-2 text-[13px] leading-6 text-[#e6d2a1]">
          {scenario.cautions[0]}
        </div>
      ) : null}
    </div>
  );
}

function RelatedCard({ entry }: { entry: LearnEntryPreview }) {
  return (
    <Link
      to={`/learn/${entry.id}`}
      className="group relative overflow-hidden rounded-[16px] border border-line bg-white/[0.02] p-4 transition-all hover:border-cyan/30 hover:bg-white/[0.04] hover:-translate-y-px"
    >
      <div className="text-[11px] uppercase tracking-[0.1em] text-muted-2">
        {humanizeToken(entry.priority || "reference")}
      </div>
      <h3 className="mt-2 text-[16px] font-medium tracking-[-0.03em] text-text transition-colors group-hover:text-cyan">
        {entry.title}
      </h3>
      <p className="mt-1.5 text-[13px] leading-6 text-muted line-clamp-2">{entry.summary}</p>
      <div className="mt-3 text-[12px] text-cyan/70 transition-colors group-hover:text-cyan">Open guide —&gt;</div>
    </Link>
  );
}

function SourceKindLabel({ kind }: { kind: string }) {
  const lower = kind.toLowerCase();
  if (lower === "video") return <span className="text-[#ff9f7a]">Video</span>;
  if (lower === "article") return <span className="text-violet">Article</span>;
  return <span className="text-muted-2">{humanizeToken(kind)}</span>;
}

function confidenceLabel(confidence: string): string {
  switch (confidence.toLowerCase()) {
    case "high":
      return "High confidence";
    case "medium":
      return "Medium confidence";
    case "low":
      return "Low confidence";
    default:
      return humanizeToken(confidence);
  }
}

function PageSkeleton() {
  return (
    <PageStack>
      <PageSection className="overflow-hidden border-cyan/18">
        <Skeleton className="h-3.5 w-12 mb-4" />
        <Skeleton className="h-3 w-20 mb-3" />
        <Skeleton className="h-12 w-4/5 mb-3" />
        <Skeleton className="h-5 w-full max-w-2xl mb-2" />
        <Skeleton className="h-5 w-3/4 max-w-xl mb-5" />
        <div className="flex gap-2">
          <Skeleton className="h-7 w-24 rounded-full" />
          <Skeleton className="h-7 w-36 rounded-full" />
        </div>
      </PageSection>
      <Grid className="grid-cols-[0.75fr_1.25fr] max-[1080px]:grid-cols-1">
        <PageSection>
          <Skeleton className="h-3.5 w-20 mb-3" />
          <Skeleton className="h-6 w-3/4 mb-5" />
          <div className="grid gap-4">
            {[0, 1, 2].map((i) => (
              <div key={i}>
                <Skeleton className="h-3 w-20 mb-2" />
                <div className="flex flex-wrap gap-2">
                  {[0, 1, 2].map((j) => (
                    <Skeleton key={j} className="h-7 w-20 rounded-full" />
                  ))}
                </div>
              </div>
            ))}
          </div>
        </PageSection>
        <PageSection>
          <Skeleton className="h-3.5 w-24 mb-3" />
          <Skeleton className="h-6 w-2/3 mb-5" />
          <Skeleton className="h-5 w-full mb-2" />
          <Skeleton className="h-5 w-5/6 mb-2" />
          <Skeleton className="h-5 w-4/5 mb-5" />
          <div className="grid gap-2">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-14 rounded-[14px]" />
            ))}
          </div>
        </PageSection>
      </Grid>
      <PageSection>
        <Skeleton className="h-3.5 w-16 mb-3" />
        <Skeleton className="h-6 w-2/5 mb-5" />
        <div className="grid gap-2">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-16 rounded-[14px]" />
          ))}
        </div>
      </PageSection>
    </PageStack>
  );
}

export function LearningPage() {
  const { entryId = "" } = useParams();
  const [data, setData] = useState<Awaited<ReturnType<typeof fetchLearningEntry>> | null>(() =>
    entryId ? getPrerenderLearningEntry(entryId) : null,
  );
  const [error, setError] = useState<string | null>(null);
  const entry = data?.entry;

  useEffect(() => {
    const initial = entryId ? getPrerenderLearningEntry(entryId) : null;
    if (initial) {
      setData(initial);
      setError(null);
      return;
    }
    let cancelled = false;
    setData(null);
    setError(null);
    void fetchLearningEntry(entryId)
      .then((response) => {
        if (cancelled) return;
        setData(response);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Could not load this learning page.");
      });
    return () => {
      cancelled = true;
    };
  }, [entryId]);

  const metaTitle = entry ? `${entry.title} · AimMod Learn` : "Aim Training Guide · AimMod Hub";
  const metaDescription = entry?.summary || "Aim training guides from AimMod covering mechanics, flaws, scenarios, and sensitivity.";

  const sourceLine = useMemo(() => {
    if (!entry?.sources.length) return null;
    return entry.sources
      .map((source) => source.author || source.title)
      .filter(Boolean)
      .join(" · ");
  }, [entry]);

  if (!data && !error) return <PageSkeleton />;
  if (data && !entry) {
    return (
      <PageStack>
        <PageSeo title="Aim Training Guide · AimMod Hub" description="This guide is unavailable." noindex />
        <PageSection>
          <SectionHeader eyebrow="Learn" title="Could not load this guide" body="The learning entry payload was missing required content." />
          <Button to="/learn">Back to learning pages</Button>
        </PageSection>
      </PageStack>
    );
  }

  return (
    <PageStack>
      <PageSeo title={metaTitle} description={metaDescription} type="article" noindex={Boolean(error) || !entry}
        schema={entry ? { "@context": "https://schema.org", "@type": "Article", headline: entry.title, description: entry.summary,
          mainEntityOfPage: `https://aimmod.app/learn/${encodeURIComponent(entry.id)}`, dateModified: data?.updatedAtIso,
          author: { "@type": "Organization", name: "AimMod Hub", url: "https://aimmod.app" },
          image: "https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png" } : undefined} />

      {error ? (
        <PageSection>
          <SectionHeader eyebrow="Learn" title="Could not load this guide" body={error} />
          <Button to="/learn">Back to learning pages</Button>
        </PageSection>
      ) : null}

      {entry ? (
        <>
          <PageSection className="relative overflow-hidden border-cyan/18 bg-[radial-gradient(circle_at_top_left,rgba(94,233,255,0.10),transparent_22%),radial-gradient(circle_at_78%_18%,rgba(255,214,102,0.08),transparent_18%),linear-gradient(135deg,rgba(8,18,20,0.98),rgba(6,12,16,0.96)_52%,rgba(5,8,11,0.98))] shadow-[0_24px_80px_rgba(0,0,0,0.42)]">
            <Breadcrumb crumbs={[{ label: "Learn", to: "/learn" }, { label: entry.title }]} />
            <div className="mt-3 max-w-[920px]">
              <h1 className="text-[clamp(26px,4.4vw,52px)] leading-[0.95] tracking-[-0.05em] text-text">
                {entry.title}
              </h1>
              <p className="mt-3 max-w-[720px] text-[15px] leading-7 text-[#cbe4d7]">{entry.summary}</p>
              <div className="mt-4 flex flex-wrap items-center gap-2">
                <span className={`rounded-full border px-3 py-1 text-[12px] ${priorityTone(entry.priority)}`}>
                  {humanizeToken(entry.priority || "reference")}
                </span>
                {entry.drills.length > 0 && (
                  <span className="rounded-full border border-line bg-white/[0.03] px-3 py-1 text-[12px] text-muted">
                    {entry.drills.length} drill{entry.drills.length !== 1 ? "s" : ""}
                  </span>
                )}
                {entry.sources.length > 0 && (
                  <span className="rounded-full border border-line bg-white/[0.03] px-3 py-1 text-[12px] text-muted">
                    {entry.sources.length} source{entry.sources.length !== 1 ? "s" : ""}
                  </span>
                )}
                {sourceLine ? (
                  <span className="rounded-full border border-line bg-white/[0.02] px-3 py-1 text-[12px] text-muted-2">
                    {sourceLine}
                  </span>
                ) : null}
              </div>
            </div>
          </PageSection>

          <Grid className="grid-cols-[0.75fr_1.25fr] max-[1080px]:grid-cols-1">
            <PageSection>
              <SectionHeader eyebrow="At a glance" title="Topic & context" />
              <div className="grid gap-4">
                <BadgeRow title="Scenario families" values={entry.scenarioTypes} tone="cyan" />
                <BadgeRow title="Context tags" values={entry.contextTags} />
                <BadgeRow title="Signal keys" values={entry.signalKeys} />
                <BadgeRow title="Focus areas" values={entry.focusAreas} />
                {entry.flaw ? (
                  <div className="rounded-[14px] border border-gold/20 bg-gold/[0.025] p-4">
                    <div className="text-[11px] uppercase tracking-[0.1em] text-gold">Primary flaw</div>
                    <h3 className="mt-2 text-[16px] font-medium tracking-[-0.03em] text-text">
                      {entry.flaw.title}
                    </h3>
                    <p className="mt-1.5 text-[13px] leading-6 text-muted">{entry.flaw.summary}</p>
                    {entry.flaw.telltales.length ? (
                      <div className="mt-3">
                        <div className="mb-2 text-[11px] uppercase tracking-[0.1em] text-muted-2">Telltales</div>
                        <div className="grid gap-1.5">
                          {entry.flaw.telltales.map((telltale) => (
                            <div
                              key={telltale}
                              className="rounded-[10px] border border-line bg-white/[0.02] px-3 py-2 text-[13px] leading-6 text-text"
                            >
                              {telltale}
                            </div>
                          ))}
                        </div>
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </div>
            </PageSection>

            <PageSection>
              <SectionHeader
                eyebrow="Summary"
                title="Key takeaways"
                body="The main point of this guide and the first steps to act on it."
              />
              <div className="rounded-[16px] border border-line bg-white/[0.025] p-5">
                <p className="text-[14px] leading-[1.75] text-text">{entry.summary}</p>
                {entry.actions.length ? (
                  <div className="mt-5 grid gap-2">
                    {entry.actions.slice(0, 3).map((action, index) => (
                      <div
                        key={`summary-action-${index}`}
                        className="flex gap-4 rounded-[12px] border border-mint/14 bg-mint/[0.03] px-4 py-3"
                      >
                        <div className="w-5 shrink-0 text-right text-[16px] font-medium leading-none text-mint/50 tabular-nums mt-0.5">
                          {index + 1}
                        </div>
                        <p className="text-[13px] leading-6 text-[#d6f1e3]">{action}</p>
                      </div>
                    ))}
                    {entry.actions.length > 3 && (
                      <div className="mt-1 text-[12px] text-muted-2">
                        +{entry.actions.length - 3} more actions below
                      </div>
                    )}
                  </div>
                ) : null}
              </div>
            </PageSection>
          </Grid>

          <WhyList items={entry.why} />
          <ActionsList items={entry.actions} />
          <AvoidList items={entry.avoid} />

          {entry.drills.length ? (
            <PageSection>
              <SectionHeader eyebrow="Drills" title="Useful drills" />
              <div className="grid gap-3 md:grid-cols-2">
                {entry.drills.map((drill) => (
                  <div
                    key={`${drill.label}-${drill.query}`}
                    className="rounded-[16px] border border-line bg-white/[0.025] p-4"
                  >
                    <div className="text-[11px] uppercase tracking-[0.1em] text-cyan">Drill</div>
                    <h3 className="mt-2 text-[16px] font-medium tracking-[-0.03em] text-text">{drill.label}</h3>
                    <p className="mt-2 text-[13px] leading-6 text-muted">{drill.reason}</p>
                    <div className="mt-3 flex items-center gap-2 rounded-[10px] border border-line bg-white/[0.02] px-3 py-2">
                      <span className="shrink-0 text-[11px] uppercase tracking-[0.08em] text-muted-2">Query</span>
                      <span className="text-[12px] text-[#cfe1d7] font-mono">{drill.query}</span>
                    </div>
                  </div>
                ))}
              </div>
            </PageSection>
          ) : null}

          {entry.mechanics.length ? (
            <PageSection>
              <SectionHeader eyebrow="Mechanics" title="Aim mechanics explained" />
              <div className="grid gap-3 md:grid-cols-2">
                {entry.mechanics.map((mechanic) => (
                  <MechanicCard key={mechanic.id} mechanic={mechanic} />
                ))}
              </div>
            </PageSection>
          ) : null}

          {entry.scenarios.length ? (
            <PageSection>
              <SectionHeader eyebrow="Scenarios" title="Related training scenarios" />
              <div className="grid gap-3 md:grid-cols-2">
                {entry.scenarios.map((scenario) => (
                  <ScenarioCard key={scenario.id} scenario={scenario} />
                ))}
              </div>
            </PageSection>
          ) : null}

          {entry.evidence.length ? (
            <PageSection>
              <SectionHeader eyebrow="Evidence" title="Source-backed claims" />
              <div className="grid gap-3">
                {entry.evidence.map((evidence, index) => (
                  <div
                    key={`${evidence.sourceId}-${index}`}
                    className="rounded-[16px] border border-line bg-white/[0.025] p-4"
                  >
                    <div className="flex flex-wrap items-center gap-2 mb-3">
                      <span className="rounded-full border border-mint/25 bg-mint/[0.05] px-2.5 py-0.5 text-[11px] uppercase tracking-[0.08em] text-mint">
                        {confidenceLabel(evidence.confidence)}
                      </span>
                      <span className="text-[11px] uppercase tracking-[0.08em] text-muted-2">
                        {humanizeToken(evidence.reviewStatus)}
                      </span>
                    </div>
                    <p className="text-[14px] leading-6 text-text">{evidence.claim}</p>
                    {evidence.excerpt ? (
                      <blockquote className="mt-3 border-l-2 border-line pl-4 text-[13px] leading-6 text-muted italic">
                        {evidence.excerpt}
                      </blockquote>
                    ) : null}
                  </div>
                ))}
              </div>
            </PageSection>
          ) : null}

          {entry.sources.length ? (
            <PageSection>
              <SectionHeader eyebrow="Sources" title="Research & references" />
              <div className="grid gap-3 md:grid-cols-2">
                {entry.sources.map((source) => (
                  <a
                    key={source.id}
                    href={source.url}
                    target="_blank"
                    rel="noreferrer"
                    className="group rounded-[16px] border border-line bg-white/[0.025] p-4 transition-all hover:border-cyan/30 hover:bg-white/[0.04] hover:-translate-y-px"
                  >
                    <div className="text-[11px] uppercase tracking-[0.08em]">
                      <SourceKindLabel kind={source.kind} />
                    </div>
                    <h3 className="mt-2 text-[16px] font-medium tracking-[-0.03em] text-text transition-colors group-hover:text-cyan">
                      {source.title}
                    </h3>
                    <p className="mt-1.5 text-[13px] text-muted">
                      {source.author}
                      {source.publishedAtIso ? ` · ${source.publishedAtIso.slice(0, 10)}` : ""}
                    </p>
                    <div className="mt-4 text-[12px] text-cyan/70 transition-colors group-hover:text-cyan">
                      Open source —&gt;
                    </div>
                  </a>
                ))}
              </div>
            </PageSection>
          ) : null}

          {data.relatedEntries.length ? (
            <PageSection>
              <SectionHeader
                eyebrow="Related"
                title="Related guides"
                body="Other guides covering related mechanics, training methods, and aim concepts."
              />
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {data.relatedEntries.map((entry) => (
                  <RelatedCard key={entry.id} entry={entry} />
                ))}
              </div>
            </PageSection>
          ) : null}
        </>
      ) : null}
    </PageStack>
  );
}

export default LearningPage;
