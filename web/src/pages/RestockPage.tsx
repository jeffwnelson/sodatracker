import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getMachineStatus, createRestock } from "../api/client";

export function RestockPage() {
  const { id } = useParams<{ id: string }>();
  const machineId = id!;
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data: status, isLoading } = useQuery({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
  });

  const [quantities, setQuantities] = useState<Record<number, string>>({});
  const [error, setError] = useState("");

  const mutation = useMutation({
    mutationFn: (slots: { position: number; quantity: number }[]) =>
      createRestock(machineId, slots),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["machineStatus", machineId] });
      queryClient.invalidateQueries({ queryKey: ["restocks", machineId] });
      navigate(`/admin/machines/${machineId}`);
    },
    onError: (err: Error) => {
      setError(err.message);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!status) return;

    const slots = status.slots
      .map((ss) => {
        const raw = quantities[ss.slot.position];
        const qty = raw ? parseInt(raw, 10) : 0;
        return { position: ss.slot.position, quantity: qty };
      })
      .filter((s) => s.quantity > 0);

    if (slots.length === 0) {
      setError("Enter at least one slot quantity");
      return;
    }
    setError("");
    mutation.mutate(slots);
  };

  if (isLoading) {
    return <div className="text-center py-12 text-gray-500">Loading...</div>;
  }

  if (!status) {
    return (
      <div className="text-center py-12 text-gray-400">Machine not found.</div>
    );
  }

  return (
    <div className="max-w-lg mx-auto space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-gray-900">
          Restock: {status.machine.name}
        </h2>
        {status.machine.location && (
          <p className="text-sm text-gray-500">{status.machine.location}</p>
        )}
      </div>

      <form onSubmit={handleSubmit} className="bg-white rounded-xl shadow p-6 space-y-4">
        {status.slots.map((ss) => (
          <div
            key={ss.slot.position}
            className="flex items-center gap-4 border-b pb-3 last:border-0 last:pb-0"
          >
            <div className="flex-1">
              <p className="text-sm font-medium text-gray-900">
                Slot {ss.slot.position}:{" "}
                {ss.product?.name || "Unassigned"}
              </p>
              <p className="text-xs text-gray-500">
                Current: {ss.currentStock} / {ss.slot.capacity}
              </p>
            </div>
            <input
              type="number"
              min="0"
              max={ss.slot.capacity}
              placeholder={String(ss.slot.capacity)}
              value={quantities[ss.slot.position] ?? ""}
              onChange={(e) =>
                setQuantities((prev) => ({
                  ...prev,
                  [ss.slot.position]: e.target.value,
                }))
              }
              className="w-20 border border-gray-300 rounded-lg px-2 py-1.5 text-sm text-center focus:outline-none focus:ring-2 focus:ring-gray-900"
            />
          </div>
        ))}

        {error && <p className="text-sm text-red-600">{error}</p>}

        <div className="flex gap-3">
          <button
            type="submit"
            disabled={mutation.isPending}
            className="flex-1 bg-gray-900 text-white py-2 rounded-lg text-sm font-medium hover:bg-gray-800 disabled:opacity-50"
          >
            {mutation.isPending ? "Submitting..." : "Submit Restock"}
          </button>
          <button
            type="button"
            onClick={() => navigate(`/admin/machines/${machineId}`)}
            className="px-4 py-2 border border-gray-300 rounded-lg text-sm text-gray-700 hover:bg-gray-50"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
