import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import { streamEvents } from "../api/client";
import { useAuth } from "./AuthContext";

const LiveContext = createContext(false);

const MAX_BACKOFF_MS = 30_000;

/** How often pages poll while the event stream is up; it's only a backstop. */
const LIVE_POLL_MS = 60_000;

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const t = setTimeout(resolve, ms);
    signal.addEventListener("abort", () => {
      clearTimeout(t);
      resolve();
    });
  });
}

/**
 * Keeps one event stream open while logged in. Each event names a machine,
 * and every query whose key contains that machine's ID refetches, so pages
 * update within a second of a vend without knowing about the stream.
 * Logged-out visitors don't get a stream and keep polling.
 */
export function LiveUpdatesProvider({ children }: { children: ReactNode }) {
  const { token } = useAuth();
  const queryClient = useQueryClient();
  const [live, setLive] = useState(false);

  useEffect(() => {
    if (!token) return;
    const ctrl = new AbortController();
    (async () => {
      let failures = 0;
      let connectedBefore = false;
      while (!ctrl.signal.aborted) {
        try {
          await streamEvents({
            signal: ctrl.signal,
            onOpen: () => {
              failures = 0;
              setLive(true);
              // Catch up on anything that happened while disconnected.
              if (connectedBefore) queryClient.invalidateQueries();
              connectedBefore = true;
            },
            onEvent: (evt) => {
              if (evt.type.startsWith("backup_")) {
                queryClient.invalidateQueries({ queryKey: ["backup"] });
                return;
              }
              const id = evt.data?.machineId;
              if (id) {
                queryClient.invalidateQueries({
                  predicate: (q) => q.queryKey.includes(id),
                });
              }
            },
          });
        } catch {
          // reconnect below
        }
        setLive(false);
        failures++;
        await sleep(
          Math.min(MAX_BACKOFF_MS, 1000 * 2 ** (failures - 1)),
          ctrl.signal,
        );
      }
    })();
    return () => {
      ctrl.abort();
      setLive(false);
    };
  }, [token, queryClient]);

  return <LiveContext.Provider value={live}>{children}</LiveContext.Provider>;
}

/** True while the event stream is connected. */
export function useLive(): boolean {
  return useContext(LiveContext);
}

/** A query's refetchInterval: `ms` without the stream, a slow backstop with it. */
export function usePollInterval(ms: number): number {
  return useLive() ? Math.max(ms, LIVE_POLL_MS) : ms;
}
