import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { getMachineStatus } from "../api/client";
import { useMachine } from "../api/machine";
import { useAuth } from "../context/AuthContext";
import type { MachineStatus } from "../api/types";
import { StockGrid } from "../components/StockGrid";
import { Card } from "../components/Card";
import { SkeletonCard } from "../components/Skeleton";
import { EmptyState } from "../components/EmptyState";
import { AppShell } from "../components/AppShell";
import { headerItemClass } from "../components/headerItemClass";
import { usePollInterval } from "../context/LiveUpdates";

function MachineCard({ machineId }: { machineId: string }) {
  const { data, isLoading, error } = useQuery<MachineStatus>({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
    refetchInterval: usePollInterval(10_000),
  });

  if (isLoading) return <SkeletonCard />;

  if (error || !data) {
    return (
      <Card>
        <p className="text-sm text-status-empty">
          Failed to load machine status
        </p>
      </Card>
    );
  }

  return (
    <Card>
      <StockGrid slots={data.slots} />
    </Card>
  );
}

export function CustomerDashboard() {
  const { isAuthenticated } = useAuth();
  const { machine, isLoading } = useMachine();

  return (
    <AppShell
      actions={
        <Link
          to={isAuthenticated ? "/admin" : "/admin/login"}
          className={headerItemClass()}
        >
          {isAuthenticated ? "Admin" : "Log in"}
        </Link>
      }
    >
      {isLoading ? (
        <SkeletonCard />
      ) : machine ? (
        <MachineCard machineId={machine.id} />
      ) : (
        <EmptyState message="No machine configured yet." />
      )}
    </AppShell>
  );
}
