import { useQuery } from "@tanstack/react-query";
import { getHealth } from "../api/client";

// The version comes from the running server (stamped at build time from the
// cocogitto release tag), so the footer always matches what is deployed.
export function AppFooter({ width = "max-w-6xl" }: { width?: string }) {
  const { data } = useQuery({
    queryKey: ["health"],
    queryFn: getHealth,
    staleTime: 5 * 60_000,
  });

  return (
    <footer className="border-t border-border-primary">
      <div
        className={`${width} mx-auto px-6 py-4 flex items-center justify-between text-xs text-text-muted`}
      >
        <span>SodaTracker</span>
        {data?.version && <span className="tabular-nums">{data.version}</span>}
      </div>
    </footer>
  );
}
