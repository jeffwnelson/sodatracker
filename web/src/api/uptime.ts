import { useQuery } from "@tanstack/react-query";
import { getAnalytics } from "./client";
import { addDays, dayKey, parseDay, timezone } from "../pages/analytics/analytics";

/** Uptime for the whole local days [start, end], bucketed by day. */
export function useUptime(machineId: string, start: string, end: string) {
  return useQuery({
    queryKey: ["analytics", "uptime-days", machineId, start, end, timezone],
    queryFn: () =>
      getAnalytics("uptime", {
        machineId,
        from: parseDay(start).toISOString(),
        // The server caps this at now.
        to: parseDay(addDays(end, 1)).toISOString(),
        tz: timezone,
        bucket: "day",
      }),
    enabled: !!machineId,
    staleTime: 60_000,
  });
}

/** The last 90 days through today, shared by the bars and the tiles. */
export function useUptime90(machineId: string) {
  const today = dayKey(new Date());
  return useUptime(machineId, addDays(today, -89), today);
}
