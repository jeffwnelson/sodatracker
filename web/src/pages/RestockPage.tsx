import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getMachineStatus, createRestock } from "../api/client";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Button } from "../components/Button";
import { SkeletonCard } from "../components/Skeleton";
import { EmptyState } from "../components/EmptyState";

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
      queryClient.invalidateQueries({
        queryKey: ["machineStatus", machineId],
      });
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
    return (
      <div className="max-w-lg mx-auto space-y-6">
        <SkeletonCard />
      </div>
    );
  }

  if (!status) {
    return <EmptyState message="Machine not found." />;
  }

  return (
    <div className="max-w-lg mx-auto space-y-6">
      <PageHeader
        title={`Restock: ${status.machine.name}`}
        subtitle={status.machine.location || undefined}
      />

      <form onSubmit={handleSubmit}>
        <Card>
          <div className="flex justify-end mb-2">
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                const all: Record<number, string> = {};
                for (const ss of status.slots) {
                  all[ss.slot.position] = String(ss.slot.capacity);
                }
                setQuantities(all);
              }}
              className="!py-1 !px-3 !text-xs"
            >
              Max All
            </Button>
          </div>
          <div className="divide-y divide-border-primary">
            {status.slots.map((ss) => (
              <div
                key={ss.slot.position}
                className="flex items-center gap-4 py-4 first:pt-0 last:pb-0"
              >
                <div className="flex-1">
                  <p className="text-sm font-medium text-text-primary">
                    Slot {ss.slot.position}:{" "}
                    {ss.product?.name || "Unassigned"}
                  </p>
                  <p className="text-xs text-text-tertiary mt-0.5">
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
                  className="w-20 rounded-xl border border-border-primary bg-surface-primary px-2 py-2 text-sm text-center text-text-primary transition-all duration-150 ease-out-custom focus:outline-none focus:ring-2 focus:ring-accent/40 focus:border-accent"
                />
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() =>
                    setQuantities((prev) => ({
                      ...prev,
                      [ss.slot.position]: String(ss.slot.capacity),
                    }))
                  }
                  className="!py-1 !px-3 !text-xs"
                >
                  Max
                </Button>
              </div>
            ))}
          </div>

          {error && (
            <p className="text-sm text-status-empty mt-4">{error}</p>
          )}

          <div className="flex gap-3 mt-6 pt-6 border-t border-border-primary">
            <Button
              type="submit"
              loading={mutation.isPending}
              className="flex-1"
            >
              Submit Restock
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => navigate(`/admin/machines/${machineId}`)}
            >
              Cancel
            </Button>
          </div>
        </Card>
      </form>
    </div>
  );
}
