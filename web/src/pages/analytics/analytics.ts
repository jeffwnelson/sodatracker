import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { getAnalytics, listMachines } from "../../api/client";
import type { AnalyticsByKind, AnalyticsKind, Bucket } from "../../api/types";

// --- Filters -------------------------------------------------------------

export const RANGES = [
  { id: "7d", label: "7 days", days: 7 },
  { id: "30d", label: "30 days", days: 30 },
  { id: "90d", label: "90 days", days: 90 },
  { id: "12m", label: "12 months", days: 365 },
  { id: "all", label: "All time", days: 0 },
] as const;

export type PresetId = (typeof RANGES)[number]["id"];
export type RangeId = PresetId | "custom";
export type CompareMode = "none" | "previous" | "year" | "custom";

export const COMPARE_MODES: { id: CompareMode; label: string }[] = [
  { id: "previous", label: "Previous period" },
  { id: "year", label: "Same period last year" },
  { id: "custom", label: "Custom range" },
];

const DEFAULT_RANGE: PresetId = "30d";

/** The server clamps an early start to when tracking began. */
const ALL_TIME_FROM = "2000-01-01T00:00:00Z";

export const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;

/** A span of whole local days, both ends inclusive, as YYYY-MM-DD. */
export interface DayRange {
  start: string;
  end: string;
}

