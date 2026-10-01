import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import type { BenchmarkSummary, GetBenchmarkPageResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { BenchmarkSheetView } from "../components/BenchmarkSheetView";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { fetchBenchmarkPage, fetchProfile, slugifyScenarioName } from "../lib/api";
import { API_BASE_URL } from "../lib/config";
import { Helmet } from "../lib/helmet";

/** A linked AimMod player's benchmark sheet. */
export function BenchmarkPage() {
  const { handle = "", benchmarkId = "" } = useParams();
  const [page, setPage] = useState<GetBenchmarkPageResponse | null>(null);
  const [profileBenchmarks, setProfileBenchmarks] = useState<BenchmarkSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const parsedId = Number(benchmarkId);

  useEffect(() => {
    let cancelled = false;
    setPage(null);
    setError(null);
    if (!Number.isFinite(parsedId) || parsedId <= 0) {
      setError("This benchmark page is not available.");
      return () => { cancelled = true; };
    }
    void fetchBenchmarkPage(handle, parsedId)
      .then((next) => { if (!cancelled) setPage(next); })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this benchmark."); });
    void fetchProfile(handle).then((p) => { if (!cancelled) setProfileBenchmarks(p.benchmarks ?? []); }).catch(() => {});
    return () => { cancelled = true; };
  }, [parsedId, handle]);

  // Refresh when the player uploads new scores.
  useEffect(() => {
    if (!handle || typeof EventSource === "undefined") return;
    const es = new EventSource(`${API_BASE_URL}/api/events?handle=${encodeURIComponent(handle)}`, { withCredentials: true });
    es.addEventListener("scores_updated", () => {
      if (Number.isFinite(parsedId) && parsedId > 0) void fetchBenchmarkPage(handle, parsedId).then(setPage).catch(() => {});
    });
    return () => es.close();
  }, [handle, parsedId]);

  if (error) {
    const noLink = /linked Steam/i.test(error);
    return (
      <PageStack>
        <EmptyState title={noLink ? "This profile has no linked KovaaK's account yet." : "This benchmark could not be loaded."}
          body={noLink ? "Benchmark ranks appear once AimMod has seen this player signed in to Steam in KovaaK's." : undefined}>
          <Button to={`/profiles/${handle}`}>Back to profile</Button>
        </EmptyState>
      </PageStack>
    );
  }
  if (!page) return <PageStack><PageSkeleton label="Loading benchmark" /></PageStack>;

  const playerName = page.userDisplayName || page.userHandle || handle;
  return (
    <PageStack>
      <Helmet><title>{`${page.benchmarkName} · ${playerName} · AimMod Hub`}</title></Helmet>
      <BenchmarkSheetView
        page={page}
        playerHref={`/profiles/${page.userHandle || handle}`}
        playerName={playerName}
        benchmarkHref={(id) => `/profiles/${page.userHandle || handle}/benchmarks/${id}`}
        scenarioHref={(scenario) => `/profiles/${page.userHandle || handle}/scenarios/${scenario.scenarioSlug || slugifyScenarioName(scenario.scenarioName)}`}
        playerBenchmarks={profileBenchmarks}
      />
    </PageStack>
  );
}
