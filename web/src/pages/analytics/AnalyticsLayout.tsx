import { NavLink, Outlet, useLocation } from "react-router-dom";
import { PageHeader } from "../../components/PageHeader";
import { Select } from "../../components/Select";
import { SkeletonCard } from "../../components/Skeleton";
import { EmptyState } from "../../components/EmptyState";
import {
  RANGES,
  timezone,
  useAnalyticsFilters,
} from "./analytics";

const tabs = [
  { to: "consumption", label: "Consumption" },
  { to: "products", label: "Products" },
  { to: "inventory", label: "Inventory" },
  { to: "spend", label: "Spend" },
  { to: "predictions", label: "Predictions" },
  { to: "highlights", label: "Highlights" },
  { to: "uptime", label: "Uptime" },
];

export function AnalyticsLayout() {
  const { machines, machineId, range, setMachine, setRange } =
    useAnalyticsFilters();
  const { search } = useLocation();

  if (machines.isLoading) {
    return (
      <div className="space-y-6">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }
  if (!machines.data || machines.data.length === 0) {
    return <EmptyState message="No machines configured yet." />;
  }

  return (
    <div className="space-y-8">
      <PageHeader title="Analytics" subtitle={`Times shown in ${timezone}`} />

      {/* One filter row scopes every chart on every tab. */}
      <div className="flex flex-wrap items-center gap-3">
        <Select
          value={machineId}
          onChange={(e) => setMachine(e.target.value)}
          className="w-full sm:w-52"
          aria-label="Machine"
        >
          {machines.data.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name}
              {m.location ? ` — ${m.location}` : ""}
            </option>
          ))}
        </Select>
        <div
          className="inline-flex flex-wrap rounded-xl border border-border-primary bg-surface-primary p-1"
          role="group"
          aria-label="Date range"
        >
          {RANGES.map((r) => (
            <button
              key={r.id}
              type="button"
              onClick={() => setRange(r.id)}
              aria-pressed={range === r.id}
              className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors duration-150 ${
                range === r.id
                  ? "bg-accent-soft text-accent"
                  : "text-text-tertiary hover:text-text-primary"
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      {/* The baseline is an inset shadow rather than a border, so the active
          tab's underline can cover it without a negative margin. A -1px
          margin overflowed the nav, and overflow-x-auto then showed a
          vertical scrollbar. */}
      <nav
        className="flex gap-1 overflow-x-auto shadow-[inset_0_-1px_0_var(--color-border-primary)]"
        aria-label="Analytics dashboards"
      >
        {tabs.map((t) => (
          <NavLink
            key={t.to}
            to={{ pathname: t.to, search }}
            className={({ isActive }) =>
              `px-3 py-2 border-b-2 text-sm font-medium whitespace-nowrap transition-colors duration-150 ${
                isActive
                  ? "border-accent text-accent"
                  : "border-transparent text-text-tertiary hover:text-text-primary"
              }`
            }
          >
            {t.label}
          </NavLink>
        ))}
      </nav>

      <Outlet />
    </div>
  );
}
