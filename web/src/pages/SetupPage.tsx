import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listMachines,
  listProducts,
  createProduct,
  createMachine,
  deleteProduct,
  deleteMachine,
  setSlots,
  getMachineStatus,
  resetAllData,
} from "../api/client";
import type { Product, Machine } from "../api/types";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { ProductDot } from "../components/ProductDot";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Input } from "../components/Input";
import { Select } from "../components/Select";
import { Button } from "../components/Button";
import { DataTable } from "../components/DataTable";
import type { Column } from "../components/DataTable";

export function SetupPage() {
  return (
    <div className="space-y-8">
      <PageHeader title="Setup" />
      <ProductsSection />
      <MachinesSection />
      <SlotConfigSection />
      <DangerZoneSection />
    </div>
  );
}

/* ── Product Modal (add + edit) ─────────────────────────── */

function ProductModal({
  product,
  onClose,
  onSaved,
  onDeleteClick,
}: {
  product?: Product;
  onClose: () => void;
  onSaved: () => void;
  onDeleteClick?: () => void;
}) {
  const isEdit = !!product;
  const [name, setName] = useState(product?.name ?? "");
  const [description, setDescription] = useState(product?.description ?? "");
  const [costPerCan, setCostPerCan] = useState(
    product?.costPerCan != null ? String(product.costPerCan) : ""
  );
  // "" means no color: the dashboard falls back to its default palette.
  const [color, setColor] = useState(product?.color ?? "");
  const [error, setError] = useState("");
  const queryClient = useQueryClient();

  const saveMut = useMutation({
    mutationFn: () =>
      createProduct({
        ...(product ? { id: product.id } : {}),
        name,
        ...(description ? { description } : {}),
        ...(costPerCan ? { costPerCan: Number(costPerCan) } : {}),
        // Always sent: saving is an upsert, so leaving it out would clear it.
        color,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["products"] });
      onSaved();
    },
    onError: (err: Error) => setError(err.message),
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    saveMut.mutate();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="fixed inset-0 bg-black/20 backdrop-blur-sm animate-fade-in"
        onClick={onClose}
      />
      <div className="relative bg-surface-primary border border-border-primary rounded-2xl p-8 max-w-md w-full mx-4 animate-scale-in shadow-subtle">
        <h3 className="text-lg font-semibold text-text-primary mb-6">
          {isEdit ? "Edit Product" : "Add Product"}
        </h3>
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Diet Coke"
            autoFocus
            required
          />
          <Input
            label="Description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="12oz can"
          />
          <Input
            label="Cost/Can"
            type="number"
            step="0.01"
            min="0"
            value={costPerCan}
            onChange={(e) => setCostPerCan(e.target.value)}
            placeholder="0.35"
          />
          <div>
            <label
              htmlFor="product-color"
              className="block text-xs font-medium uppercase tracking-wider text-text-tertiary mb-1.5"
            >
              Color
            </label>
            <div className="flex items-center gap-3">
              <input
                id="product-color"
                type="color"
                value={color || "#a8a29e"}
                onChange={(e) => setColor(e.target.value)}
                className="h-9 w-12 cursor-pointer rounded-lg border border-border-primary bg-surface-primary p-1"
              />
              <span className="text-sm text-text-secondary tabular-nums">
                {color || "Default palette"}
              </span>
              {color && (
                <button
                  type="button"
                  onClick={() => setColor("")}
                  className="text-xs font-medium text-text-tertiary hover:text-text-primary transition-colors duration-150"
                >
                  Clear
                </button>
              )}
            </div>
          </div>
          {error && <p className="text-sm text-status-empty">{error}</p>}
          <div className="flex items-center justify-between pt-4 border-t border-border-primary">
            <div>
              {isEdit && onDeleteClick && (
                <button
                  type="button"
                  onClick={onDeleteClick}
                  className="text-sm font-medium text-status-empty hover:text-red-700 transition-colors duration-150"
                >
                  Delete Product
                </button>
              )}
            </div>
            <div className="flex gap-3">
              <Button variant="secondary" type="button" onClick={onClose}>
                Cancel
              </Button>
              <Button type="submit" loading={saveMut.isPending}>
                {isEdit ? "Save" : "Add Product"}
              </Button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}

/* ── Machine Modal (add + edit) ─────────────────────────── */

function MachineModal({
  machine,
  onClose,
  onSaved,
  onDeleteClick,
}: {
  machine?: Machine;
  onClose: () => void;
  onSaved: () => void;
  onDeleteClick?: () => void;
}) {
  const isEdit = !!machine;
  const [name, setName] = useState(machine?.name ?? "");
  const [location, setLocation] = useState(machine?.location ?? "");
  const [slotCount, setSlotCount] = useState(
    machine ? String(machine.slotCount) : ""
  );
  const [error, setError] = useState("");
  const queryClient = useQueryClient();

  const saveMut = useMutation({
    mutationFn: () =>
      createMachine({
        ...(machine ? { id: machine.id } : {}),
        name,
        ...(location ? { location } : {}),
        slotCount: Number(slotCount),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["machines"] });
      onSaved();
    },
    onError: (err: Error) => setError(err.message),
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !slotCount) return;
    saveMut.mutate();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="fixed inset-0 bg-black/20 backdrop-blur-sm animate-fade-in"
        onClick={onClose}
      />
      <div className="relative bg-surface-primary border border-border-primary rounded-2xl p-8 max-w-md w-full mx-4 animate-scale-in shadow-subtle">
        <h3 className="text-lg font-semibold text-text-primary mb-6">
          {isEdit ? "Edit Machine" : "Add Machine"}
        </h3>
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Break Room"
            autoFocus
            required
          />
          <Input
            label="Location"
            value={location}
            onChange={(e) => setLocation(e.target.value)}
            placeholder="2nd Floor"
          />
          <Input
            label="Slot Count"
            type="number"
            min="1"
            value={slotCount}
            onChange={(e) => setSlotCount(e.target.value)}
            placeholder="6"
            required
          />
          {error && <p className="text-sm text-status-empty">{error}</p>}
          <div className="flex items-center justify-between pt-4 border-t border-border-primary">
            <div>
              {isEdit && onDeleteClick && (
                <button
                  type="button"
                  onClick={onDeleteClick}
                  className="text-sm font-medium text-status-empty hover:text-red-700 transition-colors duration-150"
                >
                  Delete Machine
                </button>
              )}
            </div>
            <div className="flex gap-3">
              <Button variant="secondary" type="button" onClick={onClose}>
                Cancel
              </Button>
              <Button type="submit" loading={saveMut.isPending}>
                {isEdit ? "Save" : "Add Machine"}
              </Button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}

/* ── Products Section ───────────────────────────────────── */

function ProductsSection() {
  const queryClient = useQueryClient();
  const { data: products } = useQuery({
    queryKey: ["products"],
    queryFn: listProducts,
  });

  const [showModal, setShowModal] = useState(false);
  const [editTarget, setEditTarget] = useState<Product | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [error, setError] = useState("");

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteProduct(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["products"] });
      setDeleteTarget(null);
      setEditTarget(null);
    },
    onError: (err: Error) => {
      setError(err.message);
      setDeleteTarget(null);
      setEditTarget(null);
    },
  });

  const productColumns: Column<Product>[] = [
    {
      header: "Name",
      accessor: (row) => (
        <span className="inline-flex items-center gap-2 font-medium text-text-primary">
          <ProductDot color={row.color} />
          {row.name}
        </span>
      ),
    },
    {
      header: "Description",
      accessor: (row) => (
        <span className="text-xs">{row.description || "\u2014"}</span>
      ),
    },
    {
      header: "Cost/Can",
      accessor: (row) =>
        row.costPerCan != null ? `$${row.costPerCan.toFixed(2)}` : "\u2014",
      headerClassName: "text-right",
      className: "text-right",
    },
    {
      header: "",
      accessor: (row) => (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation(); // the row click opens the edit modal
            setError("");
            setDeleteTarget({ id: row.id, name: row.name });
          }}
          className="text-xs font-medium text-status-empty hover:text-red-700 transition-colors duration-150"
        >
          Delete
        </button>
      ),
      headerClassName: "text-right",
      className: "text-right",
    },
  ];

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
          Products
        </p>
        <Button onClick={() => setShowModal(true)} className="text-xs px-3 py-1.5">
          Add Product
        </Button>
      </div>

      {products && products.length > 0 ? (
        <DataTable
          columns={productColumns}
          data={products}
          keyFn={(p) => p.id}
          onRowClick={(p) => setEditTarget(p)}
        />
      ) : (
        <p className="text-sm text-text-tertiary text-center py-6">
          No products yet.
        </p>
      )}

      {error && <p className="text-sm text-status-empty mt-2">{error}</p>}

      {showModal && (
        <ProductModal
          onClose={() => setShowModal(false)}
          onSaved={() => setShowModal(false)}
        />
      )}

      {editTarget && (
        <ProductModal
          product={editTarget}
          onClose={() => setEditTarget(null)}
          onSaved={() => setEditTarget(null)}
          onDeleteClick={() =>
            setDeleteTarget({ id: editTarget.id, name: editTarget.name })
          }
        />
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete Product"
        message={`Are you sure you want to delete "${deleteTarget?.name}"? This cannot be undone.`}
        onConfirm={() => deleteTarget && deleteMut.mutate(deleteTarget.id)}
        onCancel={() => setDeleteTarget(null)}
        isPending={deleteMut.isPending}
      />
    </Card>
  );
}

