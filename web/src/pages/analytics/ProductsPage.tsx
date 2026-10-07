import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { listProducts } from "../../api/client";
import type { ProductAnalytics, ProductStat } from "../../api/types";
import type { Comparison } from "./analytics";
import { Card } from "../../components/Card";
import { DataTable } from "../../components/DataTable";
import type { Column } from "../../components/DataTable";
import {
  SERIES,
  axisProps,
  productColorMap,
  formatBucket,
  formatBucketLong,
  formatDateTime,
  formatHour,
  money,
  percent,
  sumHeatmaps,
  useAnalytics,
  useComparison,
} from "./analytics";
import {
  ChartCard,
  ComparisonNotice,
  CompareDot,
  ChartTooltip,
  Legend,
  Loaded,
  NoData,
  Swatch,
} from "./components";
import { Heatmap, HeatmapTable } from "./Heatmap";

export function ProductsPage() {
  const query = useAnalytics("products");
  const cmp = useComparison("products", query.data);
  // Colors key off the full catalog so a product keeps its hue across ranges.
  const catalog = useQuery({ queryKey: ["products"], queryFn: listProducts });
  return (
    <Loaded query={query}>
      {(data) => (
        <Products
          data={data}
          cmp={cmp}
          colors={productColorMap(
            catalog.data ?? [],
            data.products.map((p) => p.productId),
          )}
        />
      )}
    </Loaded>
  );
}

/** Rank against the comparison range: "New" if it had no dispenses there. */
function RankChange({ p, before }: { p: ProductStat; before?: ProductStat }) {
  if (!before || before.count === 0) {
    return p.count > 0 ? <span>New</span> : <span className="text-text-tertiary">—</span>;
  }
  if (p.count === 0) return <span className="text-text-tertiary">None now</span>;
  const diff = before.rank - p.rank;
  if (diff === 0) return <span className="text-text-tertiary">Same</span>;
  return (
    <span>
      {diff > 0 ? "▲" : "▼"} {Math.abs(diff)}
    </span>
  );
}

