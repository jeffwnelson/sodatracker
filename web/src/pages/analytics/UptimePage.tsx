import type { UptimeAnalytics } from "../../api/types";
import type { Comparison } from "./analytics";
import { Card } from "../../components/Card";
import { StatusOverview } from "./StatusOverview";
import { DataTable } from "../../components/DataTable";
import {
  OTHER_COLOR,
  formatDateTime,
  formatHours,
  useAnalytics,
  useComparison,
} from "./analytics";
import {
  BreakdownPie,
  ChartCard,
  Delta,
  Loaded,
  NoData,
  StatGrid,
  StatTile,
} from "./components";

const ONLINE = "var(--color-status-ok)";
const OFFLINE = "var(--color-status-empty)";
const UNTRACKED = OTHER_COLOR;

export function UptimePage() {
  const query = useAnalytics("uptime");
  const cmp = useComparison("uptime", query.data);
  return <Loaded query={query}>{(data) => <Uptime data={data} cmp={cmp} />}</Loaded>;
}

function Uptime({
  data,
  cmp,
}: {
  data: UptimeAnalytics;
  cmp: Comparison<UptimeAnalytics> | null;
}) {
  // Heartbeats have their own start, so tracking is judged by uptime itself
  // rather than the dispense-based notice the other tabs show.
  const c = cmp?.data;
  const pct = (n: number) => `${(n * 100).toFixed(1)}%`;
  if (!data.trackingSince) {
    return (
      <NoData message="No heartbeats from the Pi agent yet. They started with v0.4.0." />
    );
  }
  const statuses = [
    { key: "onlineHours", label: "Online", color: ONLINE },
    { key: "offlineHours", label: "Offline", color: OFFLINE },
    { key: "untrackedHours", label: "Not tracked yet", color: UNTRACKED },
  ] as const;
  // Time before the first heartbeat would swamp the pie for weeks after
  // tracking began; the bar chart still shows it.
  const tracked = statuses.filter((s) => s.key !== "untrackedHours");

  return (
    <>
      <StatusOverview />

      <p className="pt-2 text-xs font-medium uppercase tracking-wider text-text-tertiary">
        In the selected range
      </p>
      <StatGrid>
        <StatTile
          label="Uptime"
          value={data.uptime === null ? "—" : `${(data.uptime * 100).toFixed(1)}%`}
          sub="Of the tracked time in this range"
          compare={c && (c.uptime === null ? "Not tracked then" : <Delta current={data.uptime} previous={c.uptime} format={pct} />)}
        />
        <StatTile
          label="Outages"
          value={data.outages}
          sub={
            data.outages
              ? `Longest: ${formatHours(data.longestOutageHours)}`
              : "None in this range"
          }
          compare={c && (c.uptime === null ? "Not tracked then" : <Delta current={data.outages} previous={c.outages} />)}
        />
        <StatTile
          label="Time offline"
          value={data.offlineHours ? formatHours(data.offlineHours) : "0"}
          sub="Dispenses in this time weren't counted"
          compare={c && (c.uptime === null ? "Not tracked then" : <Delta current={data.offlineHours} previous={c.offlineHours} format={formatHours} />)}
        />
        <StatTile
          label="Tracked since"
          value={new Date(data.trackingSince).toLocaleDateString(undefined, {
            month: "short",
            day: "numeric",
          })}
          sub="First heartbeat from the Pi agent"
        />
      </StatGrid>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ChartCard
          title="Breakdown"
          subtitle="Tracked time in this range by agent status"
          table={
            <DataTable
              columns={[
                { header: "Status", accessor: (r) => r.label },
                {
                  header: "Time",
                  accessor: (r) => formatHours(data[r.key]),
                  className: "text-right tabular-nums",
                  headerClassName: "text-right",
                },
              ]}
              data={tracked}
              keyFn={(r) => r.key}
            />
          }
        >
          <BreakdownPie
            slices={tracked.map((s) => ({ ...s, value: data[s.key] }))}
            format={formatHours}
          />
        </ChartCard>

        <Card>
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
            Outages
          </p>
          {data.outageList.length === 0 ? (
            <NoData message="No outages in this range." />
          ) : (
            <DataTable
              columns={[
                { header: "From", accessor: (r) => formatDateTime(r.from) },
                {
                  header: "To",
                  accessor: (r) =>
                    r.ongoing ? (
                      <span className="text-status-empty font-medium">Still offline</span>
                    ) : (
                      formatDateTime(r.to)
                    ),
                },
                {
                  header: "Length",
                  accessor: (r) => formatHours(r.hours),
                  className: "text-right tabular-nums",
                  headerClassName: "text-right",
                },
              ]}
              data={data.outageList}
              keyFn={(r) => r.from}
            />
          )}
        </Card>
      </div>

    </>
  );
}
