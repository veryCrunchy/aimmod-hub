import { createContext, useContext, useEffect, useState, type PropsWithChildren } from "react";
import { createUploadToken, fetchSession, logout, revokeUploadToken, type SessionPayload } from "./auth";

type AuthContextValue = SessionPayload & {
  loading: boolean;
  refresh: () => Promise<void>;
  signOut: () => Promise<void>;
  mintUploadToken: (label: string) => Promise<{ token: string; record: { id: number; label: string; lastFour: string; createdAt: string; lastUsedAt?: string | null } }>;
  revokeToken: (id: number) => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const [session, setSession] = useState<SessionPayload>({ authenticated: false });
  const [loading, setLoading] = useState(true);

  async function refresh() {
    try {
      const next = await fetchSession();
      setSession(next);
    } catch {
      setSession({ authenticated: false });
    } finally {
      setLoading(false);
    }
  }

  async function signOut() {
    await logout();
    await refresh();
  }

  async function mintUploadToken(label: string) {
    const created = await createUploadToken(label);
    await refresh();
    return created;
  }

  async function revokeToken(id: number) {
    await revokeUploadToken(id);
    await refresh();
  }

  useEffect(() => {
    void refresh();
  }, []);

  return (
    <AuthContext.Provider value={{ ...session, loading, refresh, signOut, mintUploadToken, revokeToken }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return value;
}
