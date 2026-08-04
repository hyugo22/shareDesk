import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../../auth/AuthContext";

export default function AdminLayout() {
  const { hasPermission } = useAuth();

  return (
    <div>
      <h1>Administration</h1>
      <nav className="tabs">
        {hasPermission("users.manage") && <NavLink to="/admin/users">Utilisateurs</NavLink>}
        {hasPermission("roles.manage") && <NavLink to="/admin/roles">Rôles & permissions</NavLink>}
        {hasPermission("agents.manage") && <NavLink to="/admin/agents">Agents</NavLink>}
        {hasPermission("audit.read") && <NavLink to="/admin/audit">Logs d'audit</NavLink>}
        {hasPermission("ldap.manage") && <NavLink to="/admin/ldap">Intégration AD/LDAP</NavLink>}
      </nav>
      <div className="tab-content">
        <Outlet />
      </div>
    </div>
  );
}
