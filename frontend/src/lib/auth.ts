import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

type User = { id: string; email: string; displayName: string };
type AuthState = {
  user: User | null;
  accessToken: string | null;
  setSession: (session: { user: User; accessToken: string }) => void;
  clear: () => void;
};

export const useAuth = create<AuthState>()(persist((set) => ({
  user: null, accessToken: null,
  setSession: (session) => set(session),
  clear: () => set({ user: null, accessToken: null })
}), { name: "fpl-edge-session", storage: createJSONStorage(() => sessionStorage) }));

