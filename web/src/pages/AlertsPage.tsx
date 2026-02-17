import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listMachines,
  listAlertEvents,
  getStockPredictions,
  listAlertConfigs,
  createAlertConfig,
  updateAlertConfig,
  deleteAlertConfig,
} from "../api/client";
import type { AlertConfig } from "../api/types";

export function AlertsPage() {
  const queryClient = useQueryClient();
  const { data: machines } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
  });

  const [selectedMachine, setSelectedMachine] = useState<string>("");
  const machineId = selectedMachine || machines?.[0]?.id || "";

  // Queries
  const { data: alertEvents } = useQuery({
    queryKey: ["alertEvents", machineId],
    queryFn: () => listAlertEvents(machineId, true, 50),
    enabled: !!machineId,
    refetchInterval: 10_000,
  });

  const { data: predictions } = useQuery({
    queryKey: ["predictions", machineId],
    queryFn: () => getStockPredictions(machineId),
    enabled: !!machineId,
    staleTime: 30_000,
  });

  const { data: configs } = useQuery({
    queryKey: ["alertConfigs", machineId],
    queryFn: () => listAlertConfigs(machineId),
    enabled: !!machineId,
  });

  // Create config form state
  const [newSlotPos, setNewSlotPos] = useState(0);
  const [newThreshold, setNewThreshold] = useState(2);

  const createMutation = useMutation({
    mutationFn: () =>
      createAlertConfig(machineId, {
        slotPosition: newSlotPos,
        lowThreshold: newThreshold,
        enabled: true,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alertConfigs", machineId] });
      setNewSlotPos(0);
      setNewThreshold(2);
    },
  });

  const toggleMutation = useMutation({
    mutationFn: (config: AlertConfig) =>
      updateAlertConfig(config.id, {
        lowThreshold: config.lowThreshold,
        enabled: !config.enabled,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alertConfigs", machineId] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteAlertConfig(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alertConfigs", machineId] });
    },
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold text-gray-900">Alerts</h2>
        {machines && machines.length > 1 && (
          <select
            value={machineId}
            onChange={(e) => setSelectedMachine(e.target.value)}
            className="border rounded-lg px-3 py-2 text-sm"
          >
            {machines.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        )}
      </div>

      {!machineId && (
        <p className="text-gray-400 text-center py-12">
          No machines configured.
        </p>
      )}

      {machineId && (
        <>
          {/* Active Alerts */}
          <div className="bg-white rounded-xl shadow p-6">
            <h3 className="text-sm font-medium text-gray-700 mb-4">
              Active Alerts
            </h3>
            {alertEvents && alertEvents.length > 0 ? (
              <div className="grid gap-3 sm:grid-cols-2">
                {alertEvents.map((ae) => (
                  <div
                    key={ae.id}
                    className={`rounded-lg p-4 border-l-4 ${
                      ae.alertType === "out_of_stock"
                        ? "bg-red-50 border-red-500"
                        : "bg-yellow-50 border-yellow-500"
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium text-gray-900">
                        Slot {ae.slotPosition}
                      </span>
                      <span
                        className={`text-xs font-semibold px-2 py-0.5 rounded-full ${
                          ae.alertType === "out_of_stock"
                            ? "bg-red-100 text-red-700"
                            : "bg-yellow-100 text-yellow-700"
                        }`}
                      >
                        {ae.alertType === "out_of_stock"
                          ? "Out of Stock"
                          : "Low Stock"}
                      </span>
                    </div>
                    <p className="text-xs text-gray-500 mt-1">
                      Stock: {ae.currentStock} | Threshold: {ae.threshold}
                    </p>
                    <p className="text-xs text-gray-400 mt-1">
                      {new Date(ae.timestamp).toLocaleString()}
                    </p>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-gray-400">No active alerts.</p>
            )}
          </div>

          {/* Stock Predictions */}
          <div className="bg-white rounded-xl shadow p-6">
            <h3 className="text-sm font-medium text-gray-700 mb-4">
              Stock Predictions (14-day lookback)
            </h3>
            {predictions && predictions.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-gray-500">
                      <th className="pb-2 font-medium">Slot</th>
                      <th className="pb-2 font-medium">Product</th>
                      <th className="pb-2 font-medium">Stock</th>
                      <th className="pb-2 font-medium">Rate/day</th>
                      <th className="pb-2 font-medium">Days Left</th>
                      <th className="pb-2 font-medium">Est. Stockout</th>
                    </tr>
                  </thead>
                  <tbody>
                    {predictions.map((p) => {
                      const urgent = p.estimatedDaysLeft >= 0 && p.estimatedDaysLeft < 1;
                      const warning = p.estimatedDaysLeft >= 1 && p.estimatedDaysLeft < 3;
                      return (
                        <tr
                          key={p.slotPosition}
                          className="border-b last:border-0"
                        >
                          <td className="py-2 text-gray-700">
                            #{p.slotPosition}
                          </td>
                          <td className="py-2 text-gray-700">
                            {p.productName || "—"}
                          </td>
                          <td className="py-2 text-gray-700">
                            {p.currentStock}
                          </td>
                          <td className="py-2 text-gray-700">
                            {p.dispensesPerDay > 0
                              ? p.dispensesPerDay.toFixed(1)
                              : "—"}
                          </td>
                          <td className="py-2">
                            <span
                              className={`${
                                urgent
                                  ? "text-red-600 font-semibold"
                                  : warning
                                    ? "text-yellow-600 font-semibold"
                                    : "text-gray-700"
                              }`}
                            >
                              {p.estimatedDaysLeft < 0
                                ? "No data"
                                : p.estimatedDaysLeft === 0
                                  ? "Empty"
                                  : `${p.estimatedDaysLeft} days`}
                            </span>
                          </td>
                          <td className="py-2 text-gray-500 text-xs">
                            {p.predictedStockout
                              ? new Date(
                                  p.predictedStockout
                                ).toLocaleDateString()
                              : "—"}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ) : (
              <p className="text-sm text-gray-400">No slot data available.</p>
            )}
          </div>

          {/* Alert Configuration */}
          <div className="bg-white rounded-xl shadow p-6">
            <h3 className="text-sm font-medium text-gray-700 mb-4">
              Alert Configuration
            </h3>

            {/* Existing configs */}
            {configs && configs.length > 0 && (
              <table className="w-full text-sm mb-4">
                <thead>
                  <tr className="border-b text-left text-gray-500">
                    <th className="pb-2 font-medium">Slot</th>
                    <th className="pb-2 font-medium">Threshold</th>
                    <th className="pb-2 font-medium">Enabled</th>
                    <th className="pb-2 font-medium">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {configs.map((c) => (
                    <tr key={c.id} className="border-b last:border-0">
                      <td className="py-2 text-gray-700">
                        {c.slotPosition === 0
                          ? "Machine-wide"
                          : `#${c.slotPosition}`}
                      </td>
                      <td className="py-2 text-gray-700">
                        {c.lowThreshold === -1
                          ? "Slot default"
                          : c.lowThreshold}
                      </td>
                      <td className="py-2">
                        <button
                          onClick={() => toggleMutation.mutate(c)}
                          className={`text-xs font-semibold px-2 py-0.5 rounded-full ${
                            c.enabled
                              ? "bg-green-100 text-green-700"
                              : "bg-gray-100 text-gray-500"
                          }`}
                        >
                          {c.enabled ? "On" : "Off"}
                        </button>
                      </td>
                      <td className="py-2">
                        <button
                          onClick={() => deleteMutation.mutate(c.id)}
                          className="text-xs text-red-500 hover:text-red-700"
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}

            {/* Add new config form */}
            <div className="flex items-end gap-3 pt-2 border-t">
              <div>
                <label className="block text-xs text-gray-500 mb-1">
                  Slot (0 = machine-wide)
                </label>
                <input
                  type="number"
                  min={0}
                  value={newSlotPos}
                  onChange={(e) => setNewSlotPos(Number(e.target.value))}
                  className="border rounded px-2 py-1.5 text-sm w-24"
                />
              </div>
              <div>
                <label className="block text-xs text-gray-500 mb-1">
                  Threshold (-1 = slot default)
                </label>
                <input
                  type="number"
                  min={-1}
                  value={newThreshold}
                  onChange={(e) => setNewThreshold(Number(e.target.value))}
                  className="border rounded px-2 py-1.5 text-sm w-24"
                />
              </div>
              <button
                onClick={() => createMutation.mutate()}
                disabled={createMutation.isPending}
                className="bg-gray-900 text-white px-4 py-1.5 rounded text-sm font-medium hover:bg-gray-800 disabled:opacity-50"
              >
                Add
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
