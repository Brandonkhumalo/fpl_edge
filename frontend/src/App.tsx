import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { AuthPage } from "./pages/AuthPage";
import { DashboardPage } from "./pages/DashboardPage";
import { LandingPage } from "./pages/LandingPage";
import { OnboardingPage } from "./pages/OnboardingPage";
import { PlaceholderPage } from "./pages/PlaceholderPage";

const protectedPages: [string,string,string][] = [
  ["/my-squad/edit","Squad editor","Replace owned players within budget and squad constraints."],["/my-squad/lineup","Starting XI","Compare and apply a legal optimized lineup."],["/my-squad/history","Squad history","Audit prior revisions and confirmed changes."],
  ["/scout","Player scout","Search provider-backed players and compare future projections."],["/scout/watchlist","Watchlist","Track players without changing the saved squad."],["/fixtures","Fixture analysis","Review verified fixtures, blanks, doubles and fixture swings."],["/predictions","Predictions","Inspect timestamped numerical projections and model provenance."],["/captaincy","Captaincy advisor","Compare safe, upside and differential captain options from owned starters."],
  ["/transfers","Transfer centre","Preview or confirm internal squad transfers."],["/transfers/planner","Transfer planner","Plan legal changes across multiple gameweeks."],["/transfers/history","Transfer history","Review simulated, internal and marked-official transfers."],
  ["/chips","Chip centre","Review remaining chips and evidence-based opportunities."],["/chips/wildcard","Wildcard builder","Simulate a permanent optimized squad before confirmation."],["/chips/free-hit","Free Hit builder","Simulate one temporary gameweek squad."],["/chips/triple-captain","Triple Captain","Compare upcoming captaincy opportunities."],["/chips/bench-boost","Bench Boost","Evaluate incremental expected bench value."],
  ["/optimizer","Squad optimizer","Generate legal lineups and squads under active-season rules."],["/simulator","Scenario simulator","Compare isolated strategies without mutating the saved squad."],["/analytics","Analytics","Compare historical expected and actual performance."],["/advisor","AI advisor","Ask questions grounded in validated engine outputs."],["/notifications","Notifications","Review deadline, injury and optimization alerts."],["/settings","Settings","Manage account, strategy and notification preferences."]
];

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
        <Route path="/my-squad" element={<DashboardPage />} />
        <Route path="/onboarding" element={<OnboardingPage />} />
        {protectedPages.map(([path, title, description]) => (
          <Route key={path} path={path} element={<PlaceholderPage title={title} description={description} />} />
        ))}
        <Route path="/scout/players/:id" element={<PlaceholderPage title="Player details" description="Inspect verified statistics, fixtures and timestamped projections." />} />
      </Route>
    </Route>
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes>;
}

