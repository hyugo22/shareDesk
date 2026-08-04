export interface Me {
  id: string;
  email: string;
  display_name: string;
  role: string;
  mfa_enabled: boolean;
  permissions: string[];
}

export interface Agent {
  id: string;
  name: string;
  hostname: string;
  os: string;
  os_version: string;
  arch: string;
  agent_version: string;
  tags: string[] | null;
  status: "online" | "offline";
  enrolled_at: string;
  last_seen_at: string | null;
  revoked_at: string | null;
}

export interface ControlSession {
  id: string;
  agent_id: string;
  user_id: string;
  status: string;
  started_at: string;
  ended_at: string | null;
}

export interface Role {
  id: string;
  name: string;
  description: string;
  is_system: boolean;
}

export interface Permission {
  id: string;
  key: string;
  description: string;
}

export interface User {
  id: string;
  email: string;
  display_name: string;
  role_id: string;
  role_name: string;
  mfa_enabled: boolean;
  is_active: boolean;
  last_login_at: string | null;
}

export interface LdapConfig {
  enabled: boolean;
  host: string;
  port: number;
  connection_mode: "ldaps" | "starttls";
  bind_dn: string;
  has_bind_password: boolean;
  base_dn: string;
  attribute_mapping: Record<string, string>;
  group_role_mapping: Record<string, string>;
  sync_interval_minutes: number;
  last_sync_at: string | null;
  last_sync_status?: string;
}

export interface AuditLog {
  id: number;
  occurred_at: string;
  actor_type: string;
  actor_user_id?: string;
  action: string;
  target_type?: string;
  target_id?: string;
  ip_address?: string;
  details?: Record<string, unknown>;
}
