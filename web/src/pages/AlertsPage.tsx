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
import type { AlertConfig, AlertEvent, StockPrediction } from "../api/types";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Badge } from "../components/Badge";
import { Select } from "../components/Select";
import { Input } from "../components/Input";
import { Button } from "../components/Button";
import { DataTable } from "../components/DataTable";
import type { Column } from "../components/DataTable";
import { EmptyState } from "../components/EmptyState";

export function AlertsPage() {
  const queryClient = useQueryClient();
  const { data: machines } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
  });

  const [selectedMachine, setSelectedMachine] = useState<string>("");
  const machineId = selectedMachine || machines?.[0]?.id || "";

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
      queryClient.invalidateQueries({
        queryKey: ["alertConfigs", machineId],
      });
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
      queryClient.invalidateQueries({
        queryKey: ["alertConfigs", machineId],
      });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (configId: string) => deleteAlertConfig(configId),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["alertConfigs", machineId],
      });
    },
  });

  const predictionColumns: Column<StockPrediction>[] = [
    {
      header: "Slot",
      accessor: (row) => (
        <span className="text-text-primary">#{row.slotPosition}</span>
      ),
      className: "w-16",
    },
    {
      header: "Product",
      accessor: (row) => row.productName || "\u2014",
    },
    {
      header: "Stock",
      accessor: (row) => row.currentStock,
    },
    {
      header: "Rate/day",
      accessor: (row) =>
        row.dispensesPerDay > 0 ? row.dispensesPerDay.toFixed(1) : "\u2014",
    },
    {
      header: "Days Left",
      accessor: (row) => {
        const urgent =
          row.estimatedDaysLeft >= 0 && row.estimatedDaysLeft < 1;
        const warning =
          row.estimatedDaysLeft >= 1 && row.estimatedDaysLeft < 3;
        const text =
          row.estimatedDaysLeft < 0
            ? "No data"
            : row.estimatedDaysLeft === 0
              ? "Empty"
              : `${row.estimatedDaysLeft} days`;
        return (
          <span
            className={
              urgent
                ? "text-status-empty font-semibold"
                : warning
                  ? "text-status-low font-semibold"
                  : ""
            }
          >
            {text}
          </span>
        );
      },
    },
    {
      header: "Est. Stockout",
      accessor: (row) =>
        row.predictedStockout
          ? new Date(row.predictedStockout).toLocaleDateString()
          : "\u2014",
      className: "text-xs",
    },
  ];

  const configColumns: Column<AlertConfig>[] = [
    {
      header: "Slot",
      accessor: (row) =>
        row.slotPosition === 0 ? "Machine-wide" : `#${row.slotPosition}`,
    },
    {
      header: "Threshold",
      accessor: (row) =>
        row.lowThreshold === -1 ? "Slot default" : row.lowThreshold,
    },
    {
      header: "Enabled",
      accessor: (row) => (
        <button
          onClick={() => toggleMutation.mutate(row)}
          className="transition-colors duration-150"
        >
          <Badge variant={row.enabled ? "ok" : "neutral"}>
            {row.enabled ? "On" : "Off"}
          </Badge>
        </button>
      ),
    },
    {
      header: "",
      accessor: (row) => (
        <button
          onClick={() => deleteMutation.mutate(row.id)}
          className="text-xs font-medium text-status-empty hover:text-red-700 transition-colors duration-150"
        >
          Delete
        </button>
      ),
      className: "text-right",
      headerClassName: "w-16",
    },
  ];

  return (
    <div className="space-y-8">
      <PageHeader
        title="Alerts"
        actions={
          machines && machines.length > 1 ? (
            <Select
              value={machineId}
              onChange={(e) => setSelectedMachine(e.target.value)}
              className="w-48"
            >
              {machines.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
                </option>
              ))}
            </Select>
          ) : undefined
        }
      />

      {!machineId && <EmptyState message="No machines configured." />}

      {machineId && (
        <>
          {/* Active Alerts */}
          <Card>
            <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
              Active Alerts
            </p>
            {alertEvents && alertEvents.length > 0 ? (
              <div className="grid gap-3 sm:grid-cols-2">
                {alertEvents.map((ae: AlertEvent) => (
                  <div
                    key={ae.id}
                    className={`rounded-xl p-4 border-l-4 ${
                      ae.alertType === "out_of_stock"
                        ? "bg-status-empty-bg border-status-empty"
                        : "bg-status-low-bg border-status-low"
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium text-text-primary">
                        Slot {ae.slotPosition}
                      </span>
                      <Badge
                        variant={
                          ae.alertType === "out_of_stock" ? "empty" : "low"
                        }
                      >
                        {ae.alertType === "out_of_stock"
                          ? "Out of Stock"
                          : "Low Stock"}
                      </Badge>
                    </div>
                    <p className="text-xs text-text-tertiary mt-1">
                      Stock: {ae.currentStock} | Threshold: {ae.threshold}
                    </p>
                    <p className="text-xs text-text-muted mt-1">
                      {new Date(ae.timestamp).toLocaleString()}
                    </p>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-text-tertiary">No active alerts.</p>
            )}
          </Card>

          {/* Stock Predictions */}
          <Card>
            <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
              Stock Predictions (14-day lookback)
            </p>
            {predictions && predictions.length > 0 ? (
              <DataTable
                columns={predictionColumns}
                data={predictions}
                keyFn={(p) => String(p.slotPosition)}
              />
            ) : (
              <p className="text-sm text-text-tertiary">
                No slot data available.
              </p>
            )}
          </Card>

          {/* Alert Configuration */}
          <Card>
            <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
              Alert Configuration
            </p>

            {configs && configs.length > 0 && (
              <div className="mb-6">
                <DataTable
                  columns={configColumns}
                  data={configs}
                  keyFn={(c) => c.id}
                />
              </div>
            )}

            <div className="flex items-end gap-3 pt-4 border-t border-border-primary">
              <Input
                label="Slot (0 = machine-wide)"
                type="number"
                min={0}
                value={newSlotPos}
                onChange={(e) => setNewSlotPos(Number(e.target.value))}
                className="w-36"
              />
              <Input
                label="Threshold (-1 = slot default)"
                type="number"
                min={-1}
                value={newThreshold}
                onChange={(e) => setNewThreshold(Number(e.target.value))}
                className="w-36"
              />
              <Button
                onClick={() => createMutation.mutate()}
                loading={createMutation.isPending}
              >
                Add
              </Button>
            </div>
          </Card>
        </>
      )}
    </div>
  );
}
