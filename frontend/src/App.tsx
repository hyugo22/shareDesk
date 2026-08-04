import { useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { apiJSON } from "./api/client";
import Layout from "./components/Layout";
import { RequireAuth, RequirePermission } from "./components/RequireAuth";
import Setup from "./pages/Setup";
import Login from "./pages/Login";
import AgentList from "./pages/AgentList";
import SessionViewer from "./pages/SessionViewer";
import AdminLayout from "./pages/admin/AdminLayout";
import Users from "./pages/admin/Users";
import Roles from "./pages/admin/Roles";
import AgentsAdmin from "./pages/admin/AgentsAdmin";
import AuditLogs from "./pages/admin/AuditLogs";
import LdapSettings from "./pages/admin/LdapSettings";

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
        <Route path="/sessions/:sessionId" element={<SessionViewer />} />

        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<Navigate to="users" replace />} />
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
