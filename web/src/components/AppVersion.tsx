import { useQuery } from "@tanstack/react-query";
import { getHealth } from "../api/client";

// The version comes from the running server (stamped at build time from the
// cocogitto release tag), so the header always matches what is deployed.
export function AppVersion() {
  const { data } = useQuery({
    queryKey: ["health"],
    queryFn: getHealth,
    staleTime: 5 * 60_000,
  });

  if (!data?.version) return null;
  return (
    <span className="text-xs text-text-muted tabular-nums">{data.version}</span>
  );
}
