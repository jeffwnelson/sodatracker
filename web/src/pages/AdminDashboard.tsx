import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
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

export function AdminDashboard() {
  const { data: machines, isLoading: machinesLoading } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
  });

  const [selectedMachineId, setSelectedMachineId] = useState<string>("");

  // Auto-select first machine
  const machineId = selectedMachineId || machines?.[0]?.id || "";

  const { data: stats, isLoading: statsLoading } = useQuery<DispenseStats>({
    queryKey: ["adminStats", machineId],
    queryFn: () => getAdminStats(machineId),
    enabled: !!machineId,
    staleTime: 30_000,
  });

  if (machinesLoading) {
    return <div className="text-center py-12 text-gray-500">Loading...</div>;
  }

  if (!machines || machines.length === 0) {
    return (
      <div className="text-center py-12 text-gray-400">
        No machines configured yet.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Machine selector */}
      <div className="flex items-center gap-4">
        <label className="text-sm font-medium text-gray-700">Machine:</label>
        <select
          value={machineId}
          onChange={(e) => setSelectedMachineId(e.target.value)}
          className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
        >
          {machines.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name}
              {m.location ? ` — ${m.location}` : ""}
            </option>
          ))}
        </select>
      </div>

      {statsLoading ? (
        <div className="text-center py-8 text-gray-500">
          Loading stats...
        </div>
      ) : stats ? (
        <>
          {/* Summary cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="bg-white rounded-xl shadow p-6">
              <p className="text-sm text-gray-500 mb-1">
                Total Dispenses (30d)
              </p>
              <p className="text-3xl font-bold text-gray-900">
                {stats.totalCount}
              </p>
            </div>
            <div className="bg-white rounded-xl shadow p-6">
              <p className="text-sm text-gray-500 mb-1">Total Cost (30d)</p>
              <p className="text-3xl font-bold text-gray-900">
                ${stats.totalCost.toFixed(2)}
              </p>
            </div>
          </div>

          {/* Time series chart */}
          {stats.timeSeries.length > 0 && (
            <div className="bg-white rounded-xl shadow p-6">
              <h3 className="text-sm font-medium text-gray-700 mb-4">
                Dispenses Over Time
              </h3>
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={stats.timeSeries}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis
                    dataKey="hour"
                    tickFormatter={(v: string) => {
                      const d = new Date(v);
                      return `${(d.getMonth() + 1)}/${d.getDate()} ${d.getHours()}:00`;
                    }}
                    tick={{ fontSize: 11 }}
                    interval="preserveStartEnd"
                  />
                  <YAxis allowDecimals={false} tick={{ fontSize: 11 }} />
                  <Tooltip
                    labelFormatter={(v) => new Date(String(v)).toLocaleString()}
                  />
                  <Line
                    type="monotone"
                    dataKey="count"
                    stroke="#111827"
                    strokeWidth={2}
                    dot={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          )}

          {/* Top products */}
          {stats.topProducts.length > 0 && (
            <div className="bg-white rounded-xl shadow p-6">
              <h3 className="text-sm font-medium text-gray-700 mb-4">
                Top Products
              </h3>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-gray-500">
                    <th className="pb-2 font-medium">#</th>
                    <th className="pb-2 font-medium">Product</th>
                    <th className="pb-2 font-medium text-right">Dispenses</th>
                    <th className="pb-2 font-medium text-right">Cost</th>
                  </tr>
                </thead>
                <tbody>
                  {stats.topProducts.map((p, i) => (
                    <tr key={p.productId} className="border-b last:border-0">
                      <td className="py-2 text-gray-400">{i + 1}</td>
                      <td className="py-2 font-medium text-gray-900">
                        {p.productName || p.productId}
                      </td>
                      <td className="py-2 text-right text-gray-700">
                        {p.count}
                      </td>
                      <td className="py-2 text-right text-gray-700">
                        ${p.totalCost.toFixed(2)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      ) : null}

      {/* Machine list */}
      <div className="bg-white rounded-xl shadow p-6">
        <h3 className="text-sm font-medium text-gray-700 mb-4">Machines</h3>
        <div className="space-y-2">
          {machines.map((m) => (
            <Link
              key={m.id}
              to={`/admin/machines/${m.id}`}
              className="block border rounded-lg p-3 hover:bg-gray-50 transition"
            >
              <div className="font-medium text-gray-900">{m.name}</div>
              {m.location && (
                <div className="text-sm text-gray-500">{m.location}</div>
              )}
              <div className="text-xs text-gray-400">
                {m.slotCount} slots
              </div>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
