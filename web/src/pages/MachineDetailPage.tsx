import { useParams, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  RadialBarChart,
  RadialBar,
  ResponsiveContainer,
} from "recharts";
import {
  getMachineStatus,
  getMachineCost,
  listDispenses,
  listRestocks,
  getStockPredictions,
} from "../api/client";

const statusColor: Record<string, string> = {
  ok: "#22c55e",
  low: "#eab308",
  empty: "#ef4444",
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

  // Build prediction lookup by slot position
  const predBySlot = new Map(
    predictions?.map((p) => [p.slotPosition, p]) ?? []
  );

  if (statusLoading) {
    return <div className="text-center py-12 text-gray-500">Loading...</div>;
  }

  if (!status) {
    return (
      <div className="text-center py-12 text-gray-400">Machine not found.</div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">
            {status.machine.name}
          </h2>
          {status.machine.location && (
            <p className="text-sm text-gray-500">{status.machine.location}</p>
          )}
        </div>
        <Link
          to={`/admin/machines/${machineId}/restock`}
          className="bg-gray-900 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-gray-800"
        >
          Restock Machine
        </Link>
      </div>

      {/* Cost card */}
      {costData && (
        <div className="bg-white rounded-xl shadow p-6">
          <p className="text-sm text-gray-500 mb-1">Total Cost (30d)</p>
          <p className="text-3xl font-bold text-gray-900">
            ${costData.cost.toFixed(2)}
          </p>
        </div>
      )}

      {/* Stock gauges */}
      <div className="bg-white rounded-xl shadow p-6">
        <h3 className="text-sm font-medium text-gray-700 mb-4">
          Slot Stock Levels
        </h3>
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-4">
          {status.slots.map((ss) => {
            const pct =
              ss.slot.capacity > 0
                ? (ss.currentStock / ss.slot.capacity) * 100
                : 0;
            const gaugeData = [
              { value: pct, fill: statusColor[ss.status] || "#6b7280" },
            ];
            const pred = predBySlot.get(ss.slot.position);
            const daysLeft = pred?.estimatedDaysLeft ?? -1;
            return (
              <div key={ss.slot.position} className="text-center">
                <ResponsiveContainer width="100%" height={100}>
                  <RadialBarChart
                    cx="50%"
                    cy="50%"
                    innerRadius="60%"
                    outerRadius="90%"
                    startAngle={180}
                    endAngle={0}
                    barSize={8}
                    data={gaugeData}
                  >
                    <RadialBar
                      dataKey="value"
                      background={{ fill: "#f3f4f6" }}
                      cornerRadius={4}
                    />
                  </RadialBarChart>
                </ResponsiveContainer>
                <p className="text-xs font-medium text-gray-900 -mt-4">
                  {ss.currentStock}/{ss.slot.capacity}
                </p>
                <p className="text-xs text-gray-500 truncate mt-1">
                  {ss.product?.name || `Slot ${ss.slot.position}`}
                </p>
                {daysLeft >= 0 && daysLeft < 3 && (
                  <span
                    className={`inline-block mt-1 text-[10px] font-semibold px-1.5 py-0.5 rounded-full ${
                      daysLeft < 1
                        ? "bg-red-100 text-red-700"
                        : "bg-orange-100 text-orange-700"
                    }`}
                  >
                    {daysLeft < 1
                      ? "< 1 day"
                      : `~${daysLeft} days`}
                  </span>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Dispense timeline */}
      {dispenses && dispenses.length > 0 && (
        <div className="bg-white rounded-xl shadow p-6">
          <h3 className="text-sm font-medium text-gray-700 mb-4">
            Recent Dispenses
          </h3>
          <div className="max-h-64 overflow-y-auto space-y-2">
            {dispenses.map((d) => (
              <div
                key={d.id}
                className="flex items-center justify-between text-sm border-b pb-2 last:border-0"
              >
                <div>
                  <span className="text-gray-900 font-medium">
                    Slot {d.slotPosition}
                  </span>
                  <span className="text-gray-400 ml-2">{d.sourceType}</span>
                </div>
                <span className="text-gray-500 text-xs">
                  {new Date(d.timestamp).toLocaleString()}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Restock history */}
      {restocks && restocks.length > 0 && (
        <div className="bg-white rounded-xl shadow p-6">
          <h3 className="text-sm font-medium text-gray-700 mb-4">
            Restock History
          </h3>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-gray-500">
                <th className="pb-2 font-medium">Date</th>
                <th className="pb-2 font-medium">Slots</th>
                <th className="pb-2 font-medium">User</th>
              </tr>
            </thead>
            <tbody>
              {restocks.map((r) => (
                <tr key={r.id} className="border-b last:border-0">
                  <td className="py-2 text-gray-700">
                    {new Date(r.timestamp).toLocaleString()}
                  </td>
                  <td className="py-2 text-gray-700">
                    {r.slots
                      .map((s) => `#${s.position}: ${s.quantity}`)
                      .join(", ")}
                  </td>
                  <td className="py-2 text-gray-500">{r.userId || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
