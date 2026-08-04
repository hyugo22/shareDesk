import { useState, type FormEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { ApiError } from "../api/client";

export default function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const from = (location.state as { from?: Location })?.from?.pathname ?? "/agents";

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [mfaRequired, setMfaRequired] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(email, password, totpCode);
      navigate(from, { replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.message === "mfa_required") {
        setMfaRequired(true);
      } else {
        setError(err instanceof Error ? err.message : "Erreur de connexion");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="centered">
      <form className="card login-form" onSubmit={onSubmit}>
        <h1>ShareDesk</h1>
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus />
        </label>
        <label>
          Mot de passe
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        {mfaRequired && (
          <label>
            Code MFA (application d'authentification)
            <input value={totpCode} onChange={(e) => setTotpCode(e.target.value)} required autoFocus />
          </label>
        )}
        {error && <p className="error">{error}</p>}
        <button type="submit" disabled={submitting}>
          {submitting ? "Connexion…" : "Se connecter"}
        </button>
      </form>
    </div>
  );
}
