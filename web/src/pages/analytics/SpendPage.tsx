import { useQuery } from "@tanstack/react-query";
import {
  Area,
  AreaChart,
  Bar,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { listProducts } from "../../api/client";
import type { SpendAnalytics } from "../../api/types";
import type { Comparison } from "./analytics";
import { DataTable } from "../../components/DataTable";
import {
  ACCENT,
  axisProps,
  productColorMap,
  formatBucket,
  formatBucketLong,
  money,
  percent,
  useAnalytics,
  useComparison,
} from "./analytics";
import {
  COMPARE_COLOR,
  ChartCard,
  ChartTooltip,
  ComparisonNotice,
  Delta,
  Legend,
  Loaded,
  NoData,
  StatGrid,
  StatTile,
  Swatch,
} from "./components";

export function SpendPage() {
  const query = useAnalytics("spend");
  const cmp = useComparison("spend", query.data);
  const catalog = useQuery({ queryKey: ["products"], queryFn: listProducts });
  return (
    <Loaded query={query}>
      {(data) => (
        <Spend
          data={data}
          cmp={cmp}
          colors={productColorMap(
            catalog.data ?? [],
            data.byProduct.map((p) => p.productId),
          )}
        />
      )}
    </Loaded>
  );
}

function Spend({
  data,
  cmp,
  colors,
}: {
  data: SpendAnalytics;
  cmp: Comparison<SpendAnalytics> | null;
  colors: Record<string, string>;
}) {
  const bucket = data.range.bucket;
  const c = cmp?.data;
  const series = data.series.map((pt, i) => {
    const o = cmp?.at(cmp.data.series, data.series, i);
    return { ...pt, compare: o?.spend ?? null, compareCumulative: o?.cumulative ?? null };
  });
  const compareLegend = (label: string, current: string) =>
    cmp && (
      <Legend
        items={[
          { key: "current", label: current, color: ACCENT },
          { key: "compare", label, color: COMPARE_COLOR, line: true },
        ]}
      />
    );
  const byProductCompare = new Map(c?.byProduct.map((p) => [p.productId, p.spend]));
  const moneyAxis = (v: number) => `$${v}`;

  return (
    <>
      <ComparisonNotice range={data.range} />
      <StatGrid>
        <StatTile
          label="Cost"
          value={money(data.total)}
          compare={c && <Delta current={data.total} previous={c.total} format={money} />}
        />
        <StatTile
          label="Projected monthly"
          value={money(data.projectedMonthly)}
          sub={`At ${money(data.perDay)} per day`}
          compare={c && <Delta current={data.projectedMonthly} previous={c.projectedMonthly} format={money} />}
        />
        <StatTile
          label="Average per can"
          value={data.count ? money(data.avgCostPerCan) : "—"}
          sub={`${data.count} dispense${data.count === 1 ? "" : "s"}`}
          compare={c && (c.count ? <Delta current={data.count ? data.avgCostPerCan : null} previous={c.avgCostPerCan} format={money} /> : "No dispenses")}
        />
        <StatTile
          label="Restocked cost"
          value={money(data.restockedValue)}
          sub="Cans restocked, at current product cost"
          compare={c && <Delta current={data.restockedValue} previous={c.restockedValue} format={money} />}
        />
      </StatGrid>

      {data.unpricedDispenses > 0 && (
        <p className="text-sm text-status-low bg-status-low-bg rounded-xl px-4 py-3">
          ⚠ {data.unpricedDispenses} dispense{data.unpricedDispenses === 1 ? " has" : "s have"} no
          cost because the product has no cost per can. Set one in Setup to include them.
        </p>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ChartCard
          title="Cost over time"
          subtitle={`Per ${bucket}`}
          legend={compareLegend("Comparison", "This range")}
          table={
            <DataTable
              columns={[
                { header: bucket, accessor: (r) => formatBucketLong(r.bucket, bucket), headerClassName: "capitalize" },
                { header: "Dispenses", accessor: (r) => r.count, className: "text-right tabular-nums", headerClassName: "text-right" },
                { header: "Cost", accessor: (r) => money(r.spend), className: "text-right tabular-nums", headerClassName: "text-right" },
                ...(cmp ? [{ header: "Comparison", accessor: (r: (typeof series)[number]) => (r.compare === null ? "—" : money(r.compare)), className: "text-right tabular-nums", headerClassName: "text-right" }] : []),
              ]}
              data={[...series].reverse()}
              keyFn={(r) => r.bucket}
            />
          }
        >
          {data.count === 0 && !c?.count ? (
            <NoData message="No dispenses in this range." />
          ) : (
            <ResponsiveContainer width="100%" height={240}>
              <ComposedChart data={series} margin={{ top: 4, right: 4, left: -12, bottom: 0 }}>
                <CartesianGrid vertical={false} />
                <XAxis dataKey="bucket" tickFormatter={(v: string) => formatBucket(v, bucket)} minTickGap={24} {...axisProps} />
                <YAxis tickFormatter={moneyAxis} {...axisProps} />
                <Tooltip
                  cursor={{ fill: "var(--color-surface-tertiary)" }}
                  content={(p) => <ChartTooltip {...p} title={(l) => formatBucketLong(l, bucket)} value={money} />}
                />
                <Bar dataKey="spend" name="Cost" isAnimationActive={false} fill={ACCENT} radius={[4, 4, 0, 0]} maxBarSize={28} />
                {cmp && (
                  <Line dataKey="compare" name="Comparison" isAnimationActive={false} stroke={COMPARE_COLOR} strokeWidth={2} strokeDasharray="5 4" dot={false} connectNulls type="monotone" />
                )}
              </ComposedChart>
            </ResponsiveContainer>
          )}
        </ChartCard>

        <ChartCard
          title="Running total"
          subtitle="Cumulative cost across the range"
          legend={compareLegend("Comparison", "This range")}
          table={
            <DataTable
              columns={[
                { header: bucket, accessor: (r) => formatBucketLong(r.bucket, bucket), headerClassName: "capitalize" },
                { header: "Running total", accessor: (r) => money(r.cumulative), className: "text-right tabular-nums", headerClassName: "text-right" },
                ...(cmp ? [{ header: "Comparison", accessor: (r: (typeof series)[number]) => (r.compareCumulative === null ? "—" : money(r.compareCumulative)), className: "text-right tabular-nums", headerClassName: "text-right" }] : []),
              ]}
              data={[...series].reverse()}
              keyFn={(r) => r.bucket}
            />
          }
        >
          {data.count === 0 && !c?.count ? (
            <NoData message="No dispenses in this range." />
          ) : (
            <ResponsiveContainer width="100%" height={240}>
              <AreaChart data={series} margin={{ top: 4, right: 4, left: -12, bottom: 0 }}>
                <CartesianGrid vertical={false} />
                <XAxis dataKey="bucket" tickFormatter={(v: string) => formatBucket(v, bucket)} minTickGap={24} {...axisProps} />
                <YAxis tickFormatter={moneyAxis} {...axisProps} />
                <Tooltip content={(p) => <ChartTooltip {...p} title={(l) => formatBucketLong(l, bucket)} value={money} />} />
                <Area
                  isAnimationActive={false}
                  dataKey="cumulative"
                  name="Running total"
                  type="monotone"
                  stroke={ACCENT}
                  strokeWidth={2}
                  fill="var(--color-accent-soft)"
                  fillOpacity={1}
                />
                {cmp && (
                  <Area
                    isAnimationActive={false}
                    dataKey="compareCumulative"
                    name="Comparison"
                    type="monotone"
                    stroke={COMPARE_COLOR}
                    strokeWidth={2}
                    strokeDasharray="5 4"
                    fill="none"
                    connectNulls
                  />
                )}
              </AreaChart>
            </ResponsiveContainer>
          )}
        </ChartCard>
      </div>

      <ChartCard title="Cost by product" subtitle="Share of cost in this range">
        <DataTable
          columns={[
            {
              header: "Product",
              accessor: (r) => (
                <span className="font-medium text-text-primary">
                  <Swatch color={colors[r.productId]} />
                  {r.name}
                </span>
              ),
            },
            { header: "Dispenses", accessor: (r) => r.count, className: "text-right tabular-nums", headerClassName: "text-right" },
            {
              header: "Share",
              accessor: (r) => (
                <div className="flex items-center justify-end gap-2">
                  <div className="hidden sm:block w-32 h-1.5 rounded-full bg-surface-tertiary overflow-hidden">
                    <div className="h-full rounded-full" style={{ width: percent(r.share), background: colors[r.productId] }} />
                  </div>
                  <span className="tabular-nums w-9 text-right">{percent(r.share)}</span>
                </div>
              ),
              headerClassName: "text-right",
            },
            { header: "Cost", accessor: (r) => money(r.spend), className: "text-right tabular-nums", headerClassName: "text-right" },
            ...(c
              ? [{
                  header: "Comparison",
                  accessor: (r: SpendAnalytics["byProduct"][number]) => {
                    const v = byProductCompare.get(r.productId);
                    return v === undefined ? "—" : <Delta current={r.spend} previous={v} format={money} />;
                  },
                  className: "text-right tabular-nums whitespace-nowrap",
                  headerClassName: "text-right",
                }]
              : []),
          ]}
          data={data.byProduct}
          keyFn={(r) => r.productId}
          emptyMessage="No dispenses in this range."
        />
      </ChartCard>
    </>
  );
}
