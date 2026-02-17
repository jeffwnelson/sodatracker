import { useQuery } from "@tanstack/react-query";
import { listMachines, getMachineStatus } from "../api/client";
import type { MachineStatus } from "../api/types";
import { StockGrid } from "../components/StockGrid";

function MachineCard({ machineId }: { machineId: string }) {
  const { data, isLoading, error } = useQuery<MachineStatus>({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
    refetchInterval: 10_000,
  });

  if (isLoading) {
    return (
      <div className="bg-white rounded-xl shadow p-6 animate-pulse">
        <div className="h-6 bg-gray-200 rounded w-1/3 mb-4" />
        <div className="h-24 bg-gray-100 rounded" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="bg-white rounded-xl shadow p-6">
        <p className="text-red-500">Failed to load machine status</p>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-xl shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-xl font-bold text-gray-900">
            {data.machine.name}
          </h2>
          {data.machine.location && (
            <p className="text-sm text-gray-500">{data.machine.location}</p>
          )}
        </div>
      </div>
      <StockGrid slots={data.slots} />
    </div>
  );
}

export function CustomerDashboard() {
  const {
    data: machines,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
    refetchInterval: 30_000,
  });

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white shadow-sm border-b">
        <div className="max-w-6xl mx-auto px-4 py-4">
          <h1 className="text-2xl font-bold text-gray-900">SodaTracker</h1>
          <p className="text-sm text-gray-500">Live inventory status</p>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-4 py-8 space-y-6">
        {isLoading && (
          <div className="text-center py-12 text-gray-500">Loading...</div>
        )}

        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
            Failed to load machines. Is the API running?
          </div>
        )}

        {machines?.length === 0 && (
          <div className="text-center py-12 text-gray-400">
            No machines configured yet.
          </div>
        )}

        {machines?.map((m) => <MachineCard key={m.id} machineId={m.id} />)}
      </main>
    </div>
  );
}
