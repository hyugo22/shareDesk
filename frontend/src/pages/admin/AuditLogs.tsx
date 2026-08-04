import { useEffect, useState } from "react";
import { apiFetch, apiJSON } from "../../api/client";
import type { AuditLog } from "../../api/types";

export default function AuditLogs() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [action, setAction] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function load() {
    try {
      const qs = action ? `?action=${encodeURIComponent(action)}` : "";
      setLogs((await apiJSON<AuditLog[]>(`/audit-logs${qs}`)) ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erreur de chargement");
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function exportCSV() {
    const qs = action ? `?action=${encodeURIComponent(action)}&format=csv` : "?format=csv";
    const res = await apiFetch(`/audit-logs${qs}`);
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "audit-logs.csv";
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div>
      {error && <p className="error">{error}</p>}
      <div className="inline-form">
        <input placeholder="Filtrer par action (ex: auth.login.failure)" value={action} onChange={(e) => setAction(e.target.value)} />
        <button onClick={load}>Filtrer</button>
        <button onClick={exportCSV}>Exporter CSV</button>
      </div>
      <table className="table">
        <thead>
          <tr><th>Date</th><th>Acteur</th><th>Type</th><th>Action</th><th>Cible</th><th>IP</th></tr>
        </thead>
        <tbody>
          {logs.map((l) => (
            <tr key={l.id}>
              <td>{new Date(l.occurred_at).toLocaleString()}</td>
              <td>{l.actor_name ?? "—"}</td>
              <td>{l.actor_type}</td>
              <td>{l.action}</td>
              <td>{l.target_type ? `${l.target_type}:${l.target_id}` : "—"}</td>
              <td>{l.ip_address ?? "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
