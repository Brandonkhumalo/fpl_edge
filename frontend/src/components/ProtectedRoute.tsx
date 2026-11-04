import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../lib/auth";

export function ProtectedRoute() {
  const authenticated = useAuth((s) => Boolean(s.accessToken)); const location = useLocation();
  return authenticated ? <Outlet/> : <Navigate to="/login" replace state={{ from: location.pathname }}/>;
}
