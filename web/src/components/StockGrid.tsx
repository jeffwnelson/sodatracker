import type { SlotStatus } from "../api/types";
import { Badge } from "./Badge";

const statusBarColor: Record<string, string> = {
  ok: "bg-status-ok",
  low: "bg-status-low",
  empty: "bg-status-empty",
};

const statusBadgeVariant: Record<string, "ok" | "low" | "empty"> = {
  ok: "ok",
  low: "low",
  empty: "empty",
};

export function StockGrid({ slots }: { slots: SlotStatus[] }) {
  if (slots.length === 0) {
    return (
      <p className="text-sm text-text-tertiary">No slots configured</p>
    );
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {slots.map((ss) => {
        const pct =
          ss.slot.capacity > 0
            ? Math.round((ss.currentStock / ss.slot.capacity) * 100)
            : 0;
        return (
          <div
            key={ss.slot.position}
            className="rounded-xl border border-border-primary bg-surface-primary p-4 transition-colors duration-150 hover:border-border-secondary"
          >
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-text-primary truncate">
                {ss.product?.name || `Slot ${ss.slot.position}`}
              </span>
              <Badge variant={statusBadgeVariant[ss.status]}>
                {ss.status}
              </Badge>
            </div>
            <div className="flex items-baseline gap-1 mb-2">
              <span className="text-2xl font-semibold tracking-tight text-text-primary">
                {ss.currentStock}
              </span>
              <span className="text-sm text-text-tertiary">
                / {ss.slot.capacity}
              </span>
            </div>
            <div className="h-1.5 w-full bg-surface-tertiary rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all duration-300 ease-out-custom ${statusBarColor[ss.status]}`}
                style={{ width: `${Math.max(pct, 0)}%` }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
