import { useEffect, useState, type FormEvent } from "react";
import { apiJSON } from "../../api/client";
import type { Role, User } from "../../api/types";
import { useAuth } from "../../auth/AuthContext";

export default function Users() {
  const { user: currentUser } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [form, setForm] = useState({ email: "", display_name: "", password: "", role_id: "" });

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState({ email: "", display_name: "" });

  async function load() {
    try {
      const [u, r] = await Promise.all([apiJSON<User[]>("/users"), apiJSON<Role[]>("/roles")]);
      setUsers(u ?? []);
      setRoles(r ?? []);
      setForm((f) => ({ ...f, role_id: f.role_id || r?.[0]?.id || "" }));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erreur de chargement");
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function createUser(e: FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      await apiJSON("/users", { method: "POST", body: JSON.stringify(form) });
      setForm((f) => ({ ...f, email: "", display_name: "", password: "" }));
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Création impossible");
    }
  }

  async function toggleActive(u: User) {
    await apiJSON(`/users/${u.id}`, { method: "PATCH", body: JSON.stringify({ is_active: !u.is_active }) });
    load();
  }

  async function changeRole(u: User, roleId: string) {
    await apiJSON(`/users/${u.id}`, { method: "PATCH", body: JSON.stringify({ role_id: roleId }) });
    load();
  }

  function startEdit(u: User) {
    setEditingId(u.id);
    setEditForm({ email: u.email, display_name: u.display_name });
  }

  async function saveEdit(id: string) {
    setError(null);
    try {
      await apiJSON(`/users/${id}`, { method: "PATCH", body: JSON.stringify(editForm) });
      setEditingId(null);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Modification impossible");
    }
  }

  async function resetPassword(u: User) {
    const password = window.prompt(`Nouveau mot de passe pour ${u.email} (12 caractères minimum) :`);
    if (!password) return;
    if (password.length < 12) {
      setError("Le mot de passe doit faire au moins 12 caractères.");
      return;
    }
    await apiJSON(`/users/${u.id}`, { method: "PATCH", body: JSON.stringify({ password }) });
    setInfo(`Mot de passe réinitialisé pour ${u.email}.`);
  }

  async function deleteUser(u: User) {
    if (!window.confirm(`Supprimer le compte ${u.email} ? Cette action est irréversible.`)) return;
    setError(null);
    setInfo(null);
    try {
      const res = await apiJSON<{ anonymized: boolean }>(`/users/${u.id}`, { method: "DELETE" });
      setInfo(
        res.anonymized
          ? `${u.email} a des actions dans l'historique d'audit : le compte a été anonymisé et désactivé plutôt que supprimé physiquement.`
          : `${u.email} a été supprimé.`,
      );
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Suppression impossible");
    }
  }

  return (
    <div>
      {error && <p className="error">{error}</p>}
      {info && <p className="hint">{info}</p>}
      <table className="table">
        <thead>
          <tr>
            <th>Email</th><th>Nom</th><th>Rôle</th><th>MFA</th><th>Actif</th><th>Dernière connexion</th><th />
          </tr>
        </thead>
        <tbody>
          {users.map((u) => (
            <tr key={u.id}>
              {editingId === u.id ? (
                <>
                  <td><input value={editForm.email} onChange={(e) => setEditForm({ ...editForm, email: e.target.value })} /></td>
                  <td><input value={editForm.display_name} onChange={(e) => setEditForm({ ...editForm, display_name: e.target.value })} /></td>
                </>
              ) : (
                <>
                  <td>{u.email}</td>
                  <td>{u.display_name}</td>
                </>
              )}
              <td>
                <select value={u.role_id} onChange={(e) => changeRole(u, e.target.value)}>
                  {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
                </select>
              </td>
              <td>{u.mfa_enabled ? "Activé" : "—"}</td>
              <td>{u.is_active ? "Oui" : "Non"}</td>
              <td>{u.last_login_at ? new Date(u.last_login_at).toLocaleString() : "—"}</td>
              <td>
                {editingId === u.id ? (
                  <>
                    <button onClick={() => saveEdit(u.id)}>Enregistrer</button>
                    <button onClick={() => setEditingId(null)}>Annuler</button>
                  </>
                ) : (
                  <>
                    <button onClick={() => startEdit(u)}>Modifier</button>
                    <button onClick={() => resetPassword(u)}>Réinitialiser mdp</button>
                    <button onClick={() => toggleActive(u)}>{u.is_active ? "Désactiver" : "Activer"}</button>
                    {u.id !== currentUser?.id && <button onClick={() => deleteUser(u)}>Supprimer</button>}
                  </>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <h2>Créer un utilisateur</h2>
      <form className="inline-form" onSubmit={createUser}>
        <input placeholder="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
        <input placeholder="Nom affiché" value={form.display_name} onChange={(e) => setForm({ ...form, display_name: e.target.value })} required />
        <input placeholder="Mot de passe (12+ caractères)" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required minLength={12} />
        <select value={form.role_id} onChange={(e) => setForm({ ...form, role_id: e.target.value })}>
          {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
        </select>
        <button type="submit">Créer</button>
      </form>
    </div>
  );
}
