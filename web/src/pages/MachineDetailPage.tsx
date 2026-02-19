import { useParams, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  getMachineStatus,
  getMachineCost,
  listDispenses,
  listRestocks,
  getStockPredictions,
} from "../api/client";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { DataTable } from "../components/DataTable";
import type { Column } from "../components/DataTable";
import { SkeletonCard } from "../components/Skeleton";
import { EmptyState } from "../components/EmptyState";
import type { Dispense, Restock } from "../api/types";

const statusBarColor: Record<string, string> = {
  ok: "bg-status-ok",
  low: "bg-status-low",
  empty: "bg-status-empty",
};

const statusBadgeVariant: Record<string, "ok" | "low" | "empty"> = {
  ok: "ok",
  low: "low",
  empty: "empty",
};

export function MachineDetailPage() {
  const { id } = useParams<{ id: string }>();
  const machineId = id!;

  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
    refetchInterval: 10_000,
  });

  const { data: costData } = useQuery({
    queryKey: ["machineCost", machineId],
    queryFn: () => getMachineCost(machineId),
    staleTime: 30_000,
  });

  const { data: dispenses } = useQuery({
    queryKey: ["dispenses", machineId],
    queryFn: () => listDispenses(machineId, 50),
    staleTime: 10_000,
  });

  const { data: restocks } = useQuery({
    queryKey: ["restocks", machineId],
    queryFn: () => listRestocks(machineId, 20),
    staleTime: 30_000,
  });

  const { data: predictions } = useQuery({
    queryKey: ["predictions", machineId],
    queryFn: () => getStockPredictions(machineId),
    staleTime: 30_000,
  });

  const predBySlot = new Map(
    predictions?.map((p) => [p.slotPosition, p]) ?? []
  );

  if (statusLoading) {
    return (
      <div className="space-y-6">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  if (!status) {
    return <EmptyState message="Machine not found." />;
  }

  const dispenseColumns: Column<Dispense>[] = [
    {
      header: "Slot",
      accessor: (row) => (
        <span className="font-medium text-text-primary">
          #{row.slotPosition}
        </span>
      ),
      className: "w-16",
    },
    {
      header: "Source",
      accessor: (row) => row.sourceType,
    },
    {
      header: "Time",
      accessor: (row) => (
        <span className="text-text-tertiary text-xs">
          {new Date(row.timestamp).toLocaleString()}
        </span>
      ),
      headerClassName: "text-right",
      className: "text-right",
    },
  ];

  const restockColumns: Column<Restock>[] = [
    {
      header: "Date",
      accessor: (row) => new Date(row.timestamp).toLocaleString(),
    },
    {
      header: "Slots",
      accessor: (row) =>
        row.slots.map((s) => `#${s.position}: ${s.quantity}`).join(", "),
    },
    {
      header: "User",
      accessor: (row) => row.userId || "\u2014",
    },
  ];

  return (
    <div className="space-y-8">
      <PageHeader
        title={status.machine.name}
        subtitle={status.machine.location || undefined}
        actions={
          <Link to={`/admin/machines/${machineId}/restock`}>
            <Button>Restock Machine</Button>
          </Link>
        }
      />

      {/* Cost card */}
      {costData && (
        <Card>
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-2">
            Total Cost (30d)
          </p>
          <p className="text-3xl font-semibold tracking-tight text-text-primary">
            ${costData.cost.toFixed(2)}
          </p>
        </Card>
      )}

      {/* Slot stock levels — progress bar cards */}
      <div>
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
          Slot Stock Levels
        </p>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {status.slots.map((ss) => {
            const pct =
              ss.slot.capacity > 0
                ? Math.round((ss.currentStock / ss.slot.capacity) * 100)
                : 0;
            const pred = predBySlot.get(ss.slot.position);
            const daysLeft = pred?.estimatedDaysLeft ?? -1;
            return (
              <Card key={ss.slot.position} padding="sm">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-text-primary truncate">
                    {ss.product?.name || `Slot ${ss.slot.position}`}
                  </span>
                  <Badge variant={statusBadgeVariant[ss.status]}>
                    {ss.status}
                  </Badge>
                </div>
                <div className="flex items-baseline gap-1 mb-2">
                  <span className="text-xl font-semibold tracking-tight text-text-primary">
                    {ss.currentStock}
                  </span>
                  <span className="text-sm text-text-tertiary">
                    / {ss.slot.capacity}
                  </span>
                </div>
                <div className="h-1.5 w-full bg-surface-tertiary rounded-full overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-all duration-300 ease-out-custom ${statusBarColor[ss.status]}`}
                    style={{ width: `${Math.max(pct, 0)}%` }}
                  />
                </div>
                {daysLeft >= 0 && daysLeft < 3 && (
                  <div className="mt-2">
                    <Badge variant={daysLeft < 1 ? "empty" : "low"}>
                      {daysLeft < 1 ? "< 1 day left" : `~${daysLeft} days left`}
                    </Badge>
                  </div>
                )}
              </Card>
            );
          })}
        </div>
      </div>

      {/* Recent dispenses */}
      {dispenses && dispenses.length > 0 && (
        <Card>
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
            Recent Dispenses
          </p>
          <DataTable
            columns={dispenseColumns}
            data={dispenses}
            keyFn={(d) => d.id}
          />
        </Card>
      )}

      {/* Restock history */}
      {restocks && restocks.length > 0 && (
        <Card>
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
            Restock History
          </p>
          <DataTable
            columns={restockColumns}
            data={restocks}
            keyFn={(r) => r.id}
          />
        </Card>
      )}
    </div>
  );
}
