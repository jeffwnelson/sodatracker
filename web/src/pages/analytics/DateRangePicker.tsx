import { useEffect, useRef, useState } from "react";
import { Button } from "../../components/Button";
import {
  COMPARE_MODES,
  RANGES,
  addDays,
  compareDays,
  dayKey,
  formatDayRange,
  parseDay,
  presetDays,
} from "./analytics";
import type { CompareMode, DayRange, PresetId, RangeId } from "./analytics";
import { COMPARE_COLOR, CompareDot } from "./components";

type Draft = {
  days: DayRange | null;
  preset: PresetId | null;
  compare: CompareMode;
  compareRange: DayRange | null;
};

const WEEK = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];

function monthStart(key: string): string {
  return key.slice(0, 8) + "01";
}

function addMonths(key: string, n: number): string {
  const d = parseDay(key);
  return dayKey(new Date(d.getFullYear(), d.getMonth() + n, 1));
}

const inRange = (k: string, r: DayRange | null) => !!r && k >= r.start && k <= r.end;

/**
 * Range picker with an optional comparison, after Countly's: presets on the
 * left, two months of calendar, and the range, comparison and Apply on the
 * right. The selected range is accent, the comparison orange. Nothing
 * changes until Apply.
 */
export function DateRangePicker({
  range,
  days,
  compare,
  compareRange,
  onApply,
}: {
  range: RangeId;
  days: DayRange | null;
  compare: CompareMode;
  compareRange: DayRange | null;
  onApply: (d: Draft) => void;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const preset = RANGES.find((r) => r.id === range);
  const label = days ? formatDayRange(days) : "All time";

  return (
    <div ref={ref}>
      <Button
        variant="secondary"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-haspopup="dialog"
      >
        <svg aria-hidden viewBox="0 0 20 20" className="w-4 h-4 text-text-tertiary" fill="none" stroke="currentColor" strokeWidth="1.6">
          <rect x="3" y="4.5" width="14" height="12" rx="2" />
          <path d="M3 8.5h14M7 3v3M13 3v3" strokeLinecap="round" />
        </svg>
        <span>
          {range !== "custom" && preset && preset.days > 0 && (
            <span className="text-text-tertiary">Last {preset.label} · </span>
          )}
          {label}
        </span>
        {compareRange && (
          <span className="flex items-center gap-1.5 text-text-secondary">
            <span className="text-text-tertiary">vs</span>
            <CompareDot />
            {formatDayRange(compareRange)}
          </span>
        )}
      </Button>
      {open && (
        <Popover
          initial={{ days, preset: range === "custom" ? null : (range as PresetId), compare, compareRange }}
          onCancel={() => setOpen(false)}
          onApply={(d) => {
            onApply(d);
            setOpen(false);
          }}
        />
      )}
    </div>
  );
}

function Popover({
  initial,
  onCancel,
  onApply,
}: {
  initial: Draft;
  onCancel: () => void;
  onApply: (d: Draft) => void;
}) {
  const today = dayKey(new Date());
  const [draft, setDraft] = useState<Draft>(initial);
  // Which range calendar clicks edit; the comparison only when custom.
  const [target, setTarget] = useState<"primary" | "compare">("primary");
  const [anchor, setAnchor] = useState<string | null>(null);
  const [hover, setHover] = useState<string | null>(null);
  const [view, setView] = useState(() =>
    addMonths(monthStart(initial.days?.end ?? today), -1),
  );

  const effectiveCompare =
    draft.compare === "custom"
      ? draft.compareRange
      : draft.days
        ? compareDays(draft.compare, draft.days)
        : null;

  // While choosing the second end, preview the span under the cursor.
  const preview: DayRange | null =
    anchor && hover
      ? anchor <= hover
        ? { start: anchor, end: hover }
        : { start: hover, end: anchor }
      : null;
  const shownPrimary = target === "primary" && preview ? preview : draft.days;
  const shownCompare = target === "compare" && preview ? preview : effectiveCompare;

  const pick = (k: string) => {
    if (!anchor) {
      setAnchor(k);
      setHover(k);
      return;
    }
    const r = anchor <= k ? { start: anchor, end: k } : { start: k, end: anchor };
    setAnchor(null);
    if (target === "primary") setDraft((d) => ({ ...d, days: r, preset: null }));
    else setDraft((d) => ({ ...d, compareRange: r }));
  };

  const setCompare = (mode: CompareMode) => {
    setDraft((d) => {
      // Seed a custom comparison from the previous period, as a start.
      const seeded =
        mode === "custom" && !d.compareRange && d.days ? compareDays("previous", d.days) : d.compareRange;
      return { ...d, compare: mode, compareRange: seeded };
    });
    setTarget(mode === "custom" ? "compare" : "primary");
    setAnchor(null);
  };

  const canApply =
    draft.compare !== "custom" || (!!draft.compareRange && !!draft.days);

  return (
    <div
      role="dialog"
      aria-label="Choose date range"
      className="absolute left-0 top-full mt-2 z-30 w-max max-w-[calc(100vw-2rem)] overflow-x-auto rounded-2xl border border-border-primary bg-surface-primary shadow-lg animate-scale-in"
    >
      <div className="flex">
        {/* Presets */}
        <ul className="w-36 shrink-0 border-r border-border-primary p-2 text-sm">
          {RANGES.map((r) => {
            const active = draft.preset === r.id;
            return (
              <li key={r.id}>
                <button
                  type="button"
                  onClick={() => {
                    setAnchor(null);
                    setTarget("primary");
                    setDraft((d) => ({
                      ...d,
                      days: presetDays(r.id),
                      preset: r.id,
                      ...(r.days === 0 ? { compare: "none" as const } : {}),
                    }));
                    if (r.days) setView(addMonths(monthStart(today), -1));
                  }}
                  className={`w-full text-left px-3 py-1.5 rounded-lg font-medium transition-colors duration-150 ${
                    active ? "bg-accent-soft text-accent" : "text-text-secondary hover:bg-surface-secondary"
                  }`}
                >
                  {r.days ? `Last ${r.label}` : r.label}
                </button>
              </li>
            );
          })}
        </ul>

        {/* Calendars */}
        <div className="p-4">
          <div className="flex gap-6">
            {[view, addMonths(view, 1)].map((m, i) => (
              <Month
                key={m}
                month={m}
                today={today}
                primary={shownPrimary}
                compare={shownCompare}
                onPick={pick}
                onHover={(k) => anchor && setHover(k)}
                onPrev={i === 0 ? () => setView((v) => addMonths(v, -1)) : undefined}
                onNext={i === 1 && addMonths(view, 1) < monthStart(today) ? () => setView((v) => addMonths(v, 1)) : undefined}
              />
            ))}
          </div>
          <p className="mt-3 text-xs text-text-tertiary h-4">
            {anchor
              ? "Click the last day"
              : target === "compare"
                ? "Click two days to set the comparison"
                : "Click two days to set the range"}
          </p>
        </div>

        {/* Range and comparison */}
        <div className="w-72 shrink-0 border-l border-border-primary bg-surface-secondary p-4 flex flex-col gap-4 text-sm">
          <fieldset className="space-y-1.5">
            <legend className="sr-only">Mode</legend>
            {(
              [
                ["single", "Single range"],
                ["compare", "Compare ranges"],
              ] as const
            ).map(([v, l]) => {
              const disabled = v === "compare" && !draft.days;
              return (
                <label key={v} className={`flex items-center gap-2 ${disabled ? "opacity-40" : "cursor-pointer"}`}>
                  <input
                    type="radio"
                    name="mode"
                    className="accent-[var(--color-accent)]"
                    checked={v === "single" ? draft.compare === "none" : draft.compare !== "none"}
                    disabled={disabled}
                    onChange={() => setCompare(v === "single" ? "none" : "previous")}
                  />
                  {l}
                </label>
              );
            })}
            {!draft.days && <p className="text-xs text-text-tertiary">All time has nothing before it to compare with.</p>}
          </fieldset>

          <RangeFields
            title="Range"
            color="var(--color-accent)"
            value={draft.days}
            today={today}
            active={target === "primary"}
            onFocus={() => {
              setTarget("primary");
              setAnchor(null);
            }}
            onChange={(r) => setDraft((d) => ({ ...d, days: r, preset: null }))}
          />

          {draft.compare !== "none" && (
            <div className="space-y-2">
              <label className="block">
                <span className="block text-xs font-medium uppercase tracking-wider mb-1" style={{ color: COMPARE_COLOR }}>
                  Compare to
                </span>
                <select
                  value={draft.compare}
                  onChange={(e) => setCompare(e.target.value as CompareMode)}
                  className="w-full rounded-lg border border-border-primary bg-surface-primary px-2 py-1.5"
                >
                  {COMPARE_MODES.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.label}
                    </option>
                  ))}
                </select>
              </label>
              {draft.compare === "custom" ? (
                <RangeFields
                  value={draft.compareRange}
                  color={COMPARE_COLOR}
                  today={today}
                  active={target === "compare"}
                  onFocus={() => {
                    setTarget("compare");
                    setAnchor(null);
                  }}
                  onChange={(r) => setDraft((d) => ({ ...d, compareRange: r }))}
                />
              ) : (
                effectiveCompare && (
                  <p className="flex items-center gap-1.5 text-text-secondary">
                    <CompareDot />
                    {formatDayRange(effectiveCompare)}
                  </p>
                )
              )}
            </div>
          )}

          <div className="mt-auto flex justify-end gap-2">
            <Button variant="ghost" onClick={onCancel}>
              Cancel
            </Button>
            <Button onClick={() => onApply(draft)} disabled={!canApply}>
              Apply
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function RangeFields({
  title,
  color,
  value,
  today,
  active,
  onFocus,
  onChange,
}: {
  title?: string;
  color: string;
  value: DayRange | null;
  today: string;
  active: boolean;
  onFocus: () => void;
  onChange: (r: DayRange) => void;
}) {
  const field = (which: "start" | "end", label: string) => (
    <label className="block flex-1 min-w-0">
      <span className="block text-[11px] text-text-tertiary mb-0.5">{label}</span>
      <input
        type="date"
        max={today}
        value={value?.[which] ?? ""}
        onFocus={onFocus}
        onChange={(e) => {
          const v = e.target.value;
          if (!v) return;
          const next = { start: value?.start ?? v, end: value?.end ?? v, [which]: v };
          onChange(next.start <= next.end ? next : { start: next.end, end: next.start });
        }}
        className="w-full min-w-0 rounded-lg border bg-surface-primary px-1.5 py-1.5 text-xs tabular-nums"
        style={{ borderColor: active ? color : "var(--color-border-primary)" }}
      />
    </label>
  );
  return (
    <div onClick={onFocus}>
      {title && (
        <span className="block text-xs font-medium uppercase tracking-wider mb-1" style={{ color }}>
          {title}
        </span>
      )}
      <div className="flex gap-2">
        {field("start", "From")}
        {field("end", "To")}
      </div>
    </div>
  );
}