/* ── Machines Section ───────────────────────────────────── */

function MachinesSection() {
  const queryClient = useQueryClient();
  const { data: machines } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
  });

  const [showModal, setShowModal] = useState(false);
  const [editTarget, setEditTarget] = useState<Machine | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [error, setError] = useState("");

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteMachine(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["machines"] });
      setDeleteTarget(null);
      setEditTarget(null);
    },
    onError: (err: Error) => setError(err.message),
  });

  const machineColumns: Column<Machine>[] = [
    {
      header: "Name",
      accessor: (row) => (
        <span className="font-medium text-text-primary">{row.name}</span>
      ),
    },
    {
      header: "Location",
      accessor: (row) => row.location || "\u2014",
    },
    {
      header: "Slots",
      accessor: (row) => row.slotCount,
      headerClassName: "text-right",
      className: "text-right",
    },
  ];

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
          Machines
        </p>
        <Button onClick={() => setShowModal(true)} className="text-xs px-3 py-1.5">
          Add Machine
        </Button>
      </div>

      {machines && machines.length > 0 ? (
        <DataTable
          columns={machineColumns}
          data={machines}
          keyFn={(m) => m.id}
          onRowClick={(m) => setEditTarget(m)}
        />
      ) : (
        <p className="text-sm text-text-tertiary text-center py-6">
          No machines yet.
        </p>
      )}

      {error && <p className="text-sm text-status-empty mt-2">{error}</p>}

      {showModal && (
        <MachineModal
          onClose={() => setShowModal(false)}
          onSaved={() => setShowModal(false)}
        />
      )}

      {editTarget && (
        <MachineModal
          machine={editTarget}
          onClose={() => setEditTarget(null)}
          onSaved={() => setEditTarget(null)}
          onDeleteClick={() =>
            setDeleteTarget({ id: editTarget.id, name: editTarget.name })
          }
        />
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete Machine"
        message={`Are you sure you want to delete "${deleteTarget?.name}"? All slots, dispenses, restocks, and alerts for this machine will be permanently deleted.`}
        onConfirm={() => deleteTarget && deleteMut.mutate(deleteTarget.id)}
        onCancel={() => setDeleteTarget(null)}
        isPending={deleteMut.isPending}
      />
    </Card>
  );
}

