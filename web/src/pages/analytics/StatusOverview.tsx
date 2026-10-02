import { useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getAnalytics, getMachineStatus, getSlackSettings } from "../../api/client";
import type { UptimeAnalytics } from "../../api/types";
import { useActiveAlerts } from "../../api/machine";
import { useBackupStatus } from "../../api/backup";
import { Card } from "../../components/Card";
import {
  addDays,
  addMonths,
  dayKey,
  formatHours,
  monthStart,
  parseDay,
  timezone,
  useAnalyticsFilters,
} from "./analytics";

// A status page in the manner of githubstatus.com: the overall state, each
// component's current state, and the Pi agent's last 90 days, day by day.
// It always covers the last 90 days, whatever the range picker says.

type Health = "ok" | "minor" | "major" | "none";

const HEALTH: Record<Health, { label: string; bar: string; text: string }> = {
  ok: { label: "No downtime", bar: "bg-status-ok", text: "text-status-ok" },
  minor: { label: "Minor outage", bar: "bg-status-low", text: "text-status-low" },
  major: { label: "Major outage", bar: "bg-status-empty", text: "text-status-empty" },
  none: { label: "No data", bar: "bg-border-primary", text: "text-text-tertiary" },
};

type Day = { online: number; offline: number };

/** Offline time in a day beyond this is a major outage, up to it minor. An
 * absolute bound, so today's half-tracked day isn't judged harshly. */
const MAJOR_OUTAGE_HOURS = 1;

function health(d: Day | undefined): Health {
  const tracked = d ? d.online + d.offline : 0;
  if (!d || tracked <= 0) return "none";
  if (d.offline <= 0) return "ok";
  return d.offline <= MAJOR_OUTAGE_HOURS ? "minor" : "major";
}

function uptimeOf(days: (Day | undefined)[]): number | null {
  let on = 0;
  let off = 0;
  for (const d of days) {
    if (!d) continue;
    on += d.online;
    off += d.offline;
  }
  return on + off > 0 ? on / (on + off) : null;
}

const pct = (u: number | null) => (u === null ? "—" : `${(u * 100).toFixed(2)}%`);

/** Daily online/offline hours for [start, end], keyed by day. */
function useUptimeDays(machineId: string, start: string, end: string) {
  return useQuery({
    queryKey: ["analytics", "uptime-days", machineId, start, end, timezone],
    queryFn: () =>
      getAnalytics("uptime", {
        machineId,
        from: parseDay(start).toISOString(),
        // The server caps this at now.
        to: parseDay(addDays(end, 1)).toISOString(),
        tz: timezone,
        bucket: "day",
      }),
    enabled: !!machineId,
    staleTime: 60_000,
    select: (u: UptimeAnalytics) => {
      const out: Record<string, Day> = {};
      for (const p of u.series) {
        out[dayKey(new Date(p.bucket))] = { online: p.onlineHours, offline: p.offlineHours };
      }
      return out;
    },
  });
}

function dayLabel(key: string): string {
  return parseDay(key).toLocaleDateString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
  });
}

function dayDetail(key: string, d: Day | undefined): string {
  const h = health(d);
  if (h === "none") return `${dayLabel(key)}: not tracked`;
  if (h === "ok") return `${dayLabel(key)}: no downtime`;
  return `${dayLabel(key)}: offline ${formatHours(d!.offline)} (${pct(uptimeOf([d]))} up)`;
}

type ComponentState = { name: string; status: Health; detail: string; link?: string };

