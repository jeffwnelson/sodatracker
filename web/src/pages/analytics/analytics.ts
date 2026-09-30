import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { getAnalytics, listMachines } from "../../api/client";
import type { AnalyticsKind, Bucket } from "../../api/types";

// --- Filters -------------------------------------------------------------

export const RANGES = [
  { id: "7d", label: "7 days", days: 7 },
  { id: "30d", label: "30 days", days: 30 },
  { id: "90d", label: "90 days", days: 90 },
  { id: "12m", label: "12 months", days: 365 },
  { id: "all", label: "All time", days: 0 },
] as const;

export type RangeId = (typeof RANGES)[number]["id"];

const DEFAULT_RANGE: RangeId = "30d";

/** The server clamps an early start to when tracking began. */
const ALL_TIME_FROM = "2000-01-01T00:00:00Z";

export const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;

/**
 * Start of the window: local midnight N-1 days ago, so the window covers N
 * calendar days including today. Stable for the whole day, which keeps the
 * query key stable.
 */
function rangeStart(range: RangeId): string {
  const preset = RANGES.find((r) => r.id === range);
  if (!preset || preset.days === 0) return ALL_TIME_FROM;
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() - preset.days + 1);
  return d.toISOString();
}

/** Machine and range live in the URL so every tab shares them. */
export function useAnalyticsFilters() {
  const [params, setParams] = useSearchParams();
  const machines = useQuery({ queryKey: ["machines"], queryFn: listMachines });
  const rangeParam = params.get("range");
  const range: RangeId = RANGES.some((r) => r.id === rangeParam)
    ? (rangeParam as RangeId)
    : DEFAULT_RANGE;
  const machineId = params.get("machine") || machines.data?.[0]?.id || "";

  const update = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    next.set(key, value);
    setParams(next, { replace: true });
  };

  return {
    machines,
    machineId,
    range,
    setMachine: (id: string) => update("machine", id),
    setRange: (r: RangeId) => update("range", r),
  };
}

export function useAnalytics<K extends AnalyticsKind>(kind: K) {
  const { machineId, range } = useAnalyticsFilters();
  const from = rangeStart(range);
  return useQuery({
    queryKey: ["analytics", kind, machineId, from, timezone],
    queryFn: () => getAnalytics(kind, { machineId, from, tz: timezone }),
    enabled: !!machineId,
    staleTime: 30_000,
    // Hold the previous render while a new range loads instead of flashing
    // skeletons.
    placeholderData: keepPreviousData,
  });
}

// --- Color -----------------------------------------------------------------

/**
 * Categorical slots in fixed order, validated for CVD separation on the white
 * card surface. Magenta, yellow and aqua are under 3:1 contrast, so every
 * chart that uses them has a table view and a labelled legend.
 */
export const SERIES = [
  "#2a78d6",
  "#eb6834",
  "#1baf7a",
  "#eda100",
  "#e87ba4",
  "#008300",
  "#4a3aa7",
  "#e34948",
];
export const OTHER_COLOR = "#a8a29e";

/** Single-series marks use the app accent. */
export const ACCENT = "var(--color-accent)";

/** One-hue blue ramp, light to dark, for magnitude. */
export const SEQUENTIAL = [
  "#cde2fb",
  "#9ec5f4",
  "#6da7ec",
  "#3987e5",
  "#256abf",
  "#184f95",
  "#0d366b",
];

/**
 * Colors follow the entity: ids are sorted once, so a product keeps its hue
 * whichever range or tab is showing. Past eight, the rest share gray.
 */
export function colorMap(ids: string[]): Record<string, string> {
  const out: Record<string, string> = {};
  [...new Set(ids)].sort().forEach((id, i) => {
    out[id] = SERIES[i] ?? OTHER_COLOR;
  });
  return out;
}

/**
 * Product colors for charts: a color chosen on the Setup page wins, and
 * products without one keep their stable palette slot. Chosen colors aren't
 * CVD-validated like SERIES, which is one more reason every chart keeps its
 * table view and labelled legend.
 */
export function productColorMap(
  catalog: { id: string; color?: string }[],
  ids: string[],
): Record<string, string> {
  const out = colorMap([...catalog.map((p) => p.id), ...ids]);
  for (const p of catalog) {
    if (p.color) out[p.id] = p.color;
  }
  return out;
}

// --- Formatting --------------------------------------------------------------

export const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
export const WEEKDAYS_LONG = [
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
  "Sunday",
];

export function formatBucket(iso: string, bucket: Bucket): string {
  const d = new Date(iso);
  if (bucket === "month") {
    return d.toLocaleDateString(undefined, { month: "short", year: "numeric" });
  }
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

export function formatBucketLong(iso: string, bucket: Bucket): string {
  const d = new Date(iso);
  if (bucket === "month") {
    return d.toLocaleDateString(undefined, { month: "long", year: "numeric" });
  }
  const day = d.toLocaleDateString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
    year: "numeric",
  });
  return bucket === "week" ? `Week of ${day}` : day;
}

export function formatHour(h: number): string {
  if (h === 0) return "12am";
  if (h === 12) return "12pm";
  return h < 12 ? `${h}am` : `${h - 12}pm`;
}

export function money(n: number): string {
  return `$${n.toFixed(2)}`;
}

export function percent(n: number): string {
  return `${Math.round(n * 100)}%`;
}

export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/** Recharts defaults the app's charts share. */
export const axisProps = {
  axisLine: false,
  tickLine: false,
} as const;
