import { useEffect, useState } from "react";
import { apiJSON } from "../../api/client";
import type { Agent } from "../../api/types";

export default function AgentsAdmin() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [tokenDescription, setTokenDescription] = useState("");
  const [issuedToken, setIssuedToken] = useState<string | null>(null);

  async function load() {
    try {
      setAgents(await apiJSON<Agent[]>("/agents"));
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

  async function createToken() {
    const data = await apiJSON<{ token: string; expires_in: number }>("/agents/enrollment-tokens", {
      method: "POST",
      body: JSON.stringify({ description: tokenDescription, ttl_minutes: 60 }),
    });
    setIssuedToken(data.token);
  }

  return (
    <div>
      {error && <p className="error">{error}</p>}

      <h2>Nouveau token d'enrôlement</h2>
      <p className="hint">Valide 60 minutes, usage unique. À fournir à l'installateur de l'agent.</p>
      <div className="inline-form">
        <input placeholder="Description (ex: postes comptabilité)" value={tokenDescription} onChange={(e) => setTokenDescription(e.target.value)} />
        <button onClick={createToken}>Générer</button>
      </div>
      {issuedToken && (
        <div className="card token-display">
          <code>{issuedToken}</code>
          <p className="hint">Ce token ne sera plus jamais affiché — copiez-le maintenant.</p>
        </div>
      )}

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
        </tbody>
      </table>
    </div>
  );
}
