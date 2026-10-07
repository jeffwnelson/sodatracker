import { SEQUENTIAL, WEEKDAYS, WEEKDAYS_LONG, formatHour } from "./analytics";

/** Dispenses by weekday (Monday first) and local hour, as a grid of cells. */
export function Heatmap({ heatmap }: { heatmap: number[][] }) {
  const max = Math.max(1, ...heatmap.flat());
  const step = (v: number) =>
    v === 0 ? null : SEQUENTIAL[Math.min(SEQUENTIAL.length - 1, Math.floor((v / max) * SEQUENTIAL.length - 1e-9))];

  return (
    <div>
      {/* All 24 hours fit a phone's width (about 11px a cell), so the day
          never scrolls out of view; wider screens get roomier cells. */}
      <div className="overflow-x-auto">
        <div
          className="grid gap-px sm:gap-0.5 sm:min-w-[640px]"
          style={{ gridTemplateColumns: "2rem repeat(24, minmax(0, 1fr))" }}
          role="img"
          aria-label="Heatmap of dispenses by weekday and hour; use the table view for values"
        >
          {heatmap.map((row, d) => (
            <div key={d} className="contents">
              <div className="text-[11px] text-text-tertiary flex items-center">
                {WEEKDAYS[d]}
              </div>
              {row.map((v, h) => {
                const color = step(v);
                return (
                  <div
                    key={h}
                    title={`${WEEKDAYS_LONG[d]} ${formatHour(h)}: ${v} dispense${v === 1 ? "" : "s"}`}
                    className={`h-5 sm:h-6 rounded-[2px] sm:rounded-[3px] ${color ? "" : "bg-surface-tertiary"}`}
                    style={color ? { background: color } : undefined}
                  />
                );
              })}
            </div>
          ))}
          <div />
          {Array.from({ length: 24 }, (_, h) => (
            <div key={h} className="text-[10px] sm:text-[11px] text-text-tertiary text-center pt-1 whitespace-nowrap">
              {h % 3 === 0 ? formatHour(h) : ""}
            </div>
          ))}
        </div>
      </div>
      <div className="mt-4 flex items-center gap-2 text-[11px] text-text-tertiary">
        <span>0</span>
        <span className="inline-block w-4 h-3 rounded-[3px] bg-surface-tertiary" />
        {SEQUENTIAL.map((c) => (
          <span key={c} className="inline-block w-4 h-3 rounded-[3px]" style={{ background: c }} />
        ))}
        <span>{max}</span>
      </div>
    </div>
  );
}

export function HeatmapTable({ heatmap }: { heatmap: number[][] }) {
  return (
    <div className="overflow-x-auto">
      <table className="text-xs tabular-nums">
        <thead>
          <tr>
            <th className="pr-2 text-left font-medium text-text-tertiary">Hour</th>
            {WEEKDAYS.map((d) => (
              <th key={d} className="px-2 text-right font-medium text-text-tertiary">
                {d}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: 24 }, (_, h) => (
            <tr key={h} className="border-t border-border-primary">
              <td className="pr-2 py-1 text-text-tertiary">{formatHour(h)}</td>
              {heatmap.map((row, d) => (
                <td key={d} className={`px-2 py-1 text-right ${row[h] ? "text-text-primary" : "text-text-muted"}`}>
                  {row[h]}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
