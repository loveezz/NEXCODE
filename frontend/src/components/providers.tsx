"use client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
const Theme = createContext({ dark: false, toggle: () => {} });
export const useTheme = () => useContext(Theme);
export default function Providers({ children }: { children: ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { staleTime: 20000, retry: 1 } },
      }),
  );
  const [dark, setDark] = useState(false);
  useEffect(() => {
    setDark(document.documentElement.dataset.theme === "dark");
  }, []);
  const toggle = () =>
    setDark((v) => {
      document.documentElement.dataset.theme = v ? "light" : "dark";
      try {
        localStorage.setItem("nexcode-theme", v ? "light" : "dark");
      } catch {}
      return !v;
    });
  return (
    <QueryClientProvider client={client}>
      <Theme.Provider value={{ dark, toggle }}>{children}</Theme.Provider>
    </QueryClientProvider>
  );
}
