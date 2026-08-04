import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiJSON } from "../api/client";

export default function AddMachine() {
  const navigate = useNavigate();
  const [description, setDescription] = useState("");
  const [ttlMinutes, setTtlMinutes] = useState(60);
  const [token, setToken] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const serverHost = window.location.hostname;

  async function generateToken() {
    setError(null);
    setSubmitting(true);
    try {
      const res = await apiJSON<{ token: string; expires_in: number }>("/agents/enrollment-tokens", {
        method: "POST",
        body: JSON.stringify({ description, ttl_minutes: ttlMinutes }),
      });
      setToken(res.token);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Génération du token impossible");
    } finally {
      setSubmitting(false);
    }
  }

  function copy(text: string) {
    navigator.clipboard?.writeText(text).catch(() => {});
  }

  return (
    <div>
      <button onClick={() => navigate("/agents")}>← Retour</button>
      <h1>Ajouter une machine</h1>
      <p className="hint">
        Génère un token d'enrôlement à usage unique, puis installe l'agent sur le
        poste à contrôler à distance avec ce token. L'agent génère lui-même sa
        paire de clés à l'enrôlement (jamais transmise) et échange le token
        contre un certificat client mTLS.
      </p>

      {!token ? (
        <form
          className="card settings-form"
          onSubmit={(e) => {
            e.preventDefault();
            generateToken();
          }}
        >
          <label>
            Description (ex: postes comptabilité, PC de Julie…)
            <input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optionnel" />
          </label>
          <label>
            Validité du token (minutes)
            <input
              type="number"
              min={5}
              max={1440}
              value={ttlMinutes}
              onChange={(e) => setTtlMinutes(Number(e.target.value))}
            />
          </label>
          {error && <p className="error">{error}</p>}
          <div className="button-row">
            <button type="submit" disabled={submitting}>
              {submitting ? "Génération…" : "Générer le token"}
            </button>
          </div>
        </form>
      ) : (
        <div className="card settings-form">
          <p>
            Token généré (valide {ttlMinutes} minutes, <strong>usage unique</strong> —
            il ne sera plus jamais affiché) :
          </p>
          <div className="token-display">
            <code>{token}</code>
            <button type="button" onClick={() => copy(token)}>Copier</button>
          </div>

          <p>Sur le poste à enrôler, exécute l'agent avec ces variables d'environnement :</p>
          <pre className="code-block">
{`SERVER_URL=https://${serverHost}:8080
SERVER_MTLS_HOST=${serverHost}:8443
ENROLLMENT_TOKEN=${token}
./sharedesk-agent`}
          </pre>
          <p className="hint">
            Adapte les ports si ton déploiement n'utilise pas les valeurs par défaut.
            Une fois enrôlé, l'agent apparaît automatiquement dans la liste des machines.
          </p>

          <div className="button-row">
            <button onClick={() => setToken(null)}>Générer un autre token</button>
            <button onClick={() => navigate("/agents")}>Voir les machines</button>
          </div>
        </div>
      )}
    </div>
  );
}
