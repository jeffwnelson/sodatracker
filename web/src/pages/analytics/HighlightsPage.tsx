import { useQuery } from "@tanstack/react-query";
import { listProducts } from "../../api/client";
import type { HighlightsAnalytics } from "../../api/types";
import type { Comparison } from "./analytics";
import { DataTable } from "../../components/DataTable";
import {
  formatHour,
  formatMinuteOfDay,
  productColorMap,
  useAnalytics,
  useComparison,
} from "./analytics";
import {
  BreakdownPie,
  ChartCard,
  ComparisonNotice,
  Delta,
  Loaded,
  NoData,
  StatGrid,
  StatTile,
} from "./components";

export function HighlightsPage() {
  const query = useAnalytics("highlights");
  const cmp = useComparison("highlights", query.data);
  return <Loaded query={query}>{(data) => <Highlights data={data} cmp={cmp} />}</Loaded>;
}

function day(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
  });
}

function clock(iso: string): string {
  const d = new Date(iso);
  return formatMinuteOfDay(d.getHours() * 60 + d.getMinutes());
}

function Highlights({
  data,
  cmp,
}: {
  data: HighlightsAnalytics;
  cmp: Comparison<HighlightsAnalytics> | null;
}) {
  const c = cmp?.data;
  // Highlights of a range with no dispenses are all empty.
  const was = (v: string | null | undefined) => (c ? (c.firstCan && v ? `was ${v}` : "No dispenses") : undefined);
  // Colors key off the full catalog so a product keeps its hue across tabs.
  const catalog = useQuery({ queryKey: ["products"], queryFn: listProducts });
  const colors = productColorMap(
    catalog.data ?? [],
    data.firstCanProducts.map((p) => p.productId),
  );

  if (!data.firstCan) {
    return <NoData message="No dispenses in this range." />;
  }
  const { firstCan, lastCan, busiestDay, busiestHour } = data;
  // The range's earliest and latest cans are the records among each day's
  // first and last cans.

  return (
    <>
      <ComparisonNotice range={data.range} />
      <StatGrid>
        <StatTile
          label="First can average"
          value={formatMinuteOfDay(firstCan.avgMinute)}
          sub="Average time of each day's first can"
          compare={was(c?.firstCan && formatMinuteOfDay(c.firstCan.avgMinute))}
        />
        <StatTile
          label="Last can average"
          value={lastCan ? formatMinuteOfDay(lastCan.avgMinute) : "—"}
          sub="Average time of each day's last can"
          compare={was(c?.lastCan && formatMinuteOfDay(c.lastCan.avgMinute))}
        />
        <StatTile
          label="Earliest can"
          value={clock(firstCan.record)}
          sub={day(firstCan.record)}
          compare={was(c?.firstCan && `${clock(c.firstCan.record)}, ${day(c.firstCan.record)}`)}
        />
        <StatTile
          label="Latest can"
          value={lastCan ? clock(lastCan.record) : "—"}
          sub={lastCan && day(lastCan.record)}
          compare={was(c?.lastCan && `${clock(c.lastCan.record)}, ${day(c.lastCan.record)}`)}
        />
      </StatGrid>
      <StatGrid cols={5}>
        <StatTile
          label="Busiest day"
          value={busiestDay?.count ?? "—"}
          sub={busiestDay && `Cans on ${day(busiestDay.at)}`}
          compare={was(c?.busiestDay && `${c.busiestDay.count} on ${day(c.busiestDay.at)}`)}
        />
        <StatTile
          label="Busiest hour"
          value={busiestHour?.count ?? "—"}
          sub={
            busiestHour &&
            `Cans from ${formatHour(new Date(busiestHour.at).getHours())}, ${day(busiestHour.at)}`
          }
          compare={was(c?.busiestHour && `${c.busiestHour.count} from ${formatHour(new Date(c.busiestHour.at).getHours())}`)}
        />
        <StatTile
          label="Soda consumed"
          value={`${Math.round(data.ounces).toLocaleString()} oz`}
          sub={`${(data.ounces / 128).toFixed(1)} gallons`}
          compare={c && <Delta current={data.ounces} previous={c.ounces} format={(n) => `${Math.round(n).toLocaleString()} oz`} />}
        />
        <StatTile
          label="Caffeine"
          value={data.caffeineMg >= 10_000 ? `${(data.caffeineMg / 1000).toFixed(1)} g` : `${Math.round(data.caffeineMg).toLocaleString()} mg`}
          sub={`About ${Math.round(data.caffeineMg / 95).toLocaleString()} cups of coffee`}
          compare={c && <Delta current={data.caffeineMg} previous={c.caffeineMg} format={(n) => `${Math.round(n).toLocaleString()} mg`} />}
        />
        <StatTile
          label="Aluminum to recycle"
          value={`${data.aluminumLb.toFixed(1)} lb`}
          compare={c && <Delta current={data.aluminumLb} previous={c.aluminumLb} format={(n) => `${n.toFixed(1)} lb`} />}
        />
      </StatGrid>

      <ChartCard
        title="Who opens the day"
        subtitle="Product of each day's first can"
        table={
          <DataTable
            columns={[
              { header: "Product", accessor: (r) => r.name },
              {
                header: "Days",
                accessor: (r) => r.count,
                className: "text-right tabular-nums",
                headerClassName: "text-right",
              },
            ]}
            data={data.firstCanProducts}
            keyFn={(r) => r.productId}
          />
        }
      >
        <BreakdownPie
          slices={data.firstCanProducts.map((p) => ({
            key: p.productId,
            label: p.name,
            value: p.count,
            color: colors[p.productId],
          }))}
          format={(n) => `${n} ${n === 1 ? "day" : "days"}`}
        />
      </ChartCard>
    </>
  );
}
