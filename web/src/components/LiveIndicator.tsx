import { useAuth } from "../context/AuthContext";
import { useLive } from "../context/LiveUpdates";

/** Header dot showing whether dashboards are updating in real time. */
export function LiveIndicator() {
  const { isAuthenticated } = useAuth();
  const live = useLive();
  if (!isAuthenticated) return null;
  return (
    <span
      className="flex items-center gap-1.5 px-3 text-xs font-medium text-text-tertiary"
      title={
        live
          ? "Updates arrive as sodas are dispensed"
          : "Not connected for live updates; refreshing every few seconds"
      }
    >
      <span
        className={`h-2 w-2 rounded-full ${
          live ? "bg-status-ok" : "bg-text-tertiary opacity-50"
        }`}
      />
      {live ? "Live" : "Reconnecting"}
    </span>
  );
}
