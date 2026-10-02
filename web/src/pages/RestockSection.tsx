import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getMachineStatus, createRestock } from "../api/client";
import { useMachine } from "../api/machine";
import { Card } from "../components/Card";
import { Button } from "../components/Button";
import { SkeletonCard } from "../components/Skeleton";

/**
 * Log a restock: the cans added to each slot. Lives on the Settings page;
 * stock is computed from the last restock minus dispenses since.
 */
export function RestockSection() {
  const { machineId, isLoading: machineLoading } = useMachine();
  const queryClient = useQueryClient();

  const { data: status, isLoading } = useQuery({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
    enabled: !!machineId,
  });

  const [quantities, setQuantities] = useState<Record<number, string>>({});
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  const mutation = useMutation({
    mutationFn: (slots: { position: number; quantity: number }[]) =>
      createRestock(machineId, slots),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["machineStatus", machineId],
      });
      queryClient.invalidateQueries({ queryKey: ["restocks", machineId] });
      setQuantities({});
      setSaved(true);
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
    setSaved(false);
    mutation.mutate(slots);
  };

  if (machineLoading || isLoading) return <SkeletonCard />;
  // No machine yet: the Machine section below adds it.
  if (!status) return null;

  return (
    <form onSubmit={handleSubmit}>
      <Card>
        <div className="flex items-center justify-between mb-4">
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
            Restock
          </p>
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
            size="sm"
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
                size="sm"
              >
                Max
              </Button>
            </div>
          ))}
        </div>

        {error && (
          <p className="text-sm text-status-empty mt-4">{error}</p>
        )}

        <div className="flex items-center gap-3 mt-6 pt-6 border-t border-border-primary">
          <Button type="submit" loading={mutation.isPending}>
            Save Restock
          </Button>
          {saved && <span className="text-sm text-status-ok">Restock saved</span>}
        </div>
      </Card>
    </form>
  );
}
