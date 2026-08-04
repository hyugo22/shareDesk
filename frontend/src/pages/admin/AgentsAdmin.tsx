import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiJSON } from "../../api/client";
import type { Agent } from "../../api/types";

export default function AgentsAdmin() {
  const navigate = useNavigate();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    try {
      setAgents((await apiJSON<Agent[]>("/agents")) ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erreur de chargement");
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function revoke(agent: Agent) {
    const reason = prompt(`Motif de révocation pour ${agent.name} ?`) ?? "";
    await apiJSON(`/agents/${agent.id}/revoke`, { method: "POST", body: JSON.stringify({ reason }) });
    load();
  }

  return (
    <div>
      {error && <p className="error">{error}</p>}

      <div className="button-row">
        <button onClick={() => navigate("/agents/add")}>+ Ajouter une machine</button>
      </div>

      <h2>Agents enrôlés</h2>
      <table className="table">
        <thead>
          <tr><th>Nom</th><th>OS</th><th>Statut</th><th>Version</th><th>Enrôlé le</th><th /></tr>
        </thead>
        <tbody>
          {agents.map((a) => (
            <tr key={a.id}>
              <td>{a.name}</td>
              <td>{a.os}</td>
              <td><span className={`badge badge-${a.status}`}>{a.revoked_at ? "révoqué" : a.status}</span></td>
              <td>{a.agent_version || "—"}</td>
              <td>{new Date(a.enrolled_at).toLocaleString()}</td>
              <td>{!a.revoked_at && <button onClick={() => revoke(a)}>Révoquer</button>}</td>
            </tr>
          ))}
          {agents.length === 0 && (
            <tr><td colSpan={6}>Aucun agent enrôlé.</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
