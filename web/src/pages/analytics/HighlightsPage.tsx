import { useQuery } from "@tanstack/react-query";
import { listProducts } from "../../api/client";
import type { HighlightsAnalytics } from "../../api/types";
import type { Comparison } from "./analytics";
import { DataTable } from "../../components/DataTable";
import {
  formatHour,
  formatHours,
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
  const { firstCan, lastCan, busiestDay, busiestHour, longestGap } = data;

  return (
    <>
      <ComparisonNotice range={data.range} />
      <StatGrid>
        <StatTile
          label="First can"
          value={formatMinuteOfDay(firstCan.avgMinute)}
          sub={`On average. Earliest: ${clock(firstCan.record)}, ${day(firstCan.record)}`}
          compare={was(c?.firstCan && formatMinuteOfDay(c.firstCan.avgMinute))}
        />
        <StatTile
          label="Last can"
          value={lastCan ? formatMinuteOfDay(lastCan.avgMinute) : "—"}
          sub={lastCan && `On average. Latest: ${clock(lastCan.record)}, ${day(lastCan.record)}`}
          compare={was(c?.lastCan && formatMinuteOfDay(c.lastCan.avgMinute))}
        />
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
      </StatGrid>
      <StatGrid>
        <StatTile
          label="Weekday streak"
          value={data.currentStreak}
          sub={`Weekdays in a row with a dispense. Longest: ${data.longestStreak}`}
          compare={c && `Longest then: ${c.longestStreak}`}
        />
        <StatTile
          label="Longest dry spell"
          value={longestGap ? formatHours(longestGap.hours) : "—"}
          compare={was(c?.longestGap && formatHours(c.longestGap.hours))}
          sub={
            longestGap &&
            `${day(longestGap.from)} ${clock(longestGap.from)} to ${day(longestGap.to)} ${clock(longestGap.to)}`
          }
        />
        <StatTile
          label="Days with dispenses"
          value={data.activeDays}
          sub={`${data.perActiveDay.toFixed(1)} cans on each`}
          compare={c && <Delta current={data.activeDays} previous={c.activeDays} />}
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
