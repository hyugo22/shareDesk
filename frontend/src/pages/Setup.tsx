import { useState, type FormEvent } from "react";
import { apiJSON } from "../api/client";
import { useAuth } from "../auth/AuthContext";

export default function Setup() {
  const { applySession } = useAuth();

  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (password.length < 12) {
      setError("Le mot de passe doit faire au moins 12 caractères.");
      return;
    }
    if (password !== confirmPassword) {
      setError("Les mots de passe ne correspondent pas.");
      return;
    }
    setSubmitting(true);
    try {
      const data = await apiJSON<{ access_token: string; refresh_token: string }>("/setup", {
        method: "POST",
        body: JSON.stringify({ email, display_name: displayName, password }),
      });
      await applySession(data.access_token, data.refresh_token);
      // Rechargement complet (plutôt qu'une navigation react-router) pour que
      // App réévalue /setup/status : l'assistant ne doit plus jamais se
      // réafficher une fois le premier compte créé.
      window.location.href = "/admin/ldap";
    } catch (err) {
      setError(err instanceof Error ? err.message : "Configuration impossible");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="centered">
      <form className="card login-form" onSubmit={onSubmit}>
        <h1>Bienvenue sur ShareDesk</h1>
        <p className="hint">
          Première utilisation : créez le compte administrateur de cette instance.
          Vous pourrez ensuite configurer les paramètres généraux, les rôles et
          l'intégration AD/LDAP depuis la section Administration.
        </p>
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus />
        </label>
        <label>
          Nom affiché
          <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} required />
        </label>
        <label>
          Mot de passe (12 caractères minimum)
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={12} />
        </label>
        <label>
          Confirmer le mot de passe
          <input type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} required minLength={12} />
        </label>
        {error && <p className="error">{error}</p>}
        <button type="submit" disabled={submitting}>
          {submitting ? "Création…" : "Créer le compte administrateur"}
        </button>
      </form>
    </div>
  );
}
