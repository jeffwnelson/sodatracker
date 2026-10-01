import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listMachines, getMachineStatus } from "../api/client";
import { useAuth } from "../context/AuthContext";
import type { MachineStatus } from "../api/types";
import { StockGrid } from "../components/StockGrid";
import { Card } from "../components/Card";
import { SkeletonCard } from "../components/Skeleton";
import { EmptyState } from "../components/EmptyState";
import { AppShell } from "../components/AppShell";
import { headerItemClass } from "../components/headerItemClass";
import { PageHeader } from "../components/PageHeader";
import { AgentStatusBadge } from "../components/AgentStatusBadge";
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
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 className="text-lg font-semibold tracking-tight text-text-primary">
            {data.machine.name}
          </h2>
          {data.machine.location && (
            <p className="text-sm text-text-tertiary mt-0.5">
              {data.machine.location}
            </p>
          )}
        </div>
        <AgentStatusBadge agent={data.agent} />
      </div>
      <StockGrid slots={data.slots} />
    </Card>
  );
}

export function CustomerDashboard() {
  const { isAuthenticated } = useAuth();
  const {
    data: machines,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
    refetchInterval: 30_000,
  });

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
      <div className="space-y-6">
        <PageHeader title="Inventory" subtitle="Live stock status" />

        {isLoading && (
          <div className="space-y-6">
            <SkeletonCard />
            <SkeletonCard />
          </div>
        )}

        {error && (
          <Card>
            <p className="text-sm text-status-empty">
              Failed to load machines. Is the API running?
            </p>
          </Card>
        )}

        {machines?.length === 0 && (
          <EmptyState message="No machines configured yet." />
        )}

        {machines?.map((m) => <MachineCard key={m.id} machineId={m.id} />)}
      </div>
    </AppShell>
  );
}
