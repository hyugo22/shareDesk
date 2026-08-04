import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { apiJSON, clearTokens, getRefreshToken, setTokens } from "../api/client";
import type { Me } from "../api/types";

interface AuthState {
  user: Me | null;
  loading: boolean;
  login: (email: string, password: string, totpCode?: string) => Promise<void>;
  logout: () => void;
  hasPermission: (perm: string) => boolean;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  const loadMe = useCallback(async () => {
    try {
      const me = await apiJSON<Me>("/me");
      setUser(me);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (getRefreshToken()) {
      loadMe();
    } else {
      setLoading(false);
    }
  }, [loadMe]);

  const login = useCallback(async (email: string, password: string, totpCode?: string) => {
    const data = await apiJSON<{ access_token: string; refresh_token: string }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password, totp_code: totpCode ?? "" }),
    });
    setTokens(data.access_token, data.refresh_token);
    await loadMe();
  }, [loadMe]);

  const logout = useCallback(() => {
    const refresh = getRefreshToken();
    clearTokens();
    setUser(null);
    if (refresh) {
      apiJSON("/auth/logout", { method: "POST", body: JSON.stringify({ refresh_token: refresh }) }).catch(() => {});
    }
  }, []);

  const hasPermission = useCallback((perm: string) => !!user?.permissions?.includes(perm), [user]);

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, hasPermission }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth doit être utilisé dans <AuthProvider>");
  return ctx;
}