/* ── Slot Config Section ────────────────────────────────── */

function SlotConfigSection() {
  const queryClient = useQueryClient();
  const { data: machines } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
  });
  const { data: products } = useQuery({
    queryKey: ["products"],
    queryFn: listProducts,
  });

  const [selectedMachine, setSelectedMachine] = useState("");
  const machineId = selectedMachine || machines?.[0]?.id || "";
  const machine = machines?.find((m) => m.id === machineId);

  const { data: status } = useQuery({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
    enabled: !!machineId,
  });

  const slotCount = machine?.slotCount ?? 0;
  const [slotConfigs, setSlotConfigs] = useState<
    Record<
      number,
      {
        productId: string;
        capacity: string;
        lowThreshold: string;
        gpioPin: string;
      }
    >
  >({});
  const [error, setError] = useState("");

  const initSlots = (id: string) => {
    setSelectedMachine(id);
    setSlotConfigs({});
    setError("");
  };

  const getSlotConfig = (pos: number) => {
    if (slotConfigs[pos]) return slotConfigs[pos];
    const existing = status?.slots.find((s) => s.slot.position === pos);
    if (existing) {
      return {
        productId: existing.slot.productId ?? "",
        capacity: String(existing.slot.capacity),
        lowThreshold: String(existing.slot.lowThreshold),
        gpioPin: existing.slot.gpioPin ? String(existing.slot.gpioPin) : "",
      };
    }
    return { productId: "", capacity: "12", lowThreshold: "3", gpioPin: "" };
  };

  const updateSlot = (pos: number, field: string, value: string) => {
    setSlotConfigs((prev) => ({
      ...prev,
      [pos]: { ...getSlotConfig(pos), [field]: value },
    }));
  };

  const mutation = useMutation({
    mutationFn: () => {
      const slots = Array.from({ length: slotCount }, (_, i) => {
        const pos = i + 1;
        const cfg = getSlotConfig(pos);
        return {
          position: pos,
          productId: cfg.productId,
          capacity: Number(cfg.capacity) || 12,
          lowThreshold: Number(cfg.lowThreshold) || 3,
          ...(cfg.gpioPin ? { gpioPin: Number(cfg.gpioPin) } : {}),
        };
      });
      return setSlots(machineId, slots);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["machineStatus", machineId],
      });
      queryClient.invalidateQueries({ queryKey: ["machines"] });
      setError("");
    },
    onError: (err: Error) => setError(err.message),
  });

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
          Slot Configuration
        </p>
        {machines && machines.length > 0 && (
          <Select
            value={machineId}
            onChange={(e) => initSlots(e.target.value)}
            className="w-48"
          >
            {machines.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </Select>
        )}
      </div>

      {!machineId && (
        <p className="text-sm text-text-tertiary">
          Add a machine above to configure slots.
        </p>
      )}

      {machineId && slotCount > 0 && (
        <>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border-primary">
                  <th className="pb-3 text-left text-xs font-medium uppercase tracking-wider text-text-tertiary w-16">
                    Slot
                  </th>
                  <th className="pb-3 text-left text-xs font-medium uppercase tracking-wider text-text-tertiary">
                    Product
                  </th>
                  <th className="pb-3 text-left text-xs font-medium uppercase tracking-wider text-text-tertiary w-24">
                    Capacity
                  </th>
                  <th className="pb-3 text-left text-xs font-medium uppercase tracking-wider text-text-tertiary w-24">
                    Low Threshold
                  </th>
                  <th className="pb-3 text-left text-xs font-medium uppercase tracking-wider text-text-tertiary w-24">
                    GPIO Pin
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-primary">
                {Array.from({ length: slotCount }, (_, i) => {
                  const pos = i + 1;
                  const cfg = getSlotConfig(pos);
                  return (
                    <tr key={pos}>
                      <td className="py-3 text-text-primary font-medium">
                        #{pos}
                      </td>
                      <td className="py-3">
                        <select
                          value={cfg.productId}
                          onChange={(e) =>
                            updateSlot(pos, "productId", e.target.value)
                          }
                          className="w-full rounded-xl border border-border-primary bg-surface-primary px-2 py-1.5 text-sm text-text-primary transition-all duration-150 ease-out-custom focus:outline-none focus:ring-2 focus:ring-accent/40 focus:border-accent"
                        >
                          <option value="">
                            {"\u2014"} Select product {"\u2014"}
                          </option>
                          {products?.map((p: Product) => (
                            <option key={p.id} value={p.id}>
                              {p.name}
                            </option>
                          ))}
                        </select>
                      </td>
                      <td className="py-3">
                        <input
                          type="number"
                          min="1"
                          value={cfg.capacity}
                          onChange={(e) =>
                            updateSlot(pos, "capacity", e.target.value)
                          }
                          className="w-full rounded-xl border border-border-primary bg-surface-primary px-2 py-1.5 text-sm text-text-primary transition-all duration-150 ease-out-custom focus:outline-none focus:ring-2 focus:ring-accent/40 focus:border-accent"
                        />
                      </td>
                      <td className="py-3">
                        <input
                          type="number"
                          min="0"
                          value={cfg.lowThreshold}
                          onChange={(e) =>
                            updateSlot(pos, "lowThreshold", e.target.value)
                          }
                          className="w-full rounded-xl border border-border-primary bg-surface-primary px-2 py-1.5 text-sm text-text-primary transition-all duration-150 ease-out-custom focus:outline-none focus:ring-2 focus:ring-accent/40 focus:border-accent"
                        />
                      </td>
                      <td className="py-3">
                        <input
                          type="number"
                          min="0"
                          value={cfg.gpioPin}
                          onChange={(e) =>
                            updateSlot(pos, "gpioPin", e.target.value)
                          }
                          placeholder={"\u2014"}
                          className="w-full rounded-xl border border-border-primary bg-surface-primary px-2 py-1.5 text-sm text-text-primary transition-all duration-150 ease-out-custom focus:outline-none focus:ring-2 focus:ring-accent/40 focus:border-accent"
                        />
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="flex items-center gap-3 mt-4 pt-4 border-t border-border-primary">
            <Button
              onClick={() => mutation.mutate()}
              loading={mutation.isPending}
            >
              {mutation.isPending ? "Saving..." : "Save Slots"}
            </Button>
            {mutation.isSuccess && (
              <span className="text-sm text-status-ok">Saved.</span>
            )}
          </div>
          {error && (
            <p className="text-sm text-status-empty mt-2">{error}</p>
          )}
        </>
      )}
    </Card>
  );
}

/* ── Danger Zone ─────────────────────────────────────────── */

function DangerZoneSection() {
  const queryClient = useQueryClient();
  const [showConfirm, setShowConfirm] = useState(false);

  const resetMut = useMutation({
    mutationFn: () => resetAllData(),
    onSuccess: () => {
      queryClient.invalidateQueries();
      setShowConfirm(false);
    },
  });

  return (
    <Card>
      <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary mb-4">
        Danger Zone
      </p>
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-text-primary">
            Reset All Data
          </p>
          <p className="text-xs text-text-tertiary mt-0.5">
            Permanently delete all machines, products, dispenses, restocks, and
            alerts. You will need to log in again.
          </p>
        </div>
        <Button variant="danger" onClick={() => setShowConfirm(true)}>
          Reset Data
        </Button>
      </div>
      <ConfirmDialog
        open={showConfirm}
        title="Reset All Data"
        message="This will permanently delete ALL data including machines, products, dispenses, restocks, alerts, and user accounts. This action cannot be undone. Are you sure?"
        onConfirm={() => resetMut.mutate()}
        onCancel={() => setShowConfirm(false)}
        isPending={resetMut.isPending}
      />
    </Card>
  );
}
