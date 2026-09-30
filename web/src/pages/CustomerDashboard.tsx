import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listMachines, getMachineStatus } from "../api/client";
import { useAuth } from "../context/AuthContext";
import type { MachineStatus } from "../api/types";
import { StockGrid } from "../components/StockGrid";
import { Card } from "../components/Card";
import { SkeletonCard } from "../components/Skeleton";
import { EmptyState } from "../components/EmptyState";
import { AppFooter } from "../components/AppFooter";

function MachineCard({ machineId }: { machineId: string }) {
  const { data, isLoading, error } = useQuery<MachineStatus>({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
    refetchInterval: 10_000,
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
    <div className="min-h-screen flex flex-col bg-surface-secondary">
      <header className="bg-surface-primary border-b border-border-primary">
        <div className="max-w-5xl mx-auto px-6 py-5 flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold tracking-tight text-text-primary">
              SodaTracker
            </h1>
            <p className="text-sm text-text-tertiary mt-0.5">
              Live inventory status
            </p>
          </div>
          <Link
            to={isAuthenticated ? "/admin" : "/admin/login"}
            className="inline-flex items-center rounded-xl border border-border-primary bg-surface-primary px-4 py-2 text-sm font-medium text-text-primary transition-colors duration-150 ease-out-custom hover:bg-surface-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
          >
            {isAuthenticated ? "Admin" : "Log in"}
          </Link>
        </div>
      </header>

      <main className="flex-1 w-full max-w-5xl mx-auto px-6 py-10 space-y-6">
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
      </main>
      <AppFooter width="max-w-5xl" />
    </div>
  );
}
