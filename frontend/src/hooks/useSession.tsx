import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { useQuery } from "@tanstack/react-query";
import { api, resetSession, sessionEvents } from "../services/api";
import type { User } from "../services/types";
import { queryClient } from "../lib/queryClient";
type Session = {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};
const Context = createContext<Session | null>(null);
export function SessionProvider({ children }: { children: ReactNode }) {
  const [epoch, setEpoch] = useState(0);
  const session = useQuery({
    queryKey: ["session", epoch],
    queryFn: ({ signal }) => api<{ user: User }>("/auth/me", { signal }),
    retry: false,
  });
  useEffect(() => {
    const changed = () => {
      void queryClient.cancelQueries();
      queryClient.clear();
      setEpoch((v) => v + 1);
    };
    const cross = (e: StorageEvent) => {
      if (e.key === "sor:logout") resetSession();
    };
    sessionEvents.addEventListener("changed", changed);
    window.addEventListener("storage", cross);
    return () => {
      sessionEvents.removeEventListener("changed", changed);
      window.removeEventListener("storage", cross);
    };
  }, []);
  const login = async (email: string, password: string) => {
    await api("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
    resetSession();
  };
  const logout = async () => {
    await api("/auth/logout", { method: "POST" });
    resetSession();
    localStorage.setItem("sor:logout", String(Date.now()));
  };
  return (
    <Context.Provider
      value={{
        user: session.data?.user ?? null,
        loading: session.isLoading,
        login,
        logout,
      }}
    >
      {children}
    </Context.Provider>
  );
}
export function useSession() {
  const value = useContext(Context);
  if (!value) throw new Error("SessionProvider requerido");
  return value;
}
