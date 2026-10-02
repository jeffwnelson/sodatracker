import { useQuery } from "@tanstack/react-query";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { getAdminStats, listDispenses, listProducts } from "../api/client";
import { usePollInterval } from "../context/LiveUpdates";
import { ProductDot } from "../components/ProductDot";
import { useMachine } from "../api/machine";
import type { Dispense, DispenseStats } from "../api/types";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { DataTable } from "../components/DataTable";
import type { Column } from "../components/DataTable";
import { SkeletonCard } from "../components/Skeleton";
import { EmptyState } from "../components/EmptyState";
import type { ProductRanking } from "../api/types";

export function AdminDashboard() {
  const { machine, machineId, isLoading: machinesLoading } = useMachine();

  const { data: stats, isLoading: statsLoading } = useQuery<DispenseStats>({
    queryKey: ["adminStats", machineId],
    queryFn: () => getAdminStats(machineId),
    enabled: !!machineId,
    staleTime: 30_000,
  });

  if (machinesLoading) {
    return (
      <div className="space-y-6">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  if (!machine) {
    return <EmptyState message="No machine configured yet." />;
  }

  const productColumns: Column<ProductRanking>[] = [
    {
      header: "#",
      accessor: (_row, i) => (
        <span className="text-text-tertiary">{i + 1}</span>
      ),
      className: "w-8",
    },
    {
      header: "Product",
      accessor: (row) => (
        <span className="font-medium text-text-primary">
          {row.productName || row.productId}
        </span>
      ),
    },
    {
      header: "Dispenses",
      accessor: (row) => row.count,
      headerClassName: "text-right",
      className: "text-right",
    },
    {
      header: "Cost",
      accessor: (row) => `$${row.totalCost.toFixed(2)}`,
      headerClassName: "text-right",
      className: "text-right",
    },
  ];


  return (
    <div className="space-y-8">
      <PageHeader
        title="Dashboard"
      />

      {statsLoading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <SkeletonCard />
          <SkeletonCard />
        </div>
      ) : stats ? (
        <>
          {/* Summary cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Card>
              <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-2">
                Total Dispenses (30d)
              </p>
              <p className="text-3xl font-semibold tracking-tight text-text-primary">
                {stats.totalCount}
              </p>
            </Card>
            <Card>
              <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-2">
                Total Cost (30d)
              </p>
              <p className="text-3xl font-semibold tracking-tight text-text-primary">
                ${stats.totalCost.toFixed(2)}
              </p>
            </Card>
          </div>

          {/* Time series chart */}
          {stats.timeSeries.length > 0 && (
            <Card>
              <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-6">
                Dispenses Over Time
              </p>
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={stats.timeSeries}>
                  <CartesianGrid
                    strokeDasharray="0"
                    vertical={false}
                  />
                  <XAxis
                    dataKey="hour"
                    tickFormatter={(v: string) => {
                      const d = new Date(v);
                      return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours()}:00`;
                    }}
                    axisLine={false}
                    tickLine={false}
                    interval="preserveStartEnd"
                  />
                  <YAxis
                    allowDecimals={false}
                    axisLine={false}
                    tickLine={false}
                  />
                  <Tooltip
                    labelFormatter={(v) =>
                      new Date(String(v)).toLocaleString()
                    }
                  />
                  <Line
                    type="monotone"
                    dataKey="count"
                    stroke="var(--color-accent)"
                    strokeWidth={2}
                    dot={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </Card>
          )}

          {/* Top products */}
          {stats.topProducts.length > 0 && (
            <Card>
              <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
                Top Products
              </p>
              <DataTable
                columns={productColumns}
                data={stats.topProducts}
                keyFn={(p) => p.productId}
              />
            </Card>
          )}
        </>
      ) : null}

      <RecentDispenses machineId={machineId} />
    </div>
  );
}

const RECENT_DISPENSES = 10;

/** The last few dispenses, newest first. Live updates refetch it on each one. */
function RecentDispenses({ machineId }: { machineId: string }) {
  const { data: dispenses } = useQuery({
    queryKey: ["recentDispenses", machineId],
    queryFn: () => listDispenses(machineId, RECENT_DISPENSES),
    refetchInterval: usePollInterval(10_000),
  });
  const { data: products } = useQuery({ queryKey: ["products"], queryFn: listProducts });
  const byId = new Map((products ?? []).map((p) => [p.id, p]));

  const columns: Column<Dispense>[] = [
    {
      header: "When",
      accessor: (d) => (
        <span className="tabular-nums text-text-secondary">
          {new Date(d.timestamp).toLocaleString(undefined, {
            weekday: "short",
            month: "short",
            day: "numeric",
            hour: "numeric",
            minute: "2-digit",
          })}
        </span>
      ),
    },
    {
      header: "Product",
      accessor: (d) => {
        const p = byId.get(d.productId);
        return (
          <span className="inline-flex items-center gap-2 font-medium text-text-primary">
            <ProductDot color={p?.color} />
            {p?.name ?? "Unknown product"}
          </span>
        );
      },
    },
    {
      header: "Slot",
      accessor: (d) => d.slotPosition,
      className: "text-right tabular-nums",
      headerClassName: "text-right",
    },
  ];

  return (
    <Card>
      <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
        Recent Dispenses
      </p>
      {dispenses && dispenses.length > 0 ? (
        <DataTable columns={columns} data={dispenses} keyFn={(d) => d.id} />
      ) : (
        <p className="text-sm text-text-tertiary">
          {dispenses ? "No dispenses yet." : "Loading…"}
        </p>
      )}
    </Card>
  );
}
