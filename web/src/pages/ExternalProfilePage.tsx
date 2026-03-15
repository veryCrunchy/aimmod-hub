import { useEffect, useState } from "react";
import { Helmet } from "react-helmet-async";
import { Link, useParams } from "react-router-dom";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { lookupExternalUser, type ExternalProfileResponse } from "../lib/api";

function AimModBadge() {
  return (
    <span className="inline-flex items-center gap-1 rounded-full border border-cyan/25 bg-cyan/8 px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.08em] text-cyan">
      AimMod user
    </span>
  );
}

function ExternalBadge() {
  return (
    <span className="inline-flex items-center gap-1 rounded-full border border-line bg-white/4 px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.08em] text-muted">
      KovaaK's only
    </span>
  );
}

function BenchmarkCard({
  steamId,
  benchmark,
}: {
  steamId: string;
  benchmark: ExternalProfileResponse["benchmarks"][number];
}) {
  return (
    <Link
      to={`/u/${encodeURIComponent(steamId)}/benchmarks/${benchmark.benchmarkId}`}
      className="flex items-center gap-3 rounded-[14px] border border-line bg-white/2 p-3.5 transition-colors hover:border-cyan/30 hover:bg-white/[0.035]"
    >
      {benchmark.benchmarkIconUrl ? (
        <img
          src={benchmark.benchmarkIconUrl}
          alt=""
          className="h-9 w-9 shrink-0 rounded-[10px] border border-white/10 object-cover"
        />
      ) : (
        <div className="h-9 w-9 shrink-0 rounded-[10px] border border-line bg-white/4" />
      )}
      <div className="min-w-0 flex-1">
        <div className="truncate text-[13px] font-medium text-text">{benchmark.benchmarkName}</div>
        {benchmark.benchmarkAuthor && (
          <div className="text-[10px] text-muted/60">by {benchmark.benchmarkAuthor}</div>
        )}
      </div>
      <div className="shrink-0 text-right">
        {benchmark.overallRankName ? (
          <div className="flex items-center gap-1.5">
            {benchmark.overallRankIcon && (
              <img
                src={benchmark.overallRankIcon}
                alt=""
                className="h-5 w-5 rounded-md border border-white/10 object-cover"
              />
            )}
            <span
              className="text-[12px] font-medium"
              style={{ color: benchmark.overallRankColor || "#a7c2b3" }}
            >
              {benchmark.overallRankName}
            </span>
          </div>
        ) : (
          <span className="text-[11px] text-muted/50">No rank</span>
        )}
      </div>
    </Link>
  );
}

/** Route: /u/kovaaks/:kovaaksUsername — looks up by KovaaK's username, no Steam64 required. */
export function ExternalKovaaksPage() {
  const { kovaaksUsername = "" } = useParams();
  return <ExternalProfilePage overrideQuery={kovaaksUsername} overrideSteamId="" />;
}

export function ExternalProfilePage({ overrideQuery, overrideSteamId }: { overrideQuery?: string; overrideSteamId?: string } = {}) {
  const { steamId: routeSteamId = "" } = useParams();
  const steamId = overrideSteamId !== undefined ? overrideSteamId : routeSteamId;
  const query = overrideQuery ?? steamId;
  const [data, setData] = useState<ExternalProfileResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!query) return;
    let cancelled = false;
    setData(null);
    setError(null);
    void lookupExternalUser(query)
      .then((next) => { if (!cancelled) setData(next); })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this player."); });
    return () => { cancelled = true; };
  }, [query]);

  if (error) {
    return (
      <PageStack>
        <Card className="p-4.5">
          <EmptyState
            title="Player not found"
            body={error}
          />
        </Card>
      </PageStack>
    );
  }

  if (!data) {
    return (
      <PageStack>
        <Card className="p-4.5">
          <Skeleton className="mb-3 h-3 w-24" />
          <Skeleton className="mb-3 h-8 w-48" />
          <Skeleton className="h-4 w-64" />
        </Card>
        <div className="grid gap-2">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-[68px] rounded-[14px]" />
          ))}
        </div>
      </PageStack>
    );
  }

  const displayName = data.aimmodDisplayName || data.kovaaksUsername || data.resolvedSteamId || steamId;
  const profileUrl = data.isAimmodUser && data.aimmodHandle ? `/profiles/${data.aimmodHandle}` : null;

  return (
    <PageStack>
      <Helmet>
        <title>{displayName} · KovaaK's Benchmarks · AimMod Hub</title>
      </Helmet>
      <Card className="p-3.5 md:p-4.5">
        <Breadcrumb crumbs={[{ label: "Players", to: "/community" }, { label: displayName }]} />
        <div className="mt-2 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-base font-medium text-text">{displayName}</h1>
              {data.isAimmodUser ? <AimModBadge /> : <ExternalBadge />}
            </div>
            <p className="mt-1 text-[11px] text-muted/70">
              Steam ID: {data.resolvedSteamId || steamId}
              {data.kovaaksUsername && data.kovaaksUsername !== displayName && (
                <> · KovaaK's: {data.kovaaksUsername}</>
              )}
            </p>
          </div>
          {profileUrl && (
            <Link
              to={profileUrl}
              className="shrink-0 rounded-full border border-cyan/25 bg-cyan/8 px-3 py-1.5 text-[11px] text-cyan transition-colors hover:border-cyan/40 hover:bg-cyan/12"
            >
              View AimMod profile →
            </Link>
          )}
        </div>
        {data.isAimmodUser && (
          <div className="mt-3 rounded-[10px] border border-cyan/15 bg-cyan/5 px-3 py-2 text-[11px] text-cyan/80">
            This player uses AimMod — their profile includes mouse path replays, aim fingerprint
            analysis, and per-run coaching on top of benchmark data.
          </div>
        )}
      </Card>

      {data.benchmarks.length > 0 ? (
        <div>
          <div className="mb-2 px-0.5 text-[11px] uppercase tracking-widest text-muted/60">
            {data.benchmarks.length} benchmark{data.benchmarks.length !== 1 ? "s" : ""}
          </div>
          <div className="grid gap-2">
            {data.benchmarks.map((b) => (
              <BenchmarkCard key={b.benchmarkId} steamId={steamId} benchmark={b} />
            ))}
          </div>
        </div>
      ) : (
        <Card className="p-4.5">
          <EmptyState
            title="No benchmark ranks"
            body={`${displayName} hasn't achieved a rank on any KovaaK's benchmark yet.`}
          />
        </Card>
      )}
    </PageStack>
  );
}
