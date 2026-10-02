import { useQuery } from "@tanstack/react-query";
import { getBackupStatus } from "./client";

/**
 * Backup status, shared by Settings, the Alerts page and the nav badge (same
 * query key, so one fetch serves all three). Backups change at most hourly,
 * so a slow poll is plenty.
 */
export function useBackupStatus() {
  return useQuery({
    queryKey: ["backup"],
    queryFn: getBackupStatus,
    refetchInterval: 60_000,
  });
}