function Month({
  month,
  today,
  primary,
  compare,
  onPick,
  onHover,
  onPrev,
  onNext,
}: {
  month: string;
  today: string;
  primary: DayRange | null;
  compare: DayRange | null;
  onPick: (k: string) => void;
  onHover: (k: string) => void;
  onPrev?: () => void;
  onNext?: () => void;
}) {
  const first = parseDay(month);
  const lead = first.getDay();
  const count = new Date(first.getFullYear(), first.getMonth() + 1, 0).getDate();
  const cells: (string | null)[] = [
    ...Array.from({ length: lead }, () => null),
    ...Array.from({ length: count }, (_, i) => addDays(month, i)),
  ];
  const navBtn = "w-7 h-7 rounded-lg text-text-tertiary hover:bg-surface-secondary hover:text-text-primary";

  return (
    <div className="w-[232px]">
      <div className="flex items-center justify-between mb-2">
        {onPrev ? (
          <button type="button" onClick={onPrev} className={navBtn} aria-label="Previous month">‹</button>
        ) : (
          <span className="w-7" />
        )}
        <span className="text-sm font-medium text-text-primary">
          {first.toLocaleDateString(undefined, { month: "long", year: "numeric" })}
        </span>
        {onNext ? (
          <button type="button" onClick={onNext} className={navBtn} aria-label="Next month">›</button>
        ) : (
          <span className="w-7" />
        )}
      </div>
      <div className="grid grid-cols-7 gap-y-0.5 text-center text-xs">
        {WEEK.map((w) => (
          <span key={w} className="py-1 text-[11px] font-medium text-text-tertiary">
            {w}
          </span>
        ))}
        {cells.map((k, i) => {
          if (!k) return <span key={`b${i}`} />;
          const future = k > today;
          const inP = inRange(k, primary);
          const inC = !inP && inRange(k, compare);
          const edgeP = inP && (k === primary!.start || k === primary!.end);
          const edgeC = inC && (k === compare!.start || k === compare!.end);
          let cls = "text-text-primary hover:bg-surface-tertiary";
          let style: React.CSSProperties | undefined;
          if (edgeP) cls = "bg-accent text-white";
          else if (inP) cls = "bg-accent-soft text-accent";
          else if (edgeC) {
            cls = "text-white";
            style = { background: COMPARE_COLOR };
          } else if (inC) {
            cls = "";
            style = { background: "#fdeee7", color: "#b4471c" };
          }
          return (
            <button
              key={k}
              type="button"
              disabled={future}
              onClick={() => onPick(k)}
              onMouseEnter={() => onHover(k)}
              aria-label={parseDay(k).toDateString()}
              aria-pressed={inP || inC}
              className={`h-8 rounded-md tabular-nums transition-colors duration-100 disabled:text-text-muted disabled:hover:bg-transparent ${cls} ${
                k === today ? "font-semibold underline underline-offset-4" : ""
              }`}
              style={style}
            >
              {parseDay(k).getDate()}
            </button>
          );
        })}
      </div>
    </div>
  );
}
