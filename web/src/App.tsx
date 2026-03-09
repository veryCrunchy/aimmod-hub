import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { AuthProvider } from "./lib/AuthContext";
import { AccountPage } from "./pages/AccountPage";
import { CommunityPage } from "./pages/CommunityPage";
import { DeviceLinkPage } from "./pages/DeviceLinkPage";
import { HomePage } from "./pages/HomePage";
import { ProfilePage } from "./pages/ProfilePage";
import { RunPage } from "./pages/RunPage";
import { ScenarioPage } from "./pages/ScenarioPage";

export function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <AppShell>
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/community" element={<CommunityPage />} />
            <Route path="/account" element={<AccountPage />} />
            <Route path="/link-device" element={<DeviceLinkPage />} />
            <Route path="/profiles/:handle" element={<ProfilePage />} />
            <Route path="/scenarios/:slug" element={<ScenarioPage />} />
            <Route path="/runs/:runId" element={<RunPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </AppShell>
      </BrowserRouter>
    </AuthProvider>
  );
}