export function StatusOverview() {
  const { machineId } = useAnalyticsFilters();
  const today = dayKey(new Date());
  const start = addDays(today, -89);
  const days = useUptimeDays(machineId, start, today);

  const machine = useQuery({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
    enabled: !!machineId,
  });
  const alerts = useActiveAlerts();
  const backup = useBackupStatus();
  const slack = useQuery({ queryKey: ["slack"], queryFn: getSlackSettings });

  const [active, setActive] = useState<string | null>(null);

  const agent = machine.data?.agent;
  const quiet = (alerts.data ?? []).filter(
    (a) => a.alertType === "quiet_slot" || a.alertType === "quiet_machine",
  );
  const components: ComponentState[] = [
    {
      name: "Pi agent",
      status: !agent ? "none" : agent.online ? "ok" : "major",
      detail: !agent
        ? "Checking…"
        : agent.online
          ? "Online: dispenses are being counted"
          : agent.lastSeen
            ? `Offline since ${new Date(agent.lastSeen).toLocaleString()}`
            : "No heartbeat yet",
    },
    {
      name: "Dispense sensors",
      status: !alerts.data ? "none" : quiet.length ? "minor" : "ok",
      detail: !alerts.data
        ? "Checking…"
        : quiet.length
          ? `${quiet.length} not registering. See Alerts`
          : "Every slot is registering",
      link: quiet.length ? "/admin/alerts" : undefined,
    },
    {
      name: "Backups",
      status: !backup.data ? "none" : backup.data.stale ? "major" : backup.data.pushEnabled ? "ok" : "minor",
      detail: !backup.data
        ? "Checking…"
        : backup.data.stale
          ? backup.data.lastError || "No recent push to GitHub"
          : backup.data.pushEnabled
            ? backup.data.lastPushAt
              ? `Last pushed ${new Date(backup.data.lastPushAt).toLocaleString()}`
              : "Waiting for the first push"
            : "Not pushed off the Pi: set up in Settings",
      link: backup.data?.stale || !backup.data?.pushEnabled ? "/admin/settings" : undefined,
    },
    {
      name: "Slack",
      status: !slack.data
        ? "none"
        : slack.data.commands === "error"
          ? "major"
          : slack.data.webhookSet
            ? "ok"
            : "minor",
      detail: !slack.data
        ? "Checking…"
        : slack.data.commands === "error"
          ? `/soda can't connect: ${slack.data.commandsError ?? "unknown error"}`
          : [
              slack.data.webhookSet ? "Alerts on" : "Alerts off",
              slack.data.commands === "connected" ? "/soda connected" : slack.data.commands === "off" ? "/soda off" : "/soda connecting",
            ].join(" · "),
      link: slack.data?.commands === "error" || !slack.data?.webhookSet ? "/admin/settings" : undefined,
    },
  ];
  const worst = components.some((c) => c.status === "major")
    ? "major"
    : components.some((c) => c.status === "minor")
      ? "minor"
      : "ok";
  const banner = {
    ok: { text: "All systems operational", cls: "bg-status-ok text-white" },
    minor: { text: "Partly degraded", cls: "bg-status-low text-white" },
    major: { text: "Something needs attention", cls: "bg-status-empty text-white" },
  }[worst];

  const keys = Array.from({ length: 90 }, (_, i) => addDays(start, i));
  const map = days.data ?? {};
  const uptime90 = uptimeOf(keys.map((k) => map[k]));
  const uptime30 = uptimeOf(keys.slice(60).map((k) => map[k]));

  return (
    <div className="space-y-4">
      <div className={`rounded-2xl px-5 py-4 text-base font-semibold ${banner.cls}`} role="status">
        {banner.text}
      </div>

      <Card padding="none">
        <ul className="divide-y divide-border-primary">
          {components.map((c, i) => (
            <li key={c.name} className="px-4 sm:px-6 py-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm font-medium text-text-primary">{c.name}</p>
                  <p className="mt-0.5 text-xs text-text-secondary">
                    {c.link ? (
                      <Link to={c.link} className="hover:underline">
                        {c.detail}
                      </Link>
                    ) : (
                      c.detail
                    )}
                  </p>
                </div>
                <span className={`shrink-0 text-xs font-medium ${HEALTH[c.status].text}`}>
                  {c.status === "ok" ? "Operational" : c.status === "minor" ? "Degraded" : c.status === "major" ? "Down" : "—"}
                </span>
              </div>

              {/* The agent is the one component with history: its heartbeats. */}
              {i === 0 && (
                <div className="mt-4">
                  <div
                    className="flex gap-px sm:gap-[2px] h-8"
                    role="img"
                    aria-label={`Pi agent uptime, last 90 days: ${pct(uptime90)}`}
                    onMouseLeave={() => setActive(null)}
                  >
                    {keys.map((k, j) => (
                      <button
                        key={k}
                        type="button"
                        title={dayDetail(k, map[k])}
                        onMouseEnter={() => setActive(k)}
                        onFocus={() => setActive(k)}
                        onClick={() => setActive(k)}
                        // Phones show the last 30 days, as status pages do.
                        className={`flex-1 rounded-[2px] ${HEALTH[health(map[k])].bar} ${
                          j < 60 ? "hidden sm:block" : ""
                        } ${active === k ? "opacity-70" : "hover:opacity-70"}`}
                        aria-label={dayDetail(k, map[k])}
                      />
                    ))}
                  </div>
                  <div className="mt-2 flex items-center gap-3 text-[11px] text-text-tertiary">
                    <span className="hidden sm:inline">90 days ago</span>
                    <span className="sm:hidden">30 days ago</span>
                    <span className="flex-1 border-t border-border-primary" />
                    <span className="text-text-secondary font-medium">
                      <span className="hidden sm:inline">{pct(uptime90)}</span>
                      <span className="sm:hidden">{pct(uptime30)}</span> uptime
                    </span>
                    <span className="flex-1 border-t border-border-primary" />
                    <span>Today</span>
                  </div>
                  <p className="mt-2 h-4 text-xs text-text-secondary">
                    {active ? dayDetail(active, map[active]) : ""}
                  </p>
                </div>
              )}
            </li>
          ))}
        </ul>
      </Card>

      <UptimeCalendar machineId={machineId} today={today} />
    </div>
  );
}

