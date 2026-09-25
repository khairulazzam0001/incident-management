import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { api, getStoredToken, setStoredToken, ApiError, USER_KEY } from "../api/client";
import type { User } from "../api/types";

interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function loadStoredUser(): User | null {
  try {
    const raw = localStorage.getItem(USER_KEY);
    if (!raw) return null;
    const v: unknown = JSON.parse(raw);
    if (typeof v !== "object" || v === null) return null;
    const o = v as Record<string, unknown>;
    if (typeof o.id !== "string" || typeof o.role !== "string") return null;
    return v as User;
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(() => loadStoredUser());
  const [isLoading, setIsLoading] = useState<boolean>(() => getStoredToken() !== null);

  // Verifikasi token tersimpan saat aplikasi dimuat.
  useEffect(() => {
    if (getStoredToken() === null) {
      setIsLoading(false);
      return;
    }
    let cancelled = false;
    api
      .getMeta()
      .then(() => {
        if (!cancelled) setIsLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 401) {
          setStoredToken(null);
          try {
            localStorage.removeItem(USER_KEY);
          } catch {
            // abaikan
          }
          setUser(null);
        }
        setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function login(email: string, password: string): Promise<void> {
    const res = await api.login(email, password);
    setStoredToken(res.token);
    try {
      localStorage.setItem(USER_KEY, JSON.stringify(res.user));
    } catch {
      // abaikan
    }
    setUser(res.user);
  }

  function logout(): void {
    setStoredToken(null);
    try {
      localStorage.removeItem(USER_KEY);
    } catch {
      // abaikan
    }
    setUser(null);
  }

  return (
    <AuthContext.Provider value={{ user, isLoading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const v = useContext(AuthContext);
  if (v === null) throw new Error("useAuth dipakai di luar AuthProvider.");
  return v;
}

export function isCoordinator(role: string): boolean {
  return role === "HelpDesk" || role === "SystemAnalyst" || role === "ManagerLead";
}
