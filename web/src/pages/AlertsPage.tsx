import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listMachines, listAlertEvents } from "../api/client";
import type { AlertEvent } from "../api/types";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Badge } from "../components/Badge";
import { Select } from "../components/Select";
import { EmptyState } from "../components/EmptyState";
import { usePollInterval } from "../context/LiveUpdates";

export function AlertsPage() {
  const { data: machines } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
  });

  const [selectedMachine, setSelectedMachine] = useState<string>("");
  const machineId = selectedMachine || machines?.[0]?.id || "";

  const pollInterval = usePollInterval(10_000);
  const { data: alertEvents } = useQuery({
    queryKey: ["alertEvents", machineId],
    queryFn: () => listAlertEvents(machineId, true, 50),
    enabled: !!machineId,
    refetchInterval: pollInterval,
  });

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
            <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
              Active Alerts
            </p>
            {/* Alert configs (alerts API) still exist server-side but have no
                UI; with none defined, each slot's own Low Threshold applies. */}
            <p className="text-xs text-text-muted mt-1 mb-4">
              A slot alerts when its stock reaches its Low Threshold, set per
              slot on the{" "}
              <Link to="/admin/setup" className="text-accent hover:underline">
                Setup
              </Link>{" "}
              page. Stock predictions are under Analytics.
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

        </>
      )}
    </div>
  );
}
