import { useState } from "react";
import type { ReactNode } from "react";
import type { UseQueryResult } from "@tanstack/react-query";
import { Card } from "../../components/Card";
import { SkeletonCard } from "../../components/Skeleton";

// --- Components ------------------------------------------------------------

/**
 * Loading, error and refetch states for a dashboard. A refetch keeps the old
 * render at reduced opacity rather than swapping in skeletons.
 */
export function Loaded<T>({
  query,
  children,
}: {
  query: UseQueryResult<T>;
  children: (data: T) => ReactNode;
}) {
  if (query.isPending) {
    return (
      <div className="space-y-6">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <SkeletonCard />
          <SkeletonCard />
          <SkeletonCard />
          <SkeletonCard />
        </div>
        <SkeletonCard />
      </div>
    );
  }
  if (query.isError) {
    return (
      <p className="text-sm text-status-empty">
        Could not load analytics: {query.error.message}
      </p>
    );
  }
  return (
    <div
      className={`space-y-6 transition-opacity duration-150 ${query.isPlaceholderData ? "opacity-60" : ""}`}
    >
      {children(query.data)}
    </div>
  );
}

/**
 * Change against the previous window of equal length. Neutral ink: more
 * dispenses is not inherently good or bad.
 */
export function Delta({
  current,
  previous,
  format = (n) => String(n),
}: {
  current: number;
  previous: number | null;
  format?: (n: number) => string;
}) {
  if (previous === null) {
    return <span>No earlier data to compare</span>;
  }
  const diff = current - previous;
  if (Math.abs(diff) < 1e-9) return <span>Same as previous period</span>;
  const pct = previous > 0 ? ` (${Math.round((diff / previous) * 100)}%)` : "";
  return (
    <span>
      {diff > 0 ? "▲" : "▼"} {format(Math.abs(diff))}
      {pct} vs previous period
    </span>
  );
}

export function StatTile({
  label,
  value,
  sub,
}: {
  label: string;
  value: ReactNode;
  sub?: ReactNode;
}) {
  return (
    <Card>
      <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-2">
        {label}
      </p>
      <p className="text-3xl font-semibold tracking-tight text-text-primary">
        {value}
      </p>
      {sub && <p className="mt-2 text-xs text-text-secondary">{sub}</p>}
    </Card>
  );
}

export function StatGrid({ children }: { children: ReactNode }) {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      {children}
    </div>
  );
}

/**
 * A chart with a table-view twin. The table is the accessible equivalent and
 * the fallback for the low-contrast series colors.
 */
export function ChartCard({
  title,
  subtitle,
  legend,
  table,
  children,
}: {
  title: string;
  subtitle?: string;
  legend?: ReactNode;
  table?: ReactNode;
  children: ReactNode;
}) {
  const [view, setView] = useState<"chart" | "table">("chart");
  return (
    <Card>
      <div className="flex flex-wrap items-start justify-between gap-3 mb-4">
        <div>
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
            {title}
          </p>
          {subtitle && (
            <p className="mt-1 text-sm text-text-secondary">{subtitle}</p>
          )}
        </div>
        {table && (
          <div
            className="inline-flex rounded-lg border border-border-primary p-0.5 text-xs"
            role="group"
            aria-label={`${title} view`}
          >
            {(["chart", "table"] as const).map((v) => (
              <button
                key={v}
                type="button"
                onClick={() => setView(v)}
                aria-pressed={view === v}
                className={`px-2.5 py-1 rounded-md font-medium capitalize transition-colors duration-150 ${
                  view === v
                    ? "bg-accent-soft text-accent"
                    : "text-text-tertiary hover:text-text-primary"
                }`}
              >
                {v}
              </button>
            ))}
          </div>
        )}
      </div>
      {view === "chart" || !table ? (
        <>
          {legend && <div className="mb-4">{legend}</div>}
          {children}
        </>
      ) : (
        table
      )}
    </Card>
  );
}

export function Legend({
  items,
}: {
  items: { key: string; label: string; color: string; line?: boolean }[];
}) {
  return (
    <ul className="flex flex-wrap gap-x-4 gap-y-1.5 text-xs text-text-secondary">
      {items.map((it) => (
        <li key={it.key} className="flex items-center gap-1.5">
          <span
            aria-hidden
            className={it.line ? "inline-block w-3 h-0.5 rounded-full" : "inline-block w-2.5 h-2.5 rounded-sm"}
            style={{ background: it.color }}
          />
          {it.label}
        </li>
      ))}
    </ul>
  );
}

export function Swatch({ color }: { color: string }) {
  return (
    <span
      aria-hidden
      className="inline-block w-2.5 h-2.5 rounded-sm mr-2 align-middle"
      style={{ background: color }}
    />
  );
}

type TooltipEntry = {
  name?: unknown;
  value?: unknown;
  color?: string;
  dataKey?: unknown;
};

/** Shared tooltip body: a title row then one row per series. */
export function ChartTooltip({
  active,
  payload,
  label,
  title,
  value = (v) => String(v),
}: {
  active?: boolean;
  payload?: ReadonlyArray<TooltipEntry>;
  label?: unknown;
  title: (label: string) => string;
  value?: (v: number, entry: TooltipEntry) => string;
}) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-xl border border-border-primary bg-surface-primary px-3 py-2 shadow-subtle text-xs">
      <p className="font-medium text-text-primary mb-1">{title(String(label))}</p>
      <ul className="space-y-0.5">
        {payload.map((p, i) => (
          <li key={i} className="flex items-center gap-2 text-text-secondary">
            <span
              aria-hidden
              className="inline-block w-2 h-2 rounded-sm"
              style={{ background: p.color }}
            />
            <span>{String(p.name)}</span>
            <span className="ml-auto pl-3 font-medium text-text-primary tabular-nums">
              {p.value === null || p.value === undefined
                ? "—"
                : value(Number(p.value), p)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function NoData({ message }: { message: string }) {
  return (
    <div className="flex items-center justify-center h-40 text-sm text-text-tertiary">
      {message}
    </div>
  );
}
