import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { TournamentStatus, type TournamentSummary } from "../gen/aimmod/tournament/v1/tournament_pb";
import { PageSeo } from "../components/PageSeo";
import { PageHeader } from "../components/ui/PageHeader";
import { Tabs } from "../components/ui/Tabs";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { useAuth } from "../lib/AuthContext";
import { errorMessage, formatLabels, formatWhen, statusLabels, tournamentClient } from "../lib/tournaments";

const tabs = [
  { key: "open", label: "Open and upcoming", statuses: [TournamentStatus.REGISTRATION, TournamentStatus.CHECK_IN] },
  { key: "live", label: "In progress", statuses: [TournamentStatus.IN_PROGRESS] },
  { key: "done", label: "Completed", statuses: [TournamentStatus.COMPLETED] },
  { key: "mine", label: "Mine", statuses: [] as TournamentStatus[] },
] as const;

export function TournamentsPage() {
  const auth = useAuth();
  const [tab, setTab] = useState<(typeof tabs)[number]["key"]>("open");
  const [items, setItems] = useState<TournamentSummary[] | null>(null);
  const [cursor, setCursor] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setItems(null);
    setError(null);
    const t = tabs.find((x) => x.key === tab)!;
    tournamentClient.listTournaments({ statuses: [...t.statuses], mine: tab === "mine", limit: 30 })
      .then((res) => { if (!cancelled) { setItems(res.tournaments); setCursor(res.nextCursor); } })
      .catch((err) => { if (!cancelled) setError(errorMessage(err, "Could not load tournaments.")); });
    return () => { cancelled = true; };
  }, [tab]);

  async function more() {
    const t = tabs.find((x) => x.key === tab)!;
    try {
      const res = await tournamentClient.listTournaments({ statuses: [...t.statuses], mine: tab === "mine", limit: 30, cursor });
      setItems((prev) => [...(prev ?? []), ...res.tournaments]);
      setCursor(res.nextCursor);
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  return (
    <PageStack>
      <PageSeo title="KovaaK's Tournaments · AimMod Hub" description="Brackets, check-in and live matches for AimMod tournaments in KovaaK's. Results are verified with replays and never touch KovaaK's ranked leaderboards." />
      <PageSection>
        <PageHeader
          title="Tournaments"
          meta="Bracket events played in KovaaK's. Results are checked with replays and never affect ranked leaderboards."
          actions={auth.authenticated ? <Button to="/tournaments/new" variant="primary">Create a tournament</Button> : null}
        />
        <Tabs label="Tournament list" value={tab} onChange={setTab} className="mt-4"
          tabs={tabs.filter((t) => t.key !== "mine" || auth.authenticated).map((t) => [t.key, t.label] as const)} />
      </PageSection>
      <PageSection>
        {error ? <EmptyState title="Tournaments could not be loaded." body={error} />
          : !items ? Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="mb-2 h-16" />)
          : items.length === 0 ? (
            <EmptyState title={tab === "mine" ? "You have not entered a tournament yet." : tab === "live" ? "No tournaments are running." : tab === "done" ? "No finished tournaments yet." : "No tournaments are open."}>
              {tab !== "open" ? <Button onClick={() => setTab("open")}>See open tournaments</Button> : auth.authenticated ? <Button to="/tournaments/new">Create a tournament</Button> : null}
            </EmptyState>
          )
          : (
            <ul className="divide-y divide-line border-y border-line">
              {items.map(({ tournament: t, self }) => t && (
                <li key={t.id}>
                  <Link to={`/tournaments/${encodeURIComponent(t.slug || t.id)}`} className="flex flex-col gap-1 px-1 py-3 hover:bg-white/2 md:flex-row md:items-center md:justify-between">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-semibold text-text">{t.name}</div>
                      <div className="text-[12px] text-muted">
                        {formatLabels[t.format]} · best of {t.ruleset?.bestOf ?? 1} · {t.entrantCount}/{t.maxEntrants} players
                        {t.startsAt ? ` · starts ${formatWhen(t.startsAt)}` : ""}
                      </div>
                    </div>
                    <div className="flex shrink-0 items-center gap-2 text-[12px]">
                      {self ? <span className="rounded-full border border-mint/40 px-2 py-0.5 text-mint">Entered</span> : null}
                      <span className="rounded-full border border-line px-2 py-0.5 text-muted">{statusLabels[t.status]}</span>
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        {cursor ? <div className="mt-3"><Button onClick={() => void more()}>Load more</Button></div> : null}
      </PageSection>
    </PageStack>
  );
}
