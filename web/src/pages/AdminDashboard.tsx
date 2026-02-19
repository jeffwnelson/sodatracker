import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { listMachines, getAdminStats } from "../api/client";
import type { DispenseStats } from "../api/types";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Select } from "../components/Select";
import { DataTable } from "../components/DataTable";
import type { Column } from "../components/DataTable";
import { SkeletonCard } from "../components/Skeleton";
import { EmptyState } from "../components/EmptyState";
import type { ProductRanking, Machine } from "../api/types";

export function AdminDashboard() {
  const navigate = useNavigate();
  const { data: machines, isLoading: machinesLoading } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
  });

  const [selectedMachineId, setSelectedMachineId] = useState<string>("");
  const machineId = selectedMachineId || machines?.[0]?.id || "";

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

  if (!machines || machines.length === 0) {
    return <EmptyState message="No machines configured yet." />;
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

  const machineColumns: Column<Machine>[] = [
    {
      header: "Name",
      accessor: (row) => (
        <span className="font-medium text-text-primary">
          {row.name}
        </span>
      ),
    },
    {
      header: "Location",
      accessor: (row) => row.location || "\u2014",
    },
    {
      header: "Slots",
      accessor: (row) => row.slotCount,
      headerClassName: "text-right",
      className: "text-right",
    },
  ];

  return (
    <div className="space-y-8">
      <PageHeader
        title="Dashboard"
        actions={
          <Select
            value={machineId}
            onChange={(e) => setSelectedMachineId(e.target.value)}
            className="w-52"
          >
            {machines.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
                {m.location ? ` \u2014 ${m.location}` : ""}
              </option>
            ))}
          </Select>
        }
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

      {/* Machine list */}
      <Card>
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
          Machines
        </p>
        <DataTable
          columns={machineColumns}
          data={machines}
          keyFn={(m) => m.id}
          emptyMessage="No machines configured."
          onRowClick={(m) => navigate(`/admin/machines/${m.id}`)}
        />
      </Card>
    </div>
  );
}
