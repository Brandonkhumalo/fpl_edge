import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { AuthPage } from "./pages/AuthPage";
import { DashboardPage } from "./pages/DashboardPage";
import { LandingPage } from "./pages/LandingPage";
import { OnboardingPage } from "./pages/OnboardingPage";
import { PlaceholderPage } from "./pages/PlaceholderPage";
import { ManageTeamPage } from "./pages/ManageTeamPage";
import { ChipsPage } from "./pages/ChipsPage";
import { ScoutPage } from "./pages/ScoutPage";
import { SettingsPage } from "./pages/SettingsPage";
import { HistoryPage } from "./pages/HistoryPage";
import { AdvicePage } from "./pages/AdvicePage";
import { DeadlinePage, PlannerPage, RecommendationsPage, WatchlistPage } from "./pages/RetentionPages";
import { LivePage } from "./pages/LivePage";
import { AdvisorPage } from "./pages/AdvisorPage";
import { ImportSquadPage } from "./pages/ImportSquadPage";

export function App() {
  return <Routes>
    <Route path="/" element={<LandingPage />} />
    <Route path="/login" element={<AuthPage mode="login" />} />
    <Route path="/register" element={<AuthPage mode="register" />} />
    {["features", "how-it-works", "privacy", "terms", "forgot-password", "reset-password"].map((path) => (
      <Route key={path} path={`/${path}`} element={<PlaceholderPage title={path.split("-").map((part) => part[0].toUpperCase() + part.slice(1)).join(" ")} description="This public page is being completed with the corresponding production workflow." />} />
    ))}
    <Route element={<ProtectedRoute />}>
      <Route element={<AppShell />}>
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/advice" element={<AdvicePage />} />
        <Route path="/my-squad" element={<ManageTeamPage />} />
        <Route path="/my-squad/edit" element={<ManageTeamPage />} />
        <Route path="/my-squad/import" element={<ImportSquadPage />} />
        <Route path="/captaincy" element={<ManageTeamPage />} />
        <Route path="/transfers" element={<ManageTeamPage />} />
        <Route path="/chips/*" element={<ChipsPage />} />
        <Route path="/scout" element={<ScoutPage />} />
        <Route path="/deadline" element={<DeadlinePage />} />
        <Route path="/watchlist" element={<WatchlistPage />} />
        <Route path="/recommendations" element={<RecommendationsPage />} />
        <Route path="/planner" element={<PlannerPage />} />
        <Route path="/live" element={<LivePage />} />
        <Route path="/advisor" element={<AdvisorPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/my-squad/history" element={<HistoryPage />} />
        <Route path="/transfers/history" element={<HistoryPage />} />
        <Route path="/onboarding" element={<OnboardingPage />} />
      </Route>
    </Route>
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes>;
}

