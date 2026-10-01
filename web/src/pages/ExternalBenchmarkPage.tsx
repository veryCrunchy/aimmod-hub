import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import type { BenchmarkSummary, GetBenchmarkPageResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { BenchmarkSheetView } from "../components/BenchmarkSheetView";
import { KovaaksPlayerNotice } from "../components/KovaaksPlayerNotice";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { fetchKovaaksBenchmarkPage, fetchKovaaksPlayer, slugifyScenarioName } from "../lib/api";
import { Helmet } from "../lib/helmet";

/** Any KovaaK's player's benchmark sheet, from KovaaK's public data only. */
export function ExternalBenchmarkPage() {
  const { steamId = "", benchmarkId = "" } = useParams();
  const [page, setPage] = useState<GetBenchmarkPageResponse | null>(null);
  const [benchmarks, setBenchmarks] = useState<BenchmarkSummary[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState(false);
  const parsedId = Number(benchmarkId);

  useEffect(() => {
    let cancelled = false;
    setPage(null);
    setError(false);
    if (!steamId || !Number.isFinite(parsedId) || parsedId <= 0) {
      setError(true);
      return () => { cancelled = true; };
    }
    void fetchKovaaksBenchmarkPage(steamId, parsedId)
      .then((next) => { if (!cancelled) setPage(next); })
      .catch(() => { if (!cancelled) setError(true); });
    void fetchKovaaksPlayer(steamId).then((player) => {
      if (cancelled) return;
      setBenchmarks(player.benchmarks);
      setName(player.kovaaksUsername || player.steamName);
    }).catch(() => {});
    return () => { cancelled = true; };
  }, [steamId, parsedId]);

  if (error) {
    return (
      <PageStack>
        <EmptyState title="This benchmark could not be loaded." body="KovaaK's may be busy. Try again in a moment.">
          <Button to={`/u/${steamId}`}>Back to player</Button>
        </EmptyState>
      </PageStack>
    );
  }
  if (!page) return <PageStack><PageSkeleton label="Loading benchmark" /></PageStack>;

  const playerName = name || page.kovaaksUsername || page.userDisplayName || steamId;
  return (
    <PageStack>
      <Helmet><title>{`${page.benchmarkName} · ${playerName} · KovaaK's · AimMod Hub`}</title></Helmet>
      <BenchmarkSheetView
        page={page}
        playerHref={`/u/${steamId}`}
        playerName={playerName}
        benchmarkHref={(id) => `/u/${steamId}/benchmarks/${id}`}
        scenarioHref={(scenario) => `/scenarios/${scenario.scenarioSlug || slugifyScenarioName(scenario.scenarioName)}?tab=kovaaks&name=${encodeURIComponent(scenario.scenarioName)}&player=${steamId}`}
        playerBenchmarks={benchmarks}
        benchmarksHref={null}
        notice={<KovaaksPlayerNotice steamId={steamId} aimmodHandle={page.aimmodHandle} name={playerName} />}
      />
    </PageStack>
  );
}
