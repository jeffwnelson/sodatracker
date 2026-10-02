import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getMachineStatus } from "../api/client";
import { useActiveAlerts, useMachine } from "../api/machine";
import { useBackupStatus } from "../api/backup";
import type { AlertEvent } from "../api/types";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Badge } from "../components/Badge";
import { EmptyState } from "../components/EmptyState";
import { DataTable } from "../components/DataTable";
import { StatGrid, StatTile } from "./analytics/components";
import { formatDateTime, formatHours } from "./analytics/analytics";
import { StatusOverview } from "./StatusOverview";
import { useUptime90 } from "../api/uptime";

/**
 * Everything about whether SodaTracker is working: the status page (overall
 * state, components, 90 days of agent uptime), the active alerts that the
 * nav badge counts, and the last 90 days' outages.
 */
export function StatusPage() {
  const { machineId, isLoading } = useMachine();

  return (
    <div className="space-y-8">
      <PageHeader title="Status" />
      {!isLoading && !machineId ? (
        <EmptyState message="No machine configured." />
      ) : (
        <>
          <StatusOverview afterBanner={<ActiveAlerts machineId={machineId} />} />
          <RecentUptime machineId={machineId} />
        </>
      )}
    </div>
  );
}

function ActiveAlerts({ machineId }: { machineId: string }) {
  const { data: alertEvents } = useActiveAlerts();
  const { data: backup } = useBackupStatus();
  const { data: status } = useQuery({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
    enabled: !!machineId,
  });
  const productNames = new Map(
    (status?.slots ?? []).map((s) => [s.slot.position, s.product?.name ?? ""]),
  );
  const none = !backup?.stale && (!alertEvents || alertEvents.length === 0);

  return (
    <Card>
      <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
        Active Alerts
      </p>
      {/* Alert configs (alerts API) still exist server-side but have no UI;
          with none defined, each slot's own Low Threshold applies. */}
      {none ? (
        <p className="text-sm text-text-tertiary">No active alerts.</p>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {backup?.stale && (
            <div className="rounded-xl p-4 border-l-4 bg-status-empty-bg border-status-empty">
              <div className="flex items-center justify-between gap-3">
                <span className="text-sm font-medium text-text-primary">Backups are failing</span>
                <Badge variant="empty">Backup</Badge>
              </div>
              <p className="text-xs text-text-tertiary mt-1">
                {backup.lastError ||
                  (backup.lastPushAt
                    ? `Nothing pushed to GitHub since ${new Date(backup.lastPushAt).toLocaleString()}.`
                    : "No backup has been pushed to GitHub.")}
              </p>
              <Link to="/admin/settings" className="text-xs text-accent hover:underline mt-1 inline-block">
                Backup settings
              </Link>
            </div>
          )}
          {alertEvents?.map((ae) => (
            <StockAlert key={ae.id} alert={ae} productName={productNames.get(ae.slotPosition)} />
          ))}
        </div>
      )}
    </Card>
  );
}

function StockAlert({ alert: ae, productName }: { alert: AlertEvent; productName?: string }) {
  const out = ae.alertType === "out_of_stock";
  return (
    <div
      className={`rounded-xl p-4 border-l-4 ${
        out ? "bg-status-empty-bg border-status-empty" : "bg-status-low-bg border-status-low"
      }`}
    >
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm font-medium text-text-primary">
          {productName ? `${productName} (slot ${ae.slotPosition})` : `Slot ${ae.slotPosition}`}
        </span>
        <Badge variant={out ? "empty" : "low"}>{out ? "Out of Stock" : "Low Stock"}</Badge>
      </div>
      <p className="text-xs text-text-tertiary mt-1">
        Stock: {ae.currentStock} | Threshold: {ae.threshold}
      </p>
      <p className="text-xs text-text-muted mt-1">{new Date(ae.timestamp).toLocaleString()}</p>
    </div>
  );
}

/** The Pi agent's last 90 days: totals and the outage list. */
function RecentUptime({ machineId }: { machineId: string }) {
  const { data } = useUptime90(machineId);
  if (!data) return null;
  if (!data.trackingSince) {
    return <EmptyState message="No heartbeats from the Pi agent yet." />;
  }
  return (
    <div className="space-y-4">
      <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
        Pi agent, last 90 days
      </p>
      <StatGrid>
        <StatTile
          label="Uptime"
          value={data.uptime === null ? "—" : `${(data.uptime * 100).toFixed(2)}%`}
          sub="Of the tracked time"
        />
        <StatTile
          label="Outages"
          value={data.outages}
          sub={data.outages ? `Longest: ${formatHours(data.longestOutageHours)}` : "None"}
        />
        <StatTile
          label="Time offline"
          value={data.offlineHours ? formatHours(data.offlineHours) : "0"}
          sub="Dispenses in this time weren't counted"
        />
        <StatTile
          label="Tracked since"
          value={new Date(data.trackingSince).toLocaleDateString(undefined, {
            month: "short",
            day: "numeric",
          })}
          sub="First heartbeat from the Pi agent"
        />
      </StatGrid>
      <Card>
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
          Outages
        </p>
        {data.outageList.length === 0 ? (
          <p className="text-sm text-text-tertiary">No outages in the last 90 days.</p>
        ) : (
          <DataTable
            columns={[
              { header: "From", accessor: (r) => formatDateTime(r.from) },
              {
                header: "To",
                accessor: (r) =>
                  r.ongoing ? (
                    <span className="text-status-empty font-medium">Still offline</span>
                  ) : (
                    formatDateTime(r.to)
                  ),
              },
              {
                header: "Length",
                accessor: (r) => formatHours(r.hours),
                className: "text-right tabular-nums",
                headerClassName: "text-right",
              },
            ]}
            data={data.outageList}
            keyFn={(r) => r.from}
          />
        )}
      </Card>
    </div>
  );
}
