import { Suspense, lazy, type ComponentType, type PropsWithChildren } from "react";
import { Link, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { AuthProvider } from "./lib/AuthContext";
import { LearningHubPage } from "./pages/LearningHubPage";
import LearningPage from "./pages/LearningPage";
import LearningTopicPage from "./pages/LearningTopicPage";
import { OsuLearningPage } from "./pages/OsuLearningPage";
import { RouteSeo } from "./components/PageSeo";
import { RouteErrorBoundary } from "./components/RouteErrorBoundary";
import { useSavedPageFilters } from "./hooks/useSavedPageFilters";

const AccountPage = lazy(() => import("./pages/AccountPage").then((m) => ({ default: m.AccountPage })));
const BrandingPage = lazy(() => import("./pages/BrandingPage").then((m) => ({ default: m.BrandingPage })));
const AdminPage = lazy(() => import("./pages/AdminPage").then((m) => ({ default: m.AdminPage })));
const AdminCoachingPage = lazy(() => import("./pages/AdminCoachingPage").then((m) => ({ default: m.AdminCoachingPage })));
const CommunityPage = lazy(() => import("./pages/CommunityPage").then((m) => ({ default: m.CommunityPage })));
const BenchmarkPage = lazy(() => import("./pages/BenchmarkPage").then((m) => ({ default: m.BenchmarkPage })));
const BenchmarksPage = lazy(() => import("./pages/BenchmarksPage").then((m) => ({ default: m.BenchmarksPage })));
const GlobalBenchmarksPage = lazy(() => import("./pages/GlobalBenchmarksPage").then((m) => ({ default: m.GlobalBenchmarksPage })));
const BenchmarkLeaderboardPage = lazy(() => import("./pages/BenchmarkLeaderboardPage").then((m) => ({ default: m.BenchmarkLeaderboardPage })));
const DeviceLinkPage = lazy(() => import("./pages/DeviceLinkPage").then((m) => ({ default: m.DeviceLinkPage })));
const HomePage = lazy(() => import("./pages/HomePage").then((m) => ({ default: m.HomePage })));
const LeaderboardPage = lazy(() => import("./pages/LeaderboardPage").then((m) => ({ default: m.LeaderboardPage })));
const LivePage = lazy(() => import("./pages/LivePage").then((m) => ({ default: m.LivePage })));
const OsuDownloadPage = lazy(() => import("./pages/OsuDownloadPage").then((m) => ({ default: m.OsuDownloadPage })));
const OsuCommunityPage = lazy(() => import("./pages/OsuCommunityPage").then((m) => ({ default: m.OsuCommunityPage })));
const OsuDirectoryPage = lazy(() => import("./pages/OsuDirectoryPage").then((m) => ({ default: m.OsuDirectoryPage })));
const OsuCatalogPage = lazy(() => import("./pages/OsuCatalogPage").then((m) => ({ default: m.OsuCatalogPage })));
const OsuPpTargetsPage = lazy(() => import("./pages/OsuPpTargetsPage").then((m) => ({ default: m.OsuPpTargetsPage })));
const OsuProfilePage = lazy(() => import("./pages/OsuProfilePage").then((m) => ({ default: m.OsuProfilePage })));
const OsuReplayPage = lazy(() => import("./pages/OsuReplayPage").then((m) => ({ default: m.OsuReplayPage })));
const ProductsPage = lazy(() => import("./pages/ProductsPage").then((m) => ({ default: m.ProductsPage })));
const PlayerScenarioPage = lazy(() => import("./pages/PlayerScenarioPage").then((m) => ({ default: m.PlayerScenarioPage })));
const ProfilePage = lazy(() => import("./pages/ProfilePage").then((m) => ({ default: m.ProfilePage })));
const ReplayHubPage = lazy(() => import("./pages/ReplayHubPage").then((m) => ({ default: m.ReplayHubPage })));
const RunPage = lazy(() => import("./pages/RunPage").then((m) => ({ default: m.RunPage })));
const ScenarioPage = lazy(() => import("./pages/ScenarioPage").then((m) => ({ default: m.ScenarioPage })));
const AimModPage = lazy(() => import("./pages/AimModPage").then((m) => ({ default: m.AimModPage })));
const SearchPage = lazy(() => import("./pages/SearchPage").then((m) => ({ default: m.SearchPage })));
const ExternalProfilePage = lazy(() => import("./pages/ExternalProfilePage").then((m) => ({ default: m.ExternalProfilePage })));
const ExternalKovaaksPage = lazy(() => import("./pages/ExternalProfilePage").then((m) => ({ default: m.ExternalKovaaksPage })));
const ExternalBenchmarkPage = lazy(() => import("./pages/ExternalBenchmarkPage").then((m) => ({ default: m.ExternalBenchmarkPage })));

const OsuPlayersPage = lazy(() => import("./pages/OsuPlayersPage").then(m => ({ default: m.OsuPlayersPage })));

function RouteLoading() {
  return <div role="status" className="flex min-h-64 items-center justify-center border-y border-line px-6 py-10 text-base text-muted">Loading page...</div>;
}

type RouterProps = PropsWithChildren<Record<string, unknown>>;

type AppProps = {
  RouterComponent: ComponentType<RouterProps>;
  routerProps?: Record<string, unknown>;
};

function AppRoutes() {
  useSavedPageFilters();
  return (
    <AppShell>
      <RouteSeo />
      <RouteErrorBoundary><Suspense fallback={<RouteLoading />}>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/app" element={<ProductsPage />} />
          <Route path="/branding" element={<BrandingPage />} />
          <Route path="/app/osu" element={<OsuDownloadPage />} />
          <Route path="/osu/community" element={<OsuCommunityPage />} />
          <Route path="/osu/learn" element={<OsuLearningPage />} />
          <Route path="/osu/learn/:slug" element={<OsuLearningPage />} />
          <Route path="/osu/beatmaps" element={<OsuCatalogPage key="beatmaps" />} />
          <Route path="/osu/pp-targets" element={<OsuPpTargetsPage />} />
          <Route path="/osu/skins" element={<OsuCatalogPage key="skins" skins />} />
          <Route path="/osu/players" element={<OsuPlayersPage />} />
          <Route path="/osu/replays" element={<OsuCommunityPage replayLibrary />} />
          <Route path="/osu/profiles/:handle" element={<OsuProfilePage />} />
          <Route path="/osu/replays/:shareId" element={<OsuReplayPage />} />
          <Route path="/osu/scores/:officialScoreId" element={<OsuReplayPage />} />
          <Route path="/app/kovaaks" element={<AimModPage />} />
          <Route path="/community" element={<CommunityPage />} />
          <Route path="/learn" element={<LearningHubPage />} />
          <Route path="/learn/topics/:topic" element={<LearningTopicPage />} />
          <Route path="/learn/:entryId" element={<LearningPage />} />
          <Route path="/live" element={<LivePage />} />
          <Route path="/osu" element={<OsuDirectoryPage />} />
          <Route path="/benchmarks" element={<GlobalBenchmarksPage />} />
          <Route path="/benchmarks/:benchmarkId" element={<BenchmarkLeaderboardPage />} />
          <Route path="/profiles/:handle/benchmarks/:benchmarkId" element={<BenchmarkPage />} />
          <Route path="/profiles/:handle/benchmarks" element={<BenchmarksPage />} />
          <Route path="/replays" element={<ReplayHubPage />} />
          <Route path="/account" element={<AccountPage />} />
          <Route path="/admin" element={<AdminPage />} />
          <Route path="/admin/coaching" element={<AdminCoachingPage />} />
          <Route path="/link-device" element={<DeviceLinkPage />} />
          <Route path="/search" element={<SearchPage />} />
          <Route path="/leaderboard" element={<LeaderboardPage />} />
          <Route path="/profiles/:handle" element={<ProfilePage />} />
          <Route path="/profiles/:handle/scenarios/:slug" element={<PlayerScenarioPage />} />
          <Route path="/scenarios/:slug" element={<ScenarioPage />} />
          <Route path="/runs/:runId" element={<RunPage />} />
          <Route path="/u/:steamId" element={<ExternalProfilePage />} />
          <Route path="/u/:steamId/benchmarks/:benchmarkId" element={<ExternalBenchmarkPage />} />
          <Route path="/u/kovaaks/:kovaaksUsername" element={<ExternalKovaaksPage />} />
          <Route path="*" element={<section className="py-8"><h1 className="text-2xl">Page not found</h1><Link className="text-cyan" to="/">Return to AimMod Hub</Link></section>} />
        </Routes>
      </Suspense></RouteErrorBoundary>
    </AppShell>
  );
}

export function App({ RouterComponent, routerProps }: AppProps) {
  return (
    <AuthProvider>
      <RouterComponent {...routerProps}>
        <AppRoutes />
      </RouterComponent>
    </AuthProvider>
  );
}
