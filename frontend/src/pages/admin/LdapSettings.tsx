import { useEffect, useState, type FormEvent } from "react";
import { apiJSON } from "../../api/client";
import type { LdapConfig } from "../../api/types";

export default function LdapSettings() {
  const [cfg, setCfg] = useState<LdapConfig | null>(null);
  const [password, setPassword] = useState("");
  const [testResult, setTestResult] = useState<string | null>(null);
  const [syncResult, setSyncResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setCfg(await apiJSON<LdapConfig>("/settings/ldap"));
  }

  useEffect(() => {
    load();
  }, []);

  async function save(e: FormEvent) {
    e.preventDefault();
    if (!cfg) return;
    setError(null);
    try {
      await apiJSON("/settings/ldap", {
        method: "PUT",
        body: JSON.stringify({ ...cfg, bind_password: password || undefined }),
      });
      setPassword("");
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Enregistrement impossible");
    }
  }

  async function testConnection() {
    setTestResult(null);
    const res = await apiJSON<{ success: boolean; error?: string }>("/settings/ldap/test", { method: "POST" });
    setTestResult(res.success ? "Connexion réussie." : `Échec : ${res.error}`);
  }

  async function syncNow() {
    setSyncResult(null);
    try {
      await apiJSON("/settings/ldap/sync", { method: "POST" });
      setSyncResult("Synchronisation effectuée.");
    } catch (err) {
      setSyncResult(err instanceof Error ? err.message : "Synchronisation impossible");
    }
  }

  if (!cfg) return <p>Chargement…</p>;

  return (
    <div>
      <p className="hint">
        Compte de service strictement en lecture seule côté annuaire (search/bind uniquement).
        Le mot de passe n'est jamais réaffiché une fois enregistré.
      </p>
      {error && <p className="error">{error}</p>}
      <form className="card settings-form" onSubmit={save}>
        <label className="checkbox-row">
          <input type="checkbox" checked={cfg.enabled} onChange={(e) => setCfg({ ...cfg, enabled: e.target.checked })} />
          Intégration activée
        </label>
        <label>Hôte<input value={cfg.host} onChange={(e) => setCfg({ ...cfg, host: e.target.value })} /></label>
        <label>Port<input type="number" value={cfg.port} onChange={(e) => setCfg({ ...cfg, port: Number(e.target.value) })} /></label>
        <label>
          Mode de connexion
          <select value={cfg.connection_mode} onChange={(e) => setCfg({ ...cfg, connection_mode: e.target.value as "ldaps" | "starttls" })}>
            <option value="ldaps">LDAPS</option>
            <option value="starttls">StartTLS</option>
          </select>
        </label>
        <label>DN du compte de service (lecture seule)<input value={cfg.bind_dn} onChange={(e) => setCfg({ ...cfg, bind_dn: e.target.value })} /></label>
        <label>
          Mot de passe {cfg.has_bind_password && <span className="hint">(déjà configuré — laisser vide pour conserver)</span>}
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
        </label>
        <label>Base DN<input value={cfg.base_dn} onChange={(e) => setCfg({ ...cfg, base_dn: e.target.value })} /></label>
        <label>Intervalle de synchro (min)<input type="number" value={cfg.sync_interval_minutes} onChange={(e) => setCfg({ ...cfg, sync_interval_minutes: Number(e.target.value) })} /></label>
        <div className="button-row">
          <button type="submit">Enregistrer</button>
          <button type="button" onClick={testConnection}>Tester la connexion</button>
          <button type="button" onClick={syncNow}>Synchroniser maintenant</button>
        </div>
      </form>
      {testResult && <p>{testResult}</p>}
      {syncResult && <p>{syncResult}</p>}
      {cfg.last_sync_at && <p className="hint">Dernière synchro : {new Date(cfg.last_sync_at).toLocaleString()} ({cfg.last_sync_status})</p>}
    </div>
  );
}