const WEEKDAY_INITIALS = ["S", "M", "T", "W", "T", "F", "S"];

/** Three months of days, each square colored by that day's health. */
function UptimeCalendar({ machineId, today }: { machineId: string; today: string }) {
  // offset 0 shows the current month and the two before it.
  const [offset, setOffset] = useState(0);
  const last = addMonths(monthStart(today), -3 * offset);
  const months = [addMonths(last, -2), addMonths(last, -1), last];
  const end = addDays(addMonths(last, 1), -1);
  const days = useUptimeDays(machineId, months[0], end < today ? end : today);
  const map = days.data ?? {};
  const [active, setActive] = useState<string | null>(null);

  const title = (m: string) =>
    parseDay(m).toLocaleDateString(undefined, { month: "long", year: "numeric" });

  return (
    <Card>
      <div className="flex items-center justify-between mb-4 gap-3">
        <div>
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
            Pi agent uptime by day
          </p>
          <p className="mt-1 text-sm text-text-secondary">
            {title(months[0])} to {title(months[2])}
          </p>
        </div>
        <div className="flex gap-1">
          <button
            type="button"
            onClick={() => setOffset((o) => o + 1)}
            className="w-9 h-9 rounded-lg text-text-tertiary hover:bg-surface-secondary hover:text-text-primary"
            aria-label="Earlier months"
          >
            ‹
          </button>
          <button
            type="button"
            onClick={() => setOffset((o) => Math.max(0, o - 1))}
            disabled={offset === 0}
            className="w-9 h-9 rounded-lg text-text-tertiary hover:bg-surface-secondary hover:text-text-primary disabled:opacity-30 disabled:pointer-events-none"
            aria-label="Later months"
          >
            ›
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-6 sm:gap-8 justify-items-center sm:justify-items-start">
        {months.map((m, i) => {
          const first = parseDay(m);
          const count = new Date(first.getFullYear(), first.getMonth() + 1, 0).getDate();
          const keys = Array.from({ length: count }, (_, i) => addDays(m, i));
          const tracked = keys.filter((k) => k <= today).map((k) => map[k]);
          return (
            // Phones stack the months; the current one comes first.
            <div key={m} className={`w-full max-w-64 ${i === 2 ? "order-first sm:order-none" : ""}`}>
              <div className="flex items-baseline justify-between mb-2">
                <p className="text-sm font-medium text-text-primary">
                  {first.toLocaleDateString(undefined, { month: "long" })}
                </p>
                <p className="text-xs text-text-tertiary tabular-nums">{pct(uptimeOf(tracked))}</p>
              </div>
              <div className="grid grid-cols-7 gap-1 text-center">
                {WEEKDAY_INITIALS.map((w, i) => (
                  <span key={i} className="text-[10px] text-text-tertiary">
                    {w}
                  </span>
                ))}
                {Array.from({ length: first.getDay() }, (_, i) => (
                  <span key={`pad${i}`} />
                ))}
                {keys.map((k) => {
                  const future = k > today;
                  const h = health(map[k]);
                  return (
                    <button
                      key={k}
                      type="button"
                      disabled={future}
                      title={future ? undefined : dayDetail(k, map[k])}
                      aria-label={future ? dayLabel(k) : dayDetail(k, map[k])}
                      onMouseEnter={() => !future && setActive(k)}
                      onFocus={() => setActive(k)}
                      onClick={() => setActive(k)}
                      className={`aspect-square rounded-[3px] ${
                        future ? "bg-surface-tertiary" : HEALTH[h].bar
                      } ${k === today ? "ring-2 ring-text-primary/30" : ""} ${
                        active === k ? "opacity-70" : "hover:opacity-70"
                      }`}
                    />
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-text-secondary">
        {(["ok", "minor", "major", "none"] as const).map((h) => (
          <span key={h} className="inline-flex items-center gap-1.5">
            <span className={`w-3 h-3 rounded-[3px] ${HEALTH[h].bar}`} />
            {HEALTH[h].label}
          </span>
        ))}
        <span className="ml-auto h-4 text-text-secondary">
          {active ? dayDetail(active, map[active]) : ""}
        </span>
      </div>
    </Card>
  );
}
