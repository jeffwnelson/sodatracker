import { useQuery } from "@tanstack/react-query";
import { listAlertEvents, listMachines } from "./client";
import { usePollInterval } from "../context/LiveUpdates";

/**
 * The machine SodaTracker tracks. The API still models several machines, but
 * the app runs on one, so the UI uses the first and never asks which.
 */
export function useMachine() {
  const query = useQuery({ queryKey: ["machines"], queryFn: listMachines });
  const machine = query.data?.[0];
  return { machine, machineId: machine?.id ?? "", isLoading: query.isLoading };
}

/**
 * The machine's active alerts, shared by the Alerts page and the nav badge
 * (same query key, so one fetch serves both). Live updates refetch it on
 * stock events; the poll is the backstop.
 */
export function useActiveAlerts() {
  const { machineId } = useMachine();
  return useQuery({
    queryKey: ["alertEvents", machineId],
    queryFn: () => listAlertEvents(machineId, true, 50),
    enabled: !!machineId,
    refetchInterval: usePollInterval(10_000),
  });
}
