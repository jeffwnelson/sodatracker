import { useQuery } from "@tanstack/react-query";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { listProducts } from "../../api/client";
import type { SpendAnalytics } from "../../api/types";
import { DataTable } from "../../components/DataTable";
import {
  ACCENT,
  axisProps,
  colorMap,
  formatBucket,
  formatBucketLong,
  money,
  percent,
  useAnalytics,
} from "./analytics";
import {
  ChartCard,
  ChartTooltip,
  Delta,
  Loaded,
  NoData,
  StatGrid,
  StatTile,
  Swatch,
} from "./components";

export function SpendPage() {
  const query = useAnalytics("spend");
  const catalog = useQuery({ queryKey: ["products"], queryFn: listProducts });
  return (
    <Loaded query={query}>
      {(data) => (
        <Spend
          data={data}
          colors={colorMap([
            ...(catalog.data ?? []).map((p) => p.id),
            ...data.byProduct.map((p) => p.productId),
          ])}
        />
      )}
    </Loaded>
  );
}

function Spend({
  data,
  colors,
}: {
  data: SpendAnalytics;
  colors: Record<string, string>;
}) {
  const bucket = data.range.bucket;
  const moneyAxis = (v: number) => `$${v}`;

  return (
    <>
      <StatGrid>
        <StatTile
          label="Spend"
          value={money(data.total)}
          sub={<Delta current={data.total} previous={data.previousTotal} format={money} />}
        />
        <StatTile
          label="Projected monthly"
          value={money(data.projectedMonthly)}
          sub={`At ${money(data.perDay)} per day`}
        />
        <StatTile
          label="Average per can"
          value={data.count ? money(data.avgCostPerCan) : "—"}
          sub={`${data.count} dispense${data.count === 1 ? "" : "s"}`}
        />
        <StatTile
          label="Restocked value"
          value={money(data.restockedValue)}
          sub="Cans restocked, at current product cost"
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
          title="Spend over time"
          subtitle={`Per ${bucket}`}
          table={
            <DataTable
              columns={[
                { header: bucket, accessor: (r) => formatBucketLong(r.bucket, bucket), headerClassName: "capitalize" },
                { header: "Dispenses", accessor: (r) => r.count, className: "text-right tabular-nums", headerClassName: "text-right" },
                { header: "Spend", accessor: (r) => money(r.spend), className: "text-right tabular-nums", headerClassName: "text-right" },
              ]}
              data={[...data.series].reverse()}
              keyFn={(r) => r.bucket}
            />
          }
        >
          {data.count === 0 ? (
            <NoData message="No dispenses in this range." />
          ) : (
            <ResponsiveContainer width="100%" height={240}>
              <BarChart data={data.series} margin={{ top: 4, right: 4, left: -12, bottom: 0 }}>
                <CartesianGrid vertical={false} />
                <XAxis dataKey="bucket" tickFormatter={(v: string) => formatBucket(v, bucket)} minTickGap={24} {...axisProps} />
                <YAxis tickFormatter={moneyAxis} {...axisProps} />
                <Tooltip
                  cursor={{ fill: "var(--color-surface-tertiary)" }}
                  content={(p) => <ChartTooltip {...p} title={(l) => formatBucketLong(l, bucket)} value={money} />}
                />
                <Bar dataKey="spend" name="Spend" isAnimationActive={false} fill={ACCENT} radius={[4, 4, 0, 0]} maxBarSize={28} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </ChartCard>

        <ChartCard
          title="Running total"
          subtitle="Cumulative spend across the range"
          table={
            <DataTable
              columns={[
                { header: bucket, accessor: (r) => formatBucketLong(r.bucket, bucket), headerClassName: "capitalize" },
                { header: "Running total", accessor: (r) => money(r.cumulative), className: "text-right tabular-nums", headerClassName: "text-right" },
              ]}
              data={[...data.series].reverse()}
              keyFn={(r) => r.bucket}
            />
          }
        >
          {data.count === 0 ? (
            <NoData message="No dispenses in this range." />
          ) : (
            <ResponsiveContainer width="100%" height={240}>
              <AreaChart data={data.series} margin={{ top: 4, right: 4, left: -12, bottom: 0 }}>
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
              </AreaChart>
            </ResponsiveContainer>
          )}
        </ChartCard>
      </div>

      <ChartCard title="Spend by product" subtitle="Share of spend in this range">
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
            { header: "Spend", accessor: (r) => money(r.spend), className: "text-right tabular-nums", headerClassName: "text-right" },
          ]}
          data={data.byProduct}
          keyFn={(r) => r.productId}
          emptyMessage="No dispenses in this range."
        />
      </ChartCard>
    </>
  );
}
