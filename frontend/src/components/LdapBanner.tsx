import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiJSON } from "../api/client";
import type { LdapConfig } from "../api/types";
import { useAuth } from "../auth/AuthContext";

const DISMISS_KEY = "sharedesk_ldap_banner_dismissed";

export default function LdapBanner() {
  const { hasPermission } = useAuth();
  const navigate = useNavigate();
  const [needsConfig, setNeedsConfig] = useState(false);

  useEffect(() => {
    if (!hasPermission("ldap.manage") || localStorage.getItem(DISMISS_KEY) === "1") return;
    apiJSON<LdapConfig>("/settings/ldap")
      .then((cfg) => setNeedsConfig(!cfg.enabled))
      .catch(() => {});
  }, [hasPermission]);

  if (!needsConfig) return null;

  function dismiss() {
    localStorage.setItem(DISMISS_KEY, "1");
    setNeedsConfig(false);
  }

  return (
    <div className="banner">
      <span>Un annuaire AD/LDAP n'est pas configuré sur cette instance.</span>
      <button onClick={() => navigate("/admin/ldap")}>Configurer maintenant</button>
      <button className="banner-dismiss" onClick={dismiss}>Ne plus afficher</button>
    </div>
  );
}
