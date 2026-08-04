import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import LdapBanner from "./LdapBanner";

export default function Layout() {
  const { user, logout, hasPermission } = useAuth();

  return (
    <div className="layout">
      <header className="topbar">
        <span className="brand">ShareDesk</span>
        <nav>
          <NavLink to="/agents">Machines</NavLink>
          {hasPermission("audit.read") && <NavLink to="/admin/audit">Logs</NavLink>}
          {(hasPermission("users.manage") || hasPermission("roles.manage") || hasPermission("settings.manage")) && (
            <NavLink to="/admin">Administration</NavLink>
          )}
        </nav>
        <div className="user-menu">
          <span>{user?.display_name} ({user?.role})</span>
          <button onClick={logout}>Déconnexion</button>
        </div>
      </header>
      <LdapBanner />
      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}
