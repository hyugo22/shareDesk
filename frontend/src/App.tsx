import { useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { apiJSON } from "./api/client";
import Layout from "./components/Layout";
import { RequireAuth, RequirePermission } from "./components/RequireAuth";
import { useAuth } from "./auth/AuthContext";
import Setup from "./pages/Setup";
import Login from "./pages/Login";
import AgentList from "./pages/AgentList";
import AddMachine from "./pages/AddMachine";
import SessionViewer from "./pages/SessionViewer";
import AdminLayout from "./pages/admin/AdminLayout";
import Users from "./pages/admin/Users";
import Roles from "./pages/admin/Roles";
import AgentsAdmin from "./pages/admin/AgentsAdmin";
import AuditLogs from "./pages/admin/AuditLogs";
import LdapSettings from "./pages/admin/LdapSettings";

/** Redirige vers le premier onglet Administration auquel l'utilisateur a
 * accès (plutôt que "users" en dur, plus forcément atteignable maintenant
 * que "Administration" peut n'accorder que audit.read). */
function AdminIndexRedirect() {
  const { hasPermission } = useAuth();
  const firstTab =
    (hasPermission("users.manage") && "users") ||
    (hasPermission("roles.manage") && "roles") ||
    (hasPermission("agents.manage") && "agents") ||
    (hasPermission("audit.read") && "audit") ||
    (hasPermission("ldap.manage") && "ldap") ||
    "users";
  return <Navigate to={firstTab} replace />;
}

/** true tant qu'aucun utilisateur n'existe en base : l'assistant de
 * configuration initiale doit alors passer avant toute autre page. */
function useNeedsSetup() {
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null);

  useEffect(() => {
    apiJSON<{ needs_setup: boolean }>("/setup/status")
      .then((r) => setNeedsSetup(r.needs_setup))
      .catch(() => setNeedsSetup(false)); // en cas d'erreur, ne pas bloquer l'accès au login
  }, []);

  return needsSetup;
}

export default function App() {
  const needsSetup = useNeedsSetup();

  if (needsSetup === null) return <div className="centered">Chargement…</div>;

  if (needsSetup) {
    return (
      <Routes>
        <Route path="/setup" element={<Setup />} />
        <Route path="*" element={<Navigate to="/setup" replace />} />
      </Routes>
    );
  }

  return (
    <Routes>
      <Route path="/setup" element={<Navigate to="/login" replace />} />
      <Route path="/login" element={<Login />} />

      <Route element={<RequireAuth><Layout /></RequireAuth>}>
        <Route path="/agents" element={<AgentList />} />
        <Route path="/agents/add" element={<RequirePermission perm="agents.manage"><AddMachine /></RequirePermission>} />
        <Route path="/sessions/:sessionId" element={<SessionViewer />} />

        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<AdminIndexRedirect />} />
          <Route path="users" element={<RequirePermission perm="users.manage"><Users /></RequirePermission>} />
          <Route path="roles" element={<RequirePermission perm="roles.manage"><Roles /></RequirePermission>} />
          <Route path="agents" element={<RequirePermission perm="agents.manage"><AgentsAdmin /></RequirePermission>} />
          <Route path="audit" element={<RequirePermission perm="audit.read"><AuditLogs /></RequirePermission>} />
          <Route path="ldap" element={<RequirePermission perm="ldap.manage"><LdapSettings /></RequirePermission>} />
        </Route>
      </Route>

      <Route path="/" element={<Navigate to="/agents" replace />} />
      <Route path="*" element={<Navigate to="/agents" replace />} />
    </Routes>
  );
}
