import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiJSON, downloadsBaseURL } from "../api/client";

export default function AddMachine() {
  const navigate = useNavigate();
  const downloadBase = `${downloadsBaseURL()}/downloads`;
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

          <h2>1. Télécharger l'agent</h2>
          <div className="button-row">
            <a className="button-link" href={`${downloadBase}/ShareDeskAgent.msi`}>Windows (installeur .msi)</a>
            <a className="button-link" href={`${downloadBase}/sharedesk-agent-windows-amd64.exe`}>Windows (.exe seul)</a>
            <a className="button-link" href={`${downloadBase}/sharedesk-agent-linux-amd64`}>Linux (amd64)</a>
            <a className="button-link" href={`${downloadBase}/sharedesk-agent-darwin-arm64`}>macOS (Apple Silicon)</a>
          </div>
          <p className="hint">
            Publiés automatiquement par la CI à chaque mise à jour de la plateforme.
          </p>

          <h2>2. Installer</h2>
          <p><strong>Windows — installeur MSI (recommandé, silencieux, déployable par GPO)</strong></p>
          <pre className="code-block">
{`msiexec /i ShareDeskAgent.msi /qn ^
  SERVER_URL="https://${serverHost}:8080" ^
  MTLS_HOST="${serverHost}:8443" ^
  ENROLLMENT_TOKEN="${token}"`}
          </pre>
          <p className="hint">
            Installe et démarre automatiquement le service Windows « ShareDesk Agent »,
            avec une icône de statut dans la zone de notification (flèche ↑ à côté de
            l'horloge) visible à la prochaine connexion de l'utilisateur — vert
            connecté, bleu session en cours, rouge déconnecté.
          </p>

          <p><strong>Windows — exécutable seul (si tu ne peux pas utiliser le MSI)</strong></p>
          <p className="hint">
            Ne pas double-cliquer dessus : ça ouvre puis referme aussitôt une
            fenêtre sans rien faire, faute de paramètres. Lance-le depuis une
            invite de commandes (idéalement en administrateur) :
          </p>
          <pre className="code-block">
{`sharedesk-agent-windows-amd64.exe install ^
  --server-url="https://${serverHost}:8080" ^
  --mtls-host="${serverHost}:8443" ^
  --token="${token}"`}
          </pre>
          <p className="hint">Fait exactement la même chose que le MSI : enrôlement puis installation du service Windows.</p>

          <p><strong>Linux / macOS — exécution manuelle</strong></p>
          <pre className="code-block">
{`chmod +x sharedesk-agent-*
SERVER_URL=https://${serverHost}:8080 \\
SERVER_MTLS_HOST=${serverHost}:8443 \\
ENROLLMENT_TOKEN=${token} \\
./sharedesk-agent-linux-amd64`}
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