function Products({
  data,
  cmp,
  colors,
}: {
  data: ProductAnalytics;
  cmp: Comparison<ProductAnalytics> | null;
  colors: Record<string, string>;
}) {
  const before = new Map(cmp?.data.products.map((p) => [p.productId, p]));
  const bucket = data.range.bucket;
  const used = data.products.filter((p) => p.count > 0);
  const total = used.reduce((n, p) => n + p.count, 0);
  const slow = data.products.filter((p) => p.slots.length > 0 && p.share < 0.05);
  const names = Object.fromEntries(data.products.map((p) => [p.productId, p.name]));
  // Stack and legend follow palette order, so neighbouring segments are the
  // pairs the palette was validated for.
  const slot = (id: string) => SERIES.indexOf(colors[id]);
  const stackIds = used
    .map((p) => p.productId)
    .sort((a, b) => slot(a) - slot(b));
  const legend = (
    <Legend
      items={stackIds.map((id) => ({ key: id, label: names[id], color: colors[id] }))}
    />
  );

  // An empty selection means every product.
  const [picked, setPicked] = useState<string[]>([]);
  const shown = picked.filter((id) => data.heatmaps[id]);
  const heatmap = sumHeatmaps(
    (shown.length ? shown : stackIds).map((id) => data.heatmaps[id]).filter(Boolean),
  );
  const toggle = (id: string) =>
    setPicked((ids) => (ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id]));
  const chip = (on: boolean) =>
    `inline-flex items-center rounded-full border px-3 py-1.5 sm:py-1 text-xs font-medium transition-colors duration-150 ${
      on
        ? "border-accent bg-accent-soft text-text-primary"
        : "border-border-primary text-text-tertiary hover:text-text-primary"
    }`;
  const productFilter = (
    <div className="flex flex-wrap gap-1.5" role="group" aria-label="Filter heatmap by product">
      <button type="button" onClick={() => setPicked([])} aria-pressed={shown.length === 0} className={chip(shown.length === 0)}>
        All products
      </button>
      {stackIds.map((id) => (
        <button key={id} type="button" onClick={() => toggle(id)} aria-pressed={shown.includes(id)} className={chip(shown.includes(id))}>
          <Swatch color={colors[id]} />
          {names[id]}
        </button>
      ))}
    </div>
  );

  const series = data.series.map((pt) => ({ bucket: pt.bucket, ...pt.counts }));
  const dayparts = data.dayparts.map((dp) => ({
    name: dp.name,
    hours: `${formatHour(dp.startHour)}–${formatHour(dp.endHour)}`,
    total: dp.total,
    ...dp.counts,
  }));

  const columns: Column<ProductStat>[] = [
    { header: "#", accessor: (p) => <span className="text-text-tertiary">{p.rank}</span>, className: "w-8" },
    {
      header: "Product",
      accessor: (p) => (
        <span className="font-medium text-text-primary">
          <Swatch color={colors[p.productId]} />
          {p.name}
        </span>
      ),
    },
    { header: "Dispenses", accessor: (p) => p.count, className: "text-right tabular-nums", headerClassName: "text-right" },
    {
      header: "Share",
      accessor: (p) => (
        <div className="flex items-center justify-end gap-2">
          <div className="hidden sm:block w-20 h-1.5 rounded-full bg-surface-tertiary overflow-hidden">
            <div className="h-full rounded-full" style={{ width: percent(p.share), background: colors[p.productId] }} />
          </div>
          <span className="tabular-nums w-9 text-right">{percent(p.share)}</span>
        </div>
      ),
      headerClassName: "text-right",
    },
    ...(cmp
      ? [
          {
            header: "Comparison",
            accessor: (p: ProductStat) => before.get(p.productId)?.count ?? 0,
            className: "text-right tabular-nums",
            headerClassName: "text-right",
          },
          {
            header: "Rank change",
            accessor: (p: ProductStat) => <RankChange p={p} before={before.get(p.productId)} />,
            className: "text-right whitespace-nowrap",
            headerClassName: "text-right",
          },
        ]
      : []),
    { header: "Cost", accessor: (p) => money(p.cost), className: "text-right tabular-nums", headerClassName: "text-right" },
    { header: "Slots", accessor: (p) => (p.slots.length ? p.slots.join(", ") : "—"), className: "text-right", headerClassName: "text-right" },
    {
      header: "Last dispensed",
      accessor: (p) => (p.lastDispensed ? formatDateTime(p.lastDispensed) : "—"),
      className: "text-right whitespace-nowrap",
      headerClassName: "text-right",
    },
  ];

  return (
    <>
      <ComparisonNotice range={data.range} />
      <Card>
        <div className="flex flex-wrap items-baseline justify-between gap-2 mb-4">
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
            Product ranking
          </p>
          {cmp && (
            <p className="flex items-center gap-1.5 text-xs text-text-tertiary">
              <CompareDot />
              Comparison and rank change are against the comparison range
            </p>
          )}
        </div>
        <DataTable columns={columns} data={data.products} keyFn={(p) => p.productId} emptyMessage="No products stocked or dispensed." />
        {slow.length > 0 && (
          <p className="mt-4 text-sm text-text-secondary">
            <span className="font-medium text-text-primary">Slow movers:</span>{" "}
            {slow.map((p) => p.name).join(", ")} {slow.length === 1 ? "is" : "are"} stocked
            but made up under 5% of dispenses in this range.
          </p>
        )}
      </Card>

      <ChartCard
        title="Product mix over time"
        subtitle={`Dispenses per ${bucket}, stacked by product`}
        legend={legend}
        table={
          <DataTable
            columns={[
              { header: bucket, accessor: (r: (typeof series)[number]) => formatBucketLong(r.bucket, bucket), headerClassName: "capitalize" },
              ...stackIds.map((id) => ({
                header: names[id],
                accessor: (r: (typeof series)[number]) => (r as Record<string, unknown>)[id] as number ?? 0,
                className: "text-right tabular-nums",
                headerClassName: "text-right",
              })),
            ]}
            data={[...series].reverse()}
            keyFn={(r) => r.bucket}
          />
        }
      >
        {total === 0 ? (
          <NoData message="No dispenses in this range." />
        ) : (
          <ResponsiveContainer width="100%" height={280}>
            <BarChart data={series} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
              <CartesianGrid vertical={false} />
              <XAxis dataKey="bucket" tickFormatter={(v: string) => formatBucket(v, bucket)} minTickGap={24} {...axisProps} />
              <YAxis allowDecimals={false} {...axisProps} />
              <Tooltip
                cursor={{ fill: "var(--color-surface-tertiary)" }}
                content={(p) => <ChartTooltip {...p} title={(l) => formatBucketLong(l, bucket)} />}
              />
              {stackIds.map((id, i) => (
                <Bar
                  isAnimationActive={false}
                  key={id}
                  dataKey={id}
                  name={names[id]}
                  stackId="products"
                  fill={colors[id]}
                  // A 2px surface-colored stroke leaves a gap between segments.
                  stroke="#ffffff"
                  strokeWidth={2}
                  maxBarSize={28}
                  radius={i === stackIds.length - 1 ? [4, 4, 0, 0] : undefined}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        )}
      </ChartCard>

      <ChartCard
        title="Time of day"
        subtitle="What people take in each part of the day"
        legend={legend}
        table={
          <DataTable
            columns={[
              { header: "Part of day", accessor: (r: (typeof dayparts)[number]) => `${r.name} (${r.hours})` },
              ...stackIds.map((id) => ({
                header: names[id],
                accessor: (r: (typeof dayparts)[number]) => (r as Record<string, unknown>)[id] as number ?? 0,
                className: "text-right tabular-nums",
                headerClassName: "text-right",
              })),
              { header: "Total", accessor: (r: (typeof dayparts)[number]) => r.total, className: "text-right tabular-nums", headerClassName: "text-right" },
            ]}
            data={dayparts}
            keyFn={(r) => r.name}
          />
        }
      >
        {total === 0 ? (
          <NoData message="No dispenses in this range." />
        ) : (
          <ResponsiveContainer width="100%" height={dayparts.length * 44 + 30}>
            <BarChart data={dayparts} layout="vertical" margin={{ top: 0, right: 8, left: 8, bottom: 0 }}>
              <CartesianGrid horizontal={false} />
              <XAxis type="number" allowDecimals={false} {...axisProps} />
              <YAxis
                type="category"
                dataKey="name"
                width={80}
                {...axisProps}
              />
              <Tooltip
                cursor={{ fill: "var(--color-surface-tertiary)" }}
                content={(p) => (
                  <ChartTooltip
                    {...p}
                    title={(l) => {
                      const dp = dayparts.find((d) => d.name === l);
                      return dp ? `${dp.name} · ${dp.hours}` : l;
                    }}
                  />
                )}
              />
              {stackIds.map((id, i) => (
                <Bar
                  isAnimationActive={false}
                  key={id}
                  dataKey={id}
                  name={names[id]}
                  stackId="products"
                  fill={colors[id]}
                  stroke="#ffffff"
                  strokeWidth={2}
                  barSize={22}
                  radius={i === stackIds.length - 1 ? [0, 4, 4, 0] : undefined}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        )}
      </ChartCard>

      <ChartCard
        title="When people grab each product"
        subtitle={
          shown.length
            ? `Dispenses of ${shown.map((id) => names[id]).join(", ")} by day of week and hour`
            : "Dispenses of every product by day of week and hour; pick products to narrow it"
        }
        controls={total > 0 && productFilter}
        table={<HeatmapTable heatmap={heatmap} />}
      >
        {total === 0 ? <NoData message="No dispenses in this range." /> : <Heatmap heatmap={heatmap} />}
      </ChartCard>
    </>
  );
}