export function dayKey(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

/** Local midnight of a YYYY-MM-DD day. */
export function parseDay(key: string): Date {
  const [y, m, d] = key.split("-").map(Number);
  return new Date(y, m - 1, d);
}

/** The first day of key's month. */
export function monthStart(key: string): string {
  return key.slice(0, 8) + "01";
}

/** The first day of the month n months from key's. */
export function addMonths(key: string, n: number): string {
  const d = parseDay(key);
  return dayKey(new Date(d.getFullYear(), d.getMonth() + n, 1));
}

export function addDays(key: string, n: number): string {
  const d = parseDay(key);
  d.setDate(d.getDate() + n);
  return dayKey(d);
}

/** Calendar days from a to b (b − a), ignoring DST. */
export function daysBetween(a: string, b: string): number {
  const [ay, am, ad] = a.split("-").map(Number);
  const [by, bm, bd] = b.split("-").map(Number);
  return Math.round((Date.UTC(by, bm - 1, bd) - Date.UTC(ay, am - 1, ad)) / 86_400_000);
}

const isDay = (s: string | null): s is string => !!s && /^\d{4}-\d{2}-\d{2}$/.test(s);

/** A preset covers N calendar days ending today. */
export function presetDays(id: PresetId): DayRange | null {
  const preset = RANGES.find((r) => r.id === id);
  if (!preset || preset.days === 0) return null;
  const today = dayKey(new Date());
  return { start: addDays(today, -preset.days + 1), end: today };
}

/**
 * The comparison days for a range: the same number of days just before it,
 * or the same dates a year earlier.
 */
export function compareDays(mode: CompareMode, r: DayRange): DayRange | null {
  if (mode === "previous") {
    const n = daysBetween(r.start, r.end) + 1;
    return { start: addDays(r.start, -n), end: addDays(r.end, -n) };
  }
  if (mode === "year") {
    const shift = (k: string) => {
      const d = parseDay(k);
      d.setFullYear(d.getFullYear() - 1);
      return dayKey(d);
    };
    return { start: shift(r.start), end: shift(r.end) };
  }
  return null;
}

/** An analytics window as the API takes it. to is omitted for "now". */
export interface Period {
  days: DayRange | null;
  from: string;
  to?: string;
}

// "Now" moves in five-minute steps, so query keys built from it stay put
// between refetches.
function nowStep(): Date {
  const step = 5 * 60_000;
  return new Date(Math.floor(Date.now() / step) * step);
}

/**
 * The API window for a span of days. A span ending today stops at now;
 * `partialOf`, the primary range when this is its comparison, cuts the
 * comparison at the same point in its last day, so a half-finished today is
 * compared with the same half of the other day.
 */
function toPeriod(days: DayRange, partialOf?: DayRange): Period {
  const from = parseDay(days.start).toISOString();
  const today = dayKey(new Date());
  if (days.end >= today) return { days, from };
  if (partialOf && partialOf.end >= today) {
    const now = nowStep();
    const cut = parseDay(days.end);
    cut.setHours(now.getHours(), now.getMinutes());
    return { days, from, to: cut.toISOString() };
  }
  return { days, from, to: parseDay(addDays(days.end, 1)).toISOString() };
}

export function formatDayRange(r: DayRange): string {
  const a = parseDay(r.start);
  const b = parseDay(r.end);
  const sameYear = a.getFullYear() === b.getFullYear();
  const thisYear = b.getFullYear() === new Date().getFullYear();
  const opts: Intl.DateTimeFormatOptions = { month: "short", day: "numeric" };
  const left = a.toLocaleDateString(undefined, sameYear ? opts : { ...opts, year: "numeric" });
  const right = b.toLocaleDateString(undefined, thisYear && sameYear ? opts : { ...opts, year: "numeric" });
  return r.start === r.end ? right : `${left} – ${right}`;
}

/**
 * Range and comparison live in the URL so every tab shares them:
 * ?range=<preset|custom>&from=&to= (custom days) and
 * &compare=<previous|year|custom>&cfrom=&cto= (custom comparison).
 */
export function useAnalyticsFilters() {
  const [params, setParams] = useSearchParams();
  const machines = useQuery({ queryKey: ["machines"], queryFn: listMachines });
  // One machine: the app never asks which.
  const machineId = machines.data?.[0]?.id || "";

  const rangeParam = params.get("range");
  const fromParam = params.get("from");
  const toParam = params.get("to");
  let range: RangeId = RANGES.some((r) => r.id === rangeParam)
    ? (rangeParam as PresetId)
    : DEFAULT_RANGE;
  let days: DayRange | null = presetDays(range as PresetId);
  if (rangeParam === "custom" && isDay(fromParam) && isDay(toParam)) {
    range = "custom";
    days = fromParam <= toParam ? { start: fromParam, end: toParam } : { start: toParam, end: fromParam };
  }

  // All time has nothing before it to compare with.
  const compareParam = params.get("compare") as CompareMode | null;
  let compare: CompareMode =
    days && COMPARE_MODES.some((m) => m.id === compareParam) ? compareParam! : "none";
  let compareRange: DayRange | null = days ? compareDays(compare, days) : null;
  if (compare === "custom") {
    const cf = params.get("cfrom");
    const ct = params.get("cto");
    if (isDay(cf) && isDay(ct)) {
      compareRange = cf <= ct ? { start: cf, end: ct } : { start: ct, end: cf };
    } else {
      compare = "none";
    }
  }

  const period: Period = days ? toPeriod(days) : { days: null, from: ALL_TIME_FROM };
  const comparePeriod: Period | null =
    compareRange && days ? toPeriod(compareRange, days) : null;

  const update = (values: Record<string, string | null>) => {
    const next = new URLSearchParams(params);
    for (const [k, v] of Object.entries(values)) {
      if (v === null) next.delete(k);
      else next.set(k, v);
    }
    setParams(next, { replace: true });
  };

  return {
    machines,
    machineId,
    range,
    days,
    period,
    compare,
    compareRange,
    comparePeriod,
    /** A preset keeps the comparison mode, except All time. */
    setRange: (r: PresetId) =>
      update({
        range: r,
        from: null,
        to: null,
        ...(r === "all" ? { compare: null, cfrom: null, cto: null } : {}),
      }),
    apply: (next: { days: DayRange | null; preset: PresetId | null; compare: CompareMode; compareRange: DayRange | null }) =>
      update({
        range: next.preset ?? "custom",
        from: next.preset ? null : next.days?.start ?? null,
        to: next.preset ? null : next.days?.end ?? null,
        compare: next.compare === "none" ? null : next.compare,
        cfrom: next.compare === "custom" ? next.compareRange?.start ?? null : null,
        cto: next.compare === "custom" ? next.compareRange?.end ?? null : null,
      }),
  };
}

export function useAnalytics<K extends AnalyticsKind>(kind: K) {
  const { machineId, period } = useAnalyticsFilters();
  return useQuery({
    queryKey: ["analytics", kind, machineId, period.from, period.to, timezone],
    queryFn: () =>
      getAnalytics(kind, { machineId, from: period.from, to: period.to, tz: timezone }),
    enabled: !!machineId,
    staleTime: 30_000,
    // Hold the previous render while a new range loads instead of flashing
    // skeletons.
    placeholderData: keepPreviousData,
  });
}

/** The comparison range's data, positioned against the primary range. */
export interface Comparison<T> {
  data: T;
  range: DayRange;
  /** Days of the comparison range before tracking began, so missing. */
  missingDays: number;
  /**
   * Series index in the comparison for primary index i: both count buckets
   * from their own range's first day, since the server may start either
   * later (clamped to when tracking began).
   */
  at: <P extends { bucket: string }>(series: P[], primary: P[], i: number) => P | undefined;
}

/**
 * Days of the comparison range before tracking began, and its length. The
 * primary response says when tracking began.
 */
export function comparisonCoverage(
  compareRange: DayRange,
  trackingSince: string | null,
): { missing: number; total: number } {
  const total = daysBetween(compareRange.start, compareRange.end) + 1;
  if (!trackingSince) return { missing: total, total };
  const began = dayKey(new Date(trackingSince));
  return { missing: Math.min(total, Math.max(0, daysBetween(compareRange.start, began))), total };
}

/**
 * The selected comparison for a tab, or null when comparison is off, still
 * loading, or wholly before tracking began (its zeros would read as a
 * collapse; ComparisonNotice says why instead). It uses the primary
 * response's bucket size so the two line up.
 */
export function useComparison<K extends AnalyticsKind>(
  kind: K,
  primary: AnalyticsByKind[K] | undefined,
): Comparison<AnalyticsByKind[K]> | null {
  const { machineId, period, comparePeriod, compareRange } = useAnalyticsFilters();
  const bucket = primary?.range.bucket;
  const query = useQuery({
    queryKey: ["analytics", kind, machineId, comparePeriod?.from, comparePeriod?.to, timezone, bucket],
    queryFn: () =>
      getAnalytics(kind, {
        machineId,
        from: comparePeriod!.from,
        to: comparePeriod!.to,
        tz: timezone,
        bucket,
      }),
    enabled: !!machineId && !!comparePeriod && !!bucket,
    staleTime: 30_000,
  });
  if (!comparePeriod || !compareRange || !query.data || !bucket || !period.days || !primary) return null;
  const { missing, total } = comparisonCoverage(compareRange, primary.range.trackingSince);
  if (missing >= total) return null;
  const data = query.data as AnalyticsByKind[K];
  const primaryStart = period.days.start;
  const compareStart = compareRange.start;
  return {
    data,
    range: compareRange,
    missingDays: missing,
    at: (series, primarySeries, i) => {
      const p = primarySeries[i];
      if (!p) return undefined;
      const idx = bucketOffset(primaryStart, p.bucket, bucket);
      return series.find((c) => bucketOffset(compareStart, c.bucket, bucket) === idx);
    },
  };
}

/** Which bucket, counting from the one holding `start`, `iso` falls in. */
function bucketOffset(start: string, iso: string, bucket: Bucket): number {
  const d = dayKey(new Date(iso));
  if (bucket === "month") {
    const a = parseDay(start);
    const b = parseDay(d);
    return (b.getFullYear() - a.getFullYear()) * 12 + b.getMonth() - a.getMonth();
  }
  if (bucket === "week") {
    const monday = (k: string) => addDays(k, -((parseDay(k).getDay() + 6) % 7));
    return Math.round(daysBetween(monday(start), monday(d)) / 7);
  }
  return daysBetween(start, d);
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
 * Product colors for charts: a color chosen on the Settings page wins, and
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

/** Minutes after midnight as a clock time, e.g. 495 → "8:15am". */
export function formatMinuteOfDay(min: number): string {
  const h = Math.floor(min / 60) % 24;
  const m = String(min % 60).padStart(2, "0");
  const suffix = h < 12 ? "am" : "pm";
  return `${h % 12 === 0 ? 12 : h % 12}:${m}${suffix}`;
}

/** A length of time in the largest sensible units: 40 min, 5.5h, 3d 4h. */
export function formatHours(h: number): string {
  if (h <= 0) return "0h";
  if (h < 1) return `${Math.max(1, Math.round(h * 60))} min`;
  if (h < 48) return `${Math.round(h * 10) / 10}h`;
  const d = Math.floor(h / 24);
  const rest = Math.round(h - d * 24);
  return rest ? `${d}d ${rest}h` : `${d}d`;
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

/** Sums [weekday][hour] grids, e.g. the heatmaps of several products. */
export function sumHeatmaps(maps: number[][][]): number[][] {
  const out = Array.from({ length: 7 }, () => Array<number>(24).fill(0));
  for (const m of maps) m.forEach((row, d) => row.forEach((v, h) => (out[d][h] += v)));
  return out;
}
