import { Navigate, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import { RequireAuth, RequirePermission } from "./components/RequireAuth";
import Login from "./pages/Login";
import AgentList from "./pages/AgentList";
import SessionViewer from "./pages/SessionViewer";
import AdminLayout from "./pages/admin/AdminLayout";
import Users from "./pages/admin/Users";
import Roles from "./pages/admin/Roles";
import AgentsAdmin from "./pages/admin/AgentsAdmin";
import AuditLogs from "./pages/admin/AuditLogs";
import LdapSettings from "./pages/admin/LdapSettings";

export default function App() {
  return (
    <Routes>
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
