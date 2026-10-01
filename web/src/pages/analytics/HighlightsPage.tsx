import { useQuery } from "@tanstack/react-query";
import { listProducts } from "../../api/client";
import type { HighlightsAnalytics } from "../../api/types";
import { DataTable } from "../../components/DataTable";
import {
  formatHour,
  formatHours,
  formatMinuteOfDay,
  productColorMap,
  useAnalytics,
} from "./analytics";
import {
  BreakdownPie,
  ChartCard,
  Loaded,
  NoData,
  StatGrid,
  StatTile,
} from "./components";

export function HighlightsPage() {
  const query = useAnalytics("highlights");
  return <Loaded query={query}>{(data) => <Highlights data={data} />}</Loaded>;
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

function Highlights({ data }: { data: HighlightsAnalytics }) {
  // Colors key off the full catalog so a product keeps its hue across tabs.
  const catalog = useQuery({ queryKey: ["products"], queryFn: listProducts });
  const colors = productColorMap(
    catalog.data ?? [],
    data.firstCanProducts.map((p) => p.productId),
  );

  if (!data.firstCan) {
    return <NoData message="No dispenses in this range." />;
  }
  const { firstCan, lastCan, busiestDay, busiestHour, longestGap } = data;

  return (
    <>
      <StatGrid>
        <StatTile
          label="First can"
          value={formatMinuteOfDay(firstCan.avgMinute)}
          sub={`On average. Earliest: ${clock(firstCan.record)}, ${day(firstCan.record)}`}
        />
        <StatTile
          label="Last can"
          value={lastCan ? formatMinuteOfDay(lastCan.avgMinute) : "—"}
          sub={lastCan && `On average. Latest: ${clock(lastCan.record)}, ${day(lastCan.record)}`}
        />
        <StatTile
          label="Busiest day"
          value={busiestDay?.count ?? "—"}
          sub={busiestDay && `Cans on ${day(busiestDay.at)}`}
        />
        <StatTile
          label="Busiest hour"
          value={busiestHour?.count ?? "—"}
          sub={
            busiestHour &&
            `Cans from ${formatHour(new Date(busiestHour.at).getHours())}, ${day(busiestHour.at)}`
          }
        />
      </StatGrid>
      <StatGrid>
        <StatTile
          label="Weekday streak"
          value={data.currentStreak}
          sub={`Weekdays in a row with a sale. Longest: ${data.longestStreak}`}
        />
        <StatTile
          label="Longest dry spell"
          value={longestGap ? formatHours(longestGap.hours) : "—"}
          sub={
            longestGap &&
            `${day(longestGap.from)} ${clock(longestGap.from)} to ${day(longestGap.to)} ${clock(longestGap.to)}`
          }
        />
        <StatTile
          label="Days with sales"
          value={data.activeDays}
          sub={`${data.perActiveDay.toFixed(1)} cans on each`}
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
