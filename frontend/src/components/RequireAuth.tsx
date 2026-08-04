import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";

export function RequireAuth({ children }: { children: React.ReactElement }) {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) return <div className="centered">Chargement…</div>;
  if (!user) return <Navigate to="/login" state={{ from: location }} replace />;
  return children;
}

export function RequirePermission({ perm, children }: { perm: string; children: React.ReactElement }) {
  const { hasPermission } = useAuth();
  if (!hasPermission(perm)) return <div className="centered">Accès refusé.</div>;
  return children;
}
