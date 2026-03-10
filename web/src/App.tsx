import { Suspense, lazy } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { AuthProvider } from "./lib/AuthContext";

const AccountPage = lazy(() => import("./pages/AccountPage").then((m) => ({ default: m.AccountPage })));
const AdminPage = lazy(() => import("./pages/AdminPage").then((m) => ({ default: m.AdminPage })));
const CommunityPage = lazy(() => import("./pages/CommunityPage").then((m) => ({ default: m.CommunityPage })));
const DeviceLinkPage = lazy(() => import("./pages/DeviceLinkPage").then((m) => ({ default: m.DeviceLinkPage })));
const HomePage = lazy(() => import("./pages/HomePage").then((m) => ({ default: m.HomePage })));
const LeaderboardPage = lazy(() => import("./pages/LeaderboardPage").then((m) => ({ default: m.LeaderboardPage })));
const PlayerScenarioPage = lazy(() => import("./pages/PlayerScenarioPage").then((m) => ({ default: m.PlayerScenarioPage })));
const ProfilePage = lazy(() => import("./pages/ProfilePage").then((m) => ({ default: m.ProfilePage })));
const RunPage = lazy(() => import("./pages/RunPage").then((m) => ({ default: m.RunPage })));
const ScenarioPage = lazy(() => import("./pages/ScenarioPage").then((m) => ({ default: m.ScenarioPage })));
const AimModPage = lazy(() => import("./pages/AimModPage").then((m) => ({ default: m.AimModPage })));
const SearchPage = lazy(() => import("./pages/SearchPage").then((m) => ({ default: m.SearchPage })));

function RouteLoading() {
  return <div className="rounded-[18px] border border-line bg-white/2 px-6 py-10 text-sm text-muted">Loading page...</div>;
}

export function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <AppShell>
          <Suspense fallback={<RouteLoading />}>
            <Routes>
              <Route path="/" element={<HomePage />} />
              <Route path="/app" element={<AimModPage />} />
              <Route path="/community" element={<CommunityPage />} />
              <Route path="/account" element={<AccountPage />} />
              <Route path="/admin" element={<AdminPage />} />
              <Route path="/link-device" element={<DeviceLinkPage />} />
              <Route path="/search" element={<SearchPage />} />
              <Route path="/leaderboard" element={<LeaderboardPage />} />
              <Route path="/profiles/:handle" element={<ProfilePage />} />
              <Route path="/profiles/:handle/scenarios/:slug" element={<PlayerScenarioPage />} />
              <Route path="/scenarios/:slug" element={<ScenarioPage />} />
              <Route path="/runs/:runId" element={<RunPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
        </AppShell>
      </BrowserRouter>
    </AuthProvider>
  );
}
