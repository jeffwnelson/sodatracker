import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { useQueryClient, type Query, type QueryClient } from "@tanstack/react-query";
import { streamEvents } from "../api/client";
import { useAuth } from "./AuthContext";

const LiveContext = createContext(false);

const MAX_BACKOFF_MS = 30_000;

/** How often pages poll while the event stream is up; it's only a backstop. */
const LIVE_POLL_MS = 60_000;

/** Events that arrive together (a vend and its low-stock alert) refetch once. */
const BATCH_MS = 250;

/**
 * Live analytics views refetch at most this often. Each one replays history
 * on the Pi, so a busy hour of vends would otherwise keep it aggregating.
 */
const ANALYTICS_REFRESH_MS = 60_000;

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const onAbort = () => {
      clearTimeout(t);
      resolve();
    };
    const t = setTimeout(() => {
      signal.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal.addEventListener("abort", onAbort, { once: true });
  });
}

const isAnalytics = (q: Query) => q.queryKey[0] === "analytics";
const isUptime = (q: Query) => isAnalytics(q) && q.queryKey[1] === "uptime-days";

/**
 * Turns stream events into query refreshes, each limited to what the event
 * can change:
 *
 * - Stock events (a vend, low/out of stock, a restock) refetch the machine's
 *   status, alerts and dashboards at once. Analytics whose window ends now
 *   (`meta.live`) are marked stale and refetched at most once a minute;
 *   windows in the past can't change and are left alone. Uptime doesn't
 *   depend on vends.
 * - agent_online refetches status and uptime.
 *
 * While the page is hidden, queries are only marked stale; the focus refetch
 * picks them up when it's shown again.
 */
function createRefresher(qc: QueryClient) {
  const stock = new Set<string>();
  const agent = new Set<string>();
  let batchTimer: ReturnType<typeof setTimeout> | undefined;
  let analyticsTimer: ReturnType<typeof setTimeout> | undefined;
  let lastAnalytics = 0;
  const pendingAnalytics = new Set<string>();

  const refetchType = () => (document.visibilityState === "hidden" ? "none" : "active");
  const forMachines = (ids: string[]) => (q: Query) => ids.some((id) => q.queryKey.includes(id));

  function markAnalytics(match: (q: Query) => boolean) {
    const pred = (q: Query) => isAnalytics(q) && q.meta?.live === true && match(q);
    for (const q of qc.getQueryCache().findAll({ predicate: pred })) pendingAnalytics.add(q.queryHash);
    void qc.invalidateQueries({ predicate: pred, refetchType: "none" });
    analyticsTimer ??= setTimeout(
      flushAnalytics,
      Math.max(0, lastAnalytics + ANALYTICS_REFRESH_MS - Date.now()),
    );
  }

  function flushAnalytics() {
    analyticsTimer = undefined;
    lastAnalytics = Date.now();
    const hashes = new Set(pendingAnalytics);
    pendingAnalytics.clear();
    if (refetchType() === "none") return;
    void qc.refetchQueries({ predicate: (q) => hashes.has(q.queryHash), type: "active" });
  }

  function flush() {
    batchTimer = undefined;
    if (stock.size) {
      const mine = forMachines([...stock]);
      stock.clear();
      void qc.invalidateQueries({ predicate: (q) => !isAnalytics(q) && mine(q), refetchType: refetchType() });
      markAnalytics((q) => !isUptime(q) && mine(q));
    }
    if (agent.size) {
      const mine = forMachines([...agent]);
      agent.clear();
      void qc.invalidateQueries({
        predicate: (q) => (q.queryKey[0] === "machineStatus" || isUptime(q)) && mine(q),
        refetchType: refetchType(),
      });
    }
  }

  return {
    event(type: string, machineId: string) {
      (type === "agent_online" ? agent : stock).add(machineId);
      batchTimer ??= setTimeout(flush, BATCH_MS);
    },
    /**
     * After a reconnect, anything may have changed. cancelRefetch: false
     * leaves alone the fetches the page's own focus refetch has just
     * started, so a resumed phone app doesn't ask the Pi for everything
     * twice.
     */
    catchUp() {
      void qc.invalidateQueries(
        { predicate: (q) => !isAnalytics(q), refetchType: refetchType() },
        { cancelRefetch: false },
      );
      markAnalytics(() => true);
    },
    dispose() {
      clearTimeout(batchTimer);
      clearTimeout(analyticsTimer);
    },
  };
}

/**
 * Keeps one event stream open while logged in, refreshing the queries each
 * event affects (see createRefresher), so pages update within a second of a
 * vend without knowing about the stream. Logged-out visitors don't get a
 * stream and keep polling.
 */
export function LiveUpdatesProvider({ children }: { children: ReactNode }) {
  const { token } = useAuth();
  const queryClient = useQueryClient();
  const [live, setLive] = useState(false);

  useEffect(() => {
    if (!token) return;
    const ctrl = new AbortController();
    const refresher = createRefresher(queryClient);
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
              if (connectedBefore) refresher.catchUp();
              connectedBefore = true;
            },
            onEvent: (evt) => {
              if (evt.type.startsWith("backup_")) {
                queryClient.invalidateQueries({ queryKey: ["backup"] });
                return;
              }
              const id = evt.data?.machineId;
              if (id) refresher.event(evt.type, id);
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
      refresher.dispose();
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
