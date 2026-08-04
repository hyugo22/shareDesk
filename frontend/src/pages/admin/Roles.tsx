import { useEffect, useState, type FormEvent } from "react";
import { apiJSON } from "../../api/client";
import type { Permission, Role } from "../../api/types";

export default function Roles() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [selected, setSelected] = useState<Role | null>(null);
  const [rolePerms, setRolePerms] = useState<Set<string>>(new Set());
  const [newRole, setNewRole] = useState({ name: "", description: "" });
  const [error, setError] = useState<string | null>(null);

  async function load() {
    const [r, p] = await Promise.all([apiJSON<Role[]>("/roles"), apiJSON<Permission[]>("/permissions")]);
    setRoles(r ?? []);
    setPermissions(p ?? []);
  }

  useEffect(() => {
    load();
  }, []);

  async function selectRole(role: Role) {
    setSelected(role);
    const perms = await apiJSON<string[]>(`/roles/${role.id}/permissions`);
    setRolePerms(new Set(perms ?? []));
  }

  function togglePerm(key: string) {
    setRolePerms((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  async function savePermissions() {
    if (!selected) return;
    await apiJSON(`/roles/${selected.id}/permissions`, {
      method: "PUT",
      body: JSON.stringify({ permissions: Array.from(rolePerms) }),
    });
  }

  async function createRole(e: FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      await apiJSON("/roles", { method: "POST", body: JSON.stringify(newRole) });
      setNewRole({ name: "", description: "" });
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Création impossible");
    }
  }

  async function deleteRole(role: Role) {
    if (role.is_system) return;
    await apiJSON(`/roles/${role.id}`, { method: "DELETE" });
    load();
  }

  return (
    <div className="roles-grid">
      <div>
        <h2>Rôles</h2>
        {error && <p className="error">{error}</p>}
        <ul className="list">
          {roles.map((r) => (
            <li key={r.id} className={selected?.id === r.id ? "selected" : ""}>
              <button onClick={() => selectRole(r)}>{r.name}</button>
              {!r.is_system && <button onClick={() => deleteRole(r)}>Supprimer</button>}
            </li>
          ))}
        </ul>
        <form className="inline-form" onSubmit={createRole}>
          <input placeholder="Nom du rôle" value={newRole.name} onChange={(e) => setNewRole({ ...newRole, name: e.target.value })} required />
          <input placeholder="Description" value={newRole.description} onChange={(e) => setNewRole({ ...newRole, description: e.target.value })} />
          <button type="submit">Créer un rôle</button>
        </form>
      </div>

      {selected && (
        <div>
          <h2>Permissions — {selected.name}</h2>
          <ul className="list">
            {permissions.map((p) => (
              <li key={p.id}>
                <label>
                  <input type="checkbox" checked={rolePerms.has(p.key)} onChange={() => togglePerm(p.key)} />
                  {p.key} — {p.description}
                </label>
              </li>
            ))}
          </ul>
          <button onClick={savePermissions}>Enregistrer</button>
        </div>
      )}
    </div>
  );
}
