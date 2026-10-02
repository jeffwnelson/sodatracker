import { useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getAlertSettings, getMachineStatus, updateAlertSettings } from "../api/client";
import { useActiveAlerts, useMachine } from "../api/machine";
import { useBackupStatus } from "../api/backup";
import type { AlertEvent, QuietSettings } from "../api/types";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Badge } from "../components/Badge";
import { EmptyState } from "../components/EmptyState";
import { Input } from "../components/Input";
import { Button } from "../components/Button";

export function AlertsPage() {
  const { machineId } = useMachine();
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
                  <AlertCard key={ae.id} alert={ae} productName={productNames.get(ae.slotPosition)} />
                ))}
              </div>
            ) : (
              <p className="text-sm text-text-tertiary">No active alerts.</p>
            )}
          </Card>

          <QuietSettingsCard />
        </>
      )}
    </div>
  );
}

function AlertCard({ alert: ae, productName }: { alert: AlertEvent; productName?: string }) {
  const slot = productName ? `${productName} (slot ${ae.slotPosition})` : `Slot ${ae.slotPosition}`;
  const view = {
    out_of_stock: { title: slot, badge: "Out of Stock", tone: "empty" as const,
      detail: `Stock: ${ae.currentStock} | Threshold: ${ae.threshold}` },
    low_stock: { title: slot, badge: "Low Stock", tone: "low" as const,
      detail: `Stock: ${ae.currentStock} | Threshold: ${ae.threshold}` },
    quiet_slot: { title: slot, badge: "Not Registering", tone: "low" as const,
      detail: `The other slots dispensed ${ae.currentStock} cans since this one last did. If it is being taken, check its contact.` },
    quiet_machine: { title: "Whole machine", badge: "Not Registering", tone: "low" as const,
      detail: `No slot has registered a dispense in ${ae.currentStock} hours while the Pi agent is online. Check the shared ground and the cable to the Pi.` },
  }[ae.alertType];
  return (
    <div
      className={`rounded-xl p-4 border-l-4 ${
        view.tone === "empty"
          ? "bg-status-empty-bg border-status-empty"
          : "bg-status-low-bg border-status-low"
      }`}
    >
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm font-medium text-text-primary">{view.title}</span>
        <Badge variant={view.tone}>{view.badge}</Badge>
      </div>
      <p className="text-xs text-text-tertiary mt-1">{view.detail}</p>
      <p className="text-xs text-text-muted mt-1">
        {new Date(ae.timestamp).toLocaleString()}
      </p>
    </div>
  );
}

function QuietSettingsCard() {
  const { data } = useQuery({ queryKey: ["alertSettings"], queryFn: getAlertSettings });
  return (
    <Card>
      <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-1">
        Not-Registering Alerts
      </p>
      <p className="text-xs text-text-tertiary mb-4">
        Catch cans taken but not counted: a slot whose contact stopped closing, or
        the whole harness coming loose. Set either to 0 to turn it off.
      </p>
      {data && (
        <QuietSettingsForm
          key={`${data.quietSlotCans}|${data.quietMachineHours}|${data.quietMachineWeekdaysOnly}|${data.timezone}`}
          settings={data}
        />
      )}
    </Card>
  );
}

function QuietSettingsForm({ settings }: { settings: QuietSettings }) {
  const queryClient = useQueryClient();
  const [slotCans, setSlotCans] = useState(String(settings.quietSlotCans));
  const [machineHours, setMachineHours] = useState(String(settings.quietMachineHours));
  const [weekdaysOnly, setWeekdaysOnly] = useState(settings.quietMachineWeekdaysOnly);
  const [error, setError] = useState("");

  const mutation = useMutation({
    mutationFn: () =>
      updateAlertSettings({
        quietSlotCans: Number(slotCans),
        quietMachineHours: Number(machineHours),
        quietMachineWeekdaysOnly: weekdaysOnly,
        // Weekends are the browser's: whoever sets this is at the machine.
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      }),
    onSuccess: (q) => {
      setError("");
      queryClient.setQueryData(["alertSettings"], q);
      // Saving re-evaluates, which may raise or clear alerts.
      queryClient.invalidateQueries({ queryKey: ["alertEvents"] });
    },
    onError: (err: Error) => setError(err.message),
  });

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        mutation.mutate();
      }}
    >
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <Input
            label="Slot not registering after"
            type="number"
            min={0}
            max={1000}
            value={slotCans}
            onChange={(e) => setSlotCans(e.target.value)}
          />
          <p className="mt-1 text-xs text-text-tertiary">
            cans from the other slots with none from it. Sold-out slots are skipped.
          </p>
        </div>
        <div>
          <Input
            label="Machine not registering after"
            type="number"
            min={0}
            max={720}
            value={machineHours}
            onChange={(e) => setMachineHours(e.target.value)}
          />
          <p className="mt-1 text-xs text-text-tertiary">
            hours with no dispense from any slot while the Pi agent is online.
          </p>
          <label className="mt-2 flex items-center gap-2 text-sm text-text-secondary">
            <input
              type="checkbox"
              checked={weekdaysOnly}
              onChange={(e) => setWeekdaysOnly(e.target.checked)}
            />
            Count only weekday hours ({settings.timezone})
          </label>
        </div>
      </div>
      {error && <p className="text-sm text-status-empty">{error}</p>}
      <div className="flex items-center justify-end gap-3">
        {mutation.isSuccess && <span className="text-sm text-status-ok">Saved.</span>}
        <Button type="submit" loading={mutation.isPending}>
          Save
        </Button>
      </div>
    </form>
  );
}
