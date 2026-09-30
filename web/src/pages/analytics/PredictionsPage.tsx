import { useQuery } from "@tanstack/react-query";
import { getStockPredictions } from "../../api/client";
import type { StockPrediction } from "../../api/types";
import { Card } from "../../components/Card";
import { DataTable } from "../../components/DataTable";
import type { Column } from "../../components/DataTable";
import { SkeletonCard } from "../../components/Skeleton";
import { useAnalyticsFilters } from "./analytics";
import { NoData } from "./components";

// Predictions use a fixed lookback on the server, so the date range filter
// that scopes the other analytics tabs doesn't apply here.
const LOOKBACK_DAYS = 14;

const columns: Column<StockPrediction>[] = [
  {
    header: "Slot",
    accessor: (row) => (
      <span className="font-medium text-text-primary">
        Slot {row.slotPosition}
        {row.productName ? ` · ${row.productName}` : ""}
      </span>
    ),
  },
  {
    header: "Stock",
    accessor: (row) => row.currentStock,
    className: "text-right tabular-nums",
    headerClassName: "text-right",
  },
  {
    header: "Rate/day",
    accessor: (row) =>
      row.dispensesPerDay > 0 ? row.dispensesPerDay.toFixed(1) : "—",
    className: "text-right tabular-nums",
    headerClassName: "text-right",
  },
  {
    header: "Days left",
    accessor: (row) => {
      const d = row.estimatedDaysLeft;
      const text = d < 0 ? "No data" : d === 0 ? "Empty" : `${d} days`;
      const tone =
        d >= 0 && d < 1
          ? "text-status-empty font-semibold"
          : d >= 1 && d < 3
            ? "text-status-low font-semibold"
            : "";
      return <span className={tone}>{text}</span>;
    },
    className: "text-right tabular-nums whitespace-nowrap",
    headerClassName: "text-right",
  },
  {
    header: "Est. stockout",
    accessor: (row) =>
      row.predictedStockout
        ? new Date(row.predictedStockout).toLocaleDateString()
        : "—",
    className: "text-right whitespace-nowrap",
    headerClassName: "text-right",
  },
];

export function PredictionsPage() {
  const { machineId } = useAnalyticsFilters();
  const { data, isLoading, error } = useQuery({
    queryKey: ["predictions", machineId, LOOKBACK_DAYS],
    queryFn: () => getStockPredictions(machineId, LOOKBACK_DAYS),
    enabled: !!machineId,
    staleTime: 30_000,
  });

  if (isLoading) return <SkeletonCard />;

  return (
    <Card>
      <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
        Stock predictions
      </p>
      <p className="text-xs text-text-muted mt-1 mb-4">
        Based on the last {LOOKBACK_DAYS} days of vends. The date range above
        doesn't apply here.
      </p>
      {error ? (
        <NoData message={`Could not load predictions: ${(error as Error).message}`} />
      ) : data && data.length > 0 ? (
        <DataTable
          columns={columns}
          data={data}
          keyFn={(p) => String(p.slotPosition)}
        />
      ) : (
        <NoData message="No slot data available." />
      )}
    </Card>
  );
}
