import {
  Area,
  ComposedChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { InventoryAnalytics, SlotInventory } from "../../api/types";
import { Card } from "../../components/Card";
import { DataTable } from "../../components/DataTable";
import type { Column } from "../../components/DataTable";
import {
  ACCENT,
  axisProps,
  formatBucket,
  formatBucketLong,
  formatDateTime,
  percent,
  useAnalytics,
} from "./analytics";
import {
  ChartCard,
  Loaded,
  NoData,
  StatGrid,
  StatTile,
} from "./components";

export function InventoryPage() {
  const query = useAnalytics("inventory");
  return <Loaded query={query}>{(data) => <Inventory data={data} />}</Loaded>;
}

function hours(h: number): string {
  if (h === 0) return "0h";
  if (h < 48) return `${h.toFixed(1)}h`;
  return `${(h / 24).toFixed(1)}d`;
}

function days(d: number | null): string {
  return d === null ? "—" : `${d.toFixed(1)} days`;
}

function slotLabel(s: SlotInventory): string {
  return `Slot ${s.position}${s.productName ? ` · ${s.productName}` : ""}`;
}

function Inventory({ data }: { data: InventoryAnalytics }) {
  const bucket = data.range.bucket;
  const { summary } = data;
  const tracked = data.slots.filter((s) => s.currentStock !== null);
  const series = data.series.map((pt) => ({ bucket: pt.bucket, ...pt.levels }));
  const hasLevels = data.series.some((pt) => Object.values(pt.levels).some((v) => v !== null));

  const slotColumns: Column<SlotInventory>[] = [
    {
      header: "Slot",
      accessor: (s) => (
        <span className="font-medium text-text-primary">{slotLabel(s)}</span>
      ),
    },
    {
      header: "Stock",
      accessor: (s) => (s.currentStock === null ? "Never restocked" : `${s.currentStock} / ${s.capacity}`),
      className: "text-right tabular-nums whitespace-nowrap",
      headerClassName: "text-right",
    },
    { header: "Time empty", accessor: (s) => hours(s.stockoutHours), className: "text-right tabular-nums", headerClassName: "text-right" },
    { header: "Ran out", accessor: (s) => s.stockouts, className: "text-right tabular-nums", headerClassName: "text-right" },
    { header: "Restocks", accessor: (s) => s.restocks, className: "text-right tabular-nums", headerClassName: "text-right" },
    { header: "Cans in", accessor: (s) => s.cansRestocked, className: "text-right tabular-nums", headerClassName: "text-right" },
    { header: "Cans out", accessor: (s) => s.dispensed, className: "text-right tabular-nums", headerClassName: "text-right" },
    { header: "Used before restock", accessor: (s) => (s.usedShare === null ? "—" : percent(s.usedShare)), className: "text-right tabular-nums", headerClassName: "text-right" },
    { header: "Restock every", accessor: (s) => days(s.avgRestockIntervalDays), className: "text-right tabular-nums whitespace-nowrap", headerClassName: "text-right" },
  ];

  return (
    <>
      <StatGrid>
        <StatTile
          label="Time empty"
          value={hours(summary.stockoutHours)}
          sub={`Summed across slots · ran out ${summary.stockouts} time${summary.stockouts === 1 ? "" : "s"}`}
        />
        <StatTile
          label="Used before restock"
          value={summary.usedShare === null ? "—" : percent(summary.usedShare)}
          sub="Share of restocked cans dispensed before the next restock"
        />
        <StatTile
          label="Restocks"
          value={summary.restockCount}
          sub={
            summary.avgRestockIntervalDays === null
              ? `${summary.cansRestocked} cans added`
              : `${summary.cansRestocked} cans · every ${summary.avgRestockIntervalDays.toFixed(1)} days`
          }
        />
        <StatTile
          label="Untracked dispenses"
          value={summary.untrackedDispenses}
          sub="Dispensed after stock hit zero, so restocks were under-logged"
        />
      </StatGrid>

      <ChartCard
        title="Stock levels"
        subtitle={`Cans in each slot at the end of each ${bucket}, rebuilt from restocks and dispenses. Drops are dispenses, jumps are restocks.`}
        table={
          <DataTable
            columns={[
              { header: bucket, accessor: (r: (typeof series)[number]) => formatBucketLong(r.bucket, bucket), headerClassName: "capitalize" },
              ...data.slots.map((s) => ({
                header: `Slot ${s.position}`,
                accessor: (r: (typeof series)[number]) => {
                  const v = (r as Record<string, unknown>)[String(s.position)];
                  return v === null || v === undefined ? "—" : String(v);
                },
                className: "text-right tabular-nums",
                headerClassName: "text-right",
              })),
            ]}
            data={[...series].reverse()}
            keyFn={(r) => r.bucket}
          />
        }
      >
        {!hasLevels ? (
          <NoData message="No restocks recorded yet, so stock levels are unknown." />
        ) : (
          <SlotPanels data={data} />
        )}
      </ChartCard>

      <Card>
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
          Slots
        </p>
        <DataTable columns={slotColumns} data={data.slots} keyFn={(s) => String(s.position)} emptyMessage="No slots configured." />
        {tracked.length < data.slots.length && (
          <p className="mt-4 text-xs text-text-tertiary">
            Slots that have never been restocked have no known stock, so they don't count toward time empty.
          </p>
        )}
      </Card>

      <Card>
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
          Restock log
        </p>
        <DataTable
          columns={[
            { header: "When", accessor: (r) => formatDateTime(r.timestamp), className: "whitespace-nowrap" },
            { header: "Slots", accessor: (r) => r.slots, className: "text-right tabular-nums", headerClassName: "text-right" },
            { header: "Cans", accessor: (r) => r.cans, className: "text-right tabular-nums", headerClassName: "text-right" },
            { header: "Since previous", accessor: (r) => days(r.daysSincePrevious), className: "text-right tabular-nums", headerClassName: "text-right" },
          ]}
          data={data.restocks}
          keyFn={(r) => r.id}
          emptyMessage="No restocks in this range."
        />
      </Card>
    </>
  );
}

/**
 * One small chart per slot on a shared time axis and shared y scale, so each
 * slot reads on its own and the panels still compare directly. Hovering one
 * panel moves the crosshair on all of them.
 */
function SlotPanels({ data }: { data: InventoryAnalytics }) {
  const bucket = data.range.bucket;
  const yMax = Math.max(1, ...data.slots.map((s) => s.capacity));

  return (
    <div>
      <div className="flex flex-wrap gap-x-4 gap-y-1.5 mb-4 text-xs text-text-secondary">
        <span className="flex items-center gap-1.5">
          <span aria-hidden className="inline-block w-3 h-0.5 rounded-full" style={{ background: ACCENT }} />
          Cans in slot
        </span>
        <span className="flex items-center gap-1.5">
          <span aria-hidden className="inline-block w-3 border-t border-dashed border-status-low" />
          Low-stock threshold
        </span>
        <span className="flex items-center gap-1.5">
          <span aria-hidden className="inline-block w-3 h-2.5 rounded-sm bg-status-empty-bg border border-status-empty/30" />
          Empty
        </span>
      </div>
      <div className="divide-y divide-border-primary">
        {data.slots.map((s, i) => {
          const key = String(s.position);
          const points = data.series.map((pt) => {
            const level = pt.levels[key] ?? null;
            // Full-height band wherever the slot ended the bucket empty.
            return { bucket: pt.bucket, level, empty: level === 0 ? yMax : 0 };
          });
          const last = i === data.slots.length - 1;
          return (
            <div key={key} className="py-3 first:pt-0">
              <div className="flex items-baseline justify-between gap-3 mb-1">
                <p className="text-sm font-medium text-text-primary">{slotLabel(s)}</p>
                <p className="text-xs text-text-secondary tabular-nums">
                  {s.currentStock === null ? "Never restocked" : `${s.currentStock} of ${s.capacity} now`}
                </p>
              </div>
              <ResponsiveContainer width="100%" height={last ? 96 : 72}>
                <ComposedChart
                  data={points}
                  syncId="slot-stock"
                  margin={{ top: 4, right: 4, left: -20, bottom: 0 }}
                >
                  <XAxis
                    dataKey="bucket"
                    hide={!last}
                    tickFormatter={(v: string) => formatBucket(v, bucket)}
                    minTickGap={24}
                    {...axisProps}
                  />
                  <YAxis
                    domain={[0, yMax]}
                    ticks={[0, yMax]}
                    allowDecimals={false}
                    {...axisProps}
                  />
                  <Tooltip
                    cursor={{ stroke: "var(--color-border-secondary)" }}
                    content={({ active, payload, label }) => {
                      if (!active || !payload?.length) return null;
                      const v = (payload[0].payload as { level: number | null }).level;
                      return (
                        <div className="rounded-xl border border-border-primary bg-surface-primary px-3 py-2 shadow-subtle text-xs">
                          <p className="font-medium text-text-primary">
                            End of {formatBucketLong(String(label), bucket)}
                          </p>
                          <p className="text-text-secondary">
                            {v === null ? "Not restocked yet" : v === 0 ? "Empty" : `${v} of ${s.capacity} cans`}
                          </p>
                        </div>
                      );
                    }}
                  />
                  <Area
                    isAnimationActive={false}
                    dataKey="empty"
                    type="stepAfter"
                    stroke="none"
                    fill="var(--color-status-empty-bg)"
                    fillOpacity={1}
                    activeDot={false}
                  />
                  {s.lowThreshold > 0 && (
                    <ReferenceLine
                      y={s.lowThreshold}
                      stroke="var(--color-status-low)"
                      strokeOpacity={0.5}
                      strokeDasharray="3 3"
                    />
                  )}
                  <Area
                    isAnimationActive={false}
                    dataKey="level"
                    type="stepAfter"
                    stroke={ACCENT}
                    strokeWidth={2}
                    fill="var(--color-accent-soft)"
                    fillOpacity={1}
                    connectNulls={false}
                    activeDot={{ r: 3, stroke: "#ffffff", strokeWidth: 2 }}
                  />
                </ComposedChart>
              </ResponsiveContainer>
            </div>
          );
        })}
      </div>
    </div>
  );
}
