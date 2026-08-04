import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiJSON } from "../api/client";
import type { Agent, ControlSession } from "../api/types";
import { useAuth } from "../auth/AuthContext";

export default function AgentList() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [connectingId, setConnectingId] = useState<string | null>(null);
  const navigate = useNavigate();
  const { hasPermission } = useAuth();

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const data = await apiJSON<Agent[]>("/agents");
        if (!cancelled) setAgents(data);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Erreur de chargement");
      }
    }
    load();
    const interval = setInterval(load, 10_000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  async function connect(agent: Agent) {
    setConnectingId(agent.id);
    try {
      const session = await apiJSON<ControlSession>("/sessions", {
        method: "POST",
        body: JSON.stringify({ agent_id: agent.id }),
      });
      navigate(`/sessions/${session.id}`, { state: { agentName: agent.name } });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Impossible de démarrer la session");
    } finally {
      setConnectingId(null);
    }
  }

  return (
    <div>
      <h1>Machines</h1>
      {error && <p className="error">{error}</p>}
      <table className="table">
        <thead>
          <tr>
            <th>Nom</th>
            <th>OS</th>
            <th>Statut</th>
            <th>Dernière connexion</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {agents.map((a) => (
            <tr key={a.id}>
              <td>{a.name}</td>
              <td>{a.os} {a.os_version}</td>
              <td>
                <span className={`badge badge-${a.status}`}>{a.status}</span>
              </td>
              <td>{a.last_seen_at ? new Date(a.last_seen_at).toLocaleString() : "—"}</td>
              <td>
                {hasPermission("agents.control") && (
                  <button
                    disabled={a.status !== "online" || connectingId === a.id}
                    onClick={() => connect(a)}
                  >
                    {connectingId === a.id ? "Connexion…" : "Contrôler"}
                  </button>
                )}
              </td>
            </tr>
          ))}
          {agents.length === 0 && (
            <tr>
              <td colSpan={5}>Aucun agent enrôlé.</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
