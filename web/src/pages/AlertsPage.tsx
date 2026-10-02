import { Link } from "react-router-dom";
import { useActiveAlerts, useMachine } from "../api/machine";
import { useBackupStatus } from "../api/backup";
import type { AlertEvent } from "../api/types";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Badge } from "../components/Badge";
import { EmptyState } from "../components/EmptyState";

export function AlertsPage() {
  const { machineId } = useMachine();
  const { data: alertEvents } = useActiveAlerts();
  const { data: backup } = useBackupStatus();

  return (
    <div className="space-y-8">
      <PageHeader title="Alerts" />

      {backup?.stale && (
        <div className="rounded-xl p-4 border-l-4 bg-status-empty-bg border-status-empty">
          <div className="flex items-center justify-between gap-3">
            <span className="text-sm font-medium text-text-primary">
              Backups are failing
            </span>
            <Badge variant="empty">Backup</Badge>
          </div>
          <p className="text-xs text-text-tertiary mt-1">
            {backup.lastError ||
              (backup.lastPushAt
                ? `Nothing pushed to GitHub since ${new Date(backup.lastPushAt).toLocaleString()}.`
                : "No backup has been pushed to GitHub.")}
          </p>
          <Link
            to="/admin/settings"
            className="text-xs text-accent hover:underline mt-1 inline-block"
          >
            Backup settings
          </Link>
        </div>
      )}

      {!machineId && <EmptyState message="No machine configured." />}

      {machineId && (
        <>
          {/* Active Alerts */}
          <Card>
            <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
              Active Alerts
            </p>
            {/* Alert configs (alerts API) still exist server-side but have no
                UI; with none defined, each slot's own Low Threshold applies. */}
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
