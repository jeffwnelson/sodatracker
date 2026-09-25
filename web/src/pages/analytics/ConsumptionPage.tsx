import {
  Bar,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { ConsumptionAnalytics } from "../../api/types";
import { DataTable } from "../../components/DataTable";
import {
  ACCENT,
  SEQUENTIAL,
  WEEKDAYS,
  WEEKDAYS_LONG,
  axisProps,
  formatBucket,
  formatBucketLong,
  formatHour,
  useAnalytics,
} from "./analytics";
import {
  ChartCard,
  ChartTooltip,
  Delta,
  Legend,
  Loaded,
  NoData,
  StatGrid,
  StatTile,
} from "./components";

const TREND_COLOR = "#1c1917";

export function ConsumptionPage() {
  const query = useAnalytics("consumption");
  return <Loaded query={query}>{(data) => <Consumption data={data} />}</Loaded>;
}

function Consumption({ data }: { data: ConsumptionAnalytics }) {
  const bucket = data.range.bucket;
  const trendWindow = { day: "7-day", week: "4-week", month: "3-month" }[bucket];

  return (
    <>
      <StatGrid>
        <StatTile
          label="Dispenses"
          value={data.total}
          sub={<Delta current={data.total} previous={data.previousTotal} />}
        />
        <StatTile
          label="Per day"
          value={data.perDay.toFixed(1)}
          sub={`Averaged over ${Math.round(data.range.days)} days`}
        />
        <StatTile
          label="Weekday vs weekend"
          value={`${data.weekdayPerDay.toFixed(1)} / ${data.weekendPerDay.toFixed(1)}`}
          sub="Dispenses per weekday / weekend day"
        />
        <StatTile
          label="Busiest time"
          value={data.peakHour < 0 ? "—" : formatHour(data.peakHour)}
          sub={
            data.peakWeekday < 0
              ? "No dispenses yet"
              : `Busiest day: ${WEEKDAYS_LONG[data.peakWeekday]}`
          }
        />
      </StatGrid>

      <ChartCard
        title="Dispenses over time"
        subtitle={`Per ${bucket}, with a ${trendWindow} moving average`}
        legend={
          <Legend
            items={[
              { key: "count", label: `Dispenses per ${bucket}`, color: ACCENT },
              { key: "trend", label: `${trendWindow} average`, color: TREND_COLOR, line: true },
            ]}
          />
        }
        table={
          <DataTable
            columns={[
              { header: bucket, accessor: (r) => formatBucketLong(r.bucket, bucket), headerClassName: "capitalize" },
              { header: "Dispenses", accessor: (r) => r.count, className: "text-right tabular-nums", headerClassName: "text-right" },
              { header: "Average", accessor: (r) => r.trend.toFixed(1), className: "text-right tabular-nums", headerClassName: "text-right" },
            ]}
            data={[...data.series].reverse()}
            keyFn={(r) => r.bucket}
          />
        }
      >
        {data.total === 0 ? (
          <NoData message="No dispenses in this range." />
        ) : (
          <ResponsiveContainer width="100%" height={280}>
            <ComposedChart data={data.series} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="bucket"
                tickFormatter={(v: string) => formatBucket(v, bucket)}
                minTickGap={24}
                {...axisProps}
              />
              <YAxis allowDecimals={false} {...axisProps} />
              <Tooltip
                cursor={{ fill: "var(--color-surface-tertiary)" }}
                content={(p) => (
                  <ChartTooltip
                    {...p}
                    title={(l) => formatBucketLong(l, bucket)}
                    value={(v, e) => (e.dataKey === "trend" ? v.toFixed(1) : String(v))}
                  />
                )}
              />
              <Bar
                isAnimationActive={false}
                dataKey="count"
                name="Dispenses"
                fill={ACCENT}
                radius={[4, 4, 0, 0]}
                maxBarSize={28}
              />
              <Line
                isAnimationActive={false}
                dataKey="trend"
                name={`${trendWindow} average`}
                stroke={TREND_COLOR}
                strokeWidth={2}
                dot={false}
                type="monotone"
              />
            </ComposedChart>
          </ResponsiveContainer>
        )}
      </ChartCard>

      <ChartCard
        title="When people buy"
        subtitle="Dispenses by day of week and hour"
        table={<HeatmapTable heatmap={data.heatmap} />}
      >
        {data.total === 0 ? (
          <NoData message="No dispenses in this range." />
        ) : (
          <Heatmap heatmap={data.heatmap} />
        )}
      </ChartCard>
    </>
  );
}

function Heatmap({ heatmap }: { heatmap: number[][] }) {
  const max = Math.max(1, ...heatmap.flat());
  const step = (v: number) =>
    v === 0 ? null : SEQUENTIAL[Math.min(SEQUENTIAL.length - 1, Math.floor((v / max) * SEQUENTIAL.length - 1e-9))];

  return (
    <div>
      <div className="overflow-x-auto">
        <div
          className="grid gap-0.5 min-w-[640px]"
          style={{ gridTemplateColumns: "2.5rem repeat(24, minmax(0, 1fr))" }}
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
                    className={`h-6 rounded-[3px] ${color ? "" : "bg-surface-tertiary"}`}
                    style={color ? { background: color } : undefined}
                  />
                );
              })}
            </div>
          ))}
          <div />
          {Array.from({ length: 24 }, (_, h) => (
            <div key={h} className="text-[11px] text-text-tertiary text-center pt-1">
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

function HeatmapTable({ heatmap }: { heatmap: number[][] }) {
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
