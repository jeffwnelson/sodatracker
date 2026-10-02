import { useState } from "react";
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import type { ReactNode } from "react";
import type { UseQueryResult } from "@tanstack/react-query";
import type { AnalyticsRange } from "../../api/types";
import { comparisonCoverage, formatDayRange, useAnalyticsFilters } from "./analytics";
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

/** Comparison marks: the picker, tile lines and chart overlays share it. */
export const COMPARE_COLOR = "#eb6834";

export function CompareDot() {
  return (
    <span
      aria-hidden
      className="inline-block w-2 h-2 rounded-full shrink-0"
      style={{ background: COMPARE_COLOR }}
    />
  );
}

/**
 * Change against the comparison range. Neutral ink: more dispenses is not
 * inherently good or bad.
 */
export function Delta({
  current,
  previous,
  format = (n) => String(n),
}: {
  current: number | null;
  previous: number | null;
  format?: (n: number) => string;
}) {
  if (previous === null) return <span>No comparison data</span>;
  if (current === null) return <span>was {format(previous)}</span>;
  const diff = current - previous;
  if (Math.abs(diff) < 1e-9) return <span>Same · was {format(previous)}</span>;
  const pct = previous !== 0 ? ` (${diff > 0 ? "+" : "−"}${Math.abs(Math.round((diff / previous) * 100))}%)` : "";
  return (
    <span>
      {diff > 0 ? "▲" : "▼"} {format(Math.abs(diff))}
      {pct} · was {format(previous)}
    </span>
  );
}

export function StatTile({
  label,
  value,
  sub,
  compare,
}: {
  label: string;
  value: ReactNode;
  sub?: ReactNode;
  /** The comparison range's figure, usually a <Delta>. */
  compare?: ReactNode;
}) {
  return (
    <Card>
      <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-2">
        {label}
      </p>
      <p className="text-2xl sm:text-3xl font-semibold tracking-tight text-text-primary">
        {value}
      </p>
      {sub && <p className="mt-2 text-xs text-text-secondary">{sub}</p>}
      {compare && (
        <p className="mt-1.5 flex items-center gap-1.5 text-xs text-text-secondary">
          <CompareDot />
          {compare}
        </p>
      )}
    </Card>
  );
}

export function StatGrid({ children, cols = 4 }: { children: ReactNode; cols?: 4 | 5 }) {
  return (
    <div className={`grid grid-cols-2 gap-3 sm:gap-4 ${cols === 5 ? "lg:grid-cols-5" : "lg:grid-cols-4"}`}>
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
  compact = false,
  children,
}: {
  title: string;
  subtitle?: string;
  legend?: ReactNode;
  table?: ReactNode;
  /** Tighter padding, for cards laid out several to a row. */
  compact?: boolean;
  children: ReactNode;
}) {
  const [view, setView] = useState<"chart" | "table">("chart");
  return (
    <Card padding={compact ? "sm" : "md"}>
      {/* Compact cards keep the view toggle beside the title, wrapping the
          subtitle instead, so cards side by side line up. */}
      <div
        className={`flex items-start justify-between gap-3 ${compact ? "mb-3" : "flex-wrap mb-4"}`}
      >
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
            {title}
          </p>
          {subtitle && (
            <p className="mt-1 text-sm text-text-secondary">{subtitle}</p>
          )}
        </div>
        {table && (
          <div
            className="inline-flex shrink-0 rounded-lg border border-border-primary p-0.5 text-xs"
            role="group"
            aria-label={`${title} view`}
          >
            {(["chart", "table"] as const).map((v) => (
              <button
                key={v}
                type="button"
                onClick={() => setView(v)}
                aria-pressed={view === v}
                className={`px-3 py-2 sm:px-2.5 sm:py-1 rounded-md font-medium capitalize transition-colors duration-150 ${
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

export type Slice = { key: string; label: string; value: number; color: string };

/**
 * A donut beside a legend that lists each slice's value and share, so the
 * numbers are readable without hovering. Zero slices are left out. compact
 * stacks a smaller donut above the legend, for cards a third of the width.
 */
export function BreakdownPie({
  slices,
  format = (n) => String(n),
  compact = false,
}: {
  slices: Slice[];
  format?: (n: number) => string;
  compact?: boolean;
}) {
  const shown = slices.filter((s) => s.value > 0);
  const total = shown.reduce((sum, s) => sum + s.value, 0);
  return (
    <div
      className={`flex flex-col items-center ${compact ? "gap-4" : "sm:flex-row gap-6"}`}
    >
      <div className={`${compact ? "w-40 h-40" : "w-56 h-56"} shrink-0`}>
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              isAnimationActive={false}
              data={shown}
              dataKey="value"
              nameKey="label"
              innerRadius="58%"
              outerRadius="100%"
              paddingAngle={shown.length > 1 ? 1.5 : 0}
              stroke="none"
            >
              {shown.map((s) => (
                <Cell key={s.key} fill={s.color} />
              ))}
            </Pie>
            <Tooltip
              content={(p) => (
                <ChartTooltip
                  {...p}
                  label={p.payload?.[0]?.name}
                  title={(l) => l}
                  value={(v) => `${format(v)} (${Math.round((v / total) * 100)}%)`}
                />
              )}
            />
          </PieChart>
        </ResponsiveContainer>
      </div>
      <ul className={`w-full max-w-sm text-sm ${compact ? "space-y-1.5" : "space-y-2"}`}>
        {shown.map((s) => (
          <li key={s.key} className="flex items-center gap-2">
            <Swatch color={s.color} />
            <span className="text-text-secondary">{s.label}</span>
            <span className="ml-auto font-medium text-text-primary tabular-nums">
              {format(s.value)}
            </span>
            <span className="w-12 text-right text-text-tertiary tabular-nums">
              {Math.round((s.value / total) * 100)}%
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * Says how much of the comparison range has no data because tracking hadn't
 * begun, since a half-empty comparison reads as a big change.
 */
export function ComparisonNotice({ range }: { range: AnalyticsRange }) {
  const { compareRange } = useAnalyticsFilters();
  if (!compareRange) return null;
  const { missing, total } = comparisonCoverage(compareRange, range.trackingSince);
  if (missing === 0) return null;
  return (
    <p className="flex items-center gap-2 text-sm text-text-secondary bg-surface-tertiary rounded-xl px-4 py-3">
      <CompareDot />
      {missing >= total
        ? `The comparison range, ${formatDayRange(compareRange)}, is before tracking began, so there is nothing to compare with.`
        : `Tracking began partway through the comparison range: ${missing} of its ${total} days have no data.`}
    </p>
  );
}
