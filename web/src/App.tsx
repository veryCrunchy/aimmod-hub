import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { AuthProvider } from "./lib/AuthContext";
import { AccountPage } from "./pages/AccountPage";
import { AdminPage } from "./pages/AdminPage";
import { CommunityPage } from "./pages/CommunityPage";
import { DeviceLinkPage } from "./pages/DeviceLinkPage";
import { HomePage } from "./pages/HomePage";
import { LeaderboardPage } from "./pages/LeaderboardPage";
import { PlayerScenarioPage } from "./pages/PlayerScenarioPage";
import { ProfilePage } from "./pages/ProfilePage";
import { RunPage } from "./pages/RunPage";
import { ScenarioPage } from "./pages/ScenarioPage";
import { SearchPage } from "./pages/SearchPage";

export function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <AppShell>
          <Routes>
            <Route path="/" element={<HomePage />} />
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
        </AppShell>
      </BrowserRouter>
    </AuthProvider>
  );
}
