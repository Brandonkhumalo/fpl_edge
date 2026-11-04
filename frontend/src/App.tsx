import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { AuthPage } from "./pages/AuthPage";
import { DashboardPage } from "./pages/DashboardPage";
import { LandingPage } from "./pages/LandingPage";
import { OnboardingPage } from "./pages/OnboardingPage";
import { PlaceholderPage } from "./pages/PlaceholderPage";

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
      </Route>
    </Route>
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes>;
}

