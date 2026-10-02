import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
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
import { useMachine } from "../api/machine";
import { RestockSection } from "./RestockSection";
import { BackupSection } from "./BackupSection";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { ProductDot } from "../components/ProductDot";
import { PageHeader } from "../components/PageHeader";
import { Card } from "../components/Card";
import { Input } from "../components/Input";
import { Button } from "../components/Button";
import { DataTable } from "../components/DataTable";
import type { Column } from "../components/DataTable";

export function SettingsPage() {
  return (
    <div className="space-y-8">
      <PageHeader title="Settings" />
      <RestockSection />
      <ProductsSection />
      <MachineSection />
      <SlotConfigSection />
      <BackupSection />
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
  const [sizeOz, setSizeOz] = useState(String(product?.sizeOz ?? 12));
  const [caffeineMg, setCaffeineMg] = useState(String(product?.caffeineMg ?? 0));
  const [error, setError] = useState("");
  const queryClient = useQueryClient();

  const saveMut = useMutation({
    mutationFn: () =>
      createProduct({
        ...(product ? { id: product.id } : {}),
        name,
        ...(description ? { description } : {}),
        ...(costPerCan ? { costPerCan: Number(costPerCan) } : {}),
        // Always sent: saving is an upsert, so leaving them out would clear them.
        color,
        sizeOz: Number(sizeOz) || 12,
        caffeineMg: Number(caffeineMg) || 0,
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
          <div className="grid grid-cols-2 gap-3">
            <Input
              label="Size (fl oz)"
              type="number"
              step="0.1"
              min="0"
              value={sizeOz}
              onChange={(e) => setSizeOz(e.target.value)}
              placeholder="12"
            />
            <Input
              label="Caffeine (mg/can)"
              type="number"
              step="1"
              min="0"
              value={caffeineMg}
              onChange={(e) => setCaffeineMg(e.target.value)}
              placeholder="0"
            />
          </div>
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
                <Button variant="link" size="sm" onClick={() => setColor("")}>
                  Clear
                </Button>
              )}
            </div>
          </div>
          {error && <p className="text-sm text-status-empty">{error}</p>}
          <div className="flex items-center justify-between pt-4 border-t border-border-primary">
            <div>
              {isEdit && onDeleteClick && (
                <Button variant="link-danger" onClick={onDeleteClick}>
                  Delete Product
                </Button>
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
                <Button variant="link-danger" onClick={onDeleteClick}>
                  Delete Machine
                </Button>
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
      // Phones keep name, cost and Delete; the description is in the modal.
      className: "hidden sm:table-cell",
      headerClassName: "hidden sm:table-cell",
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
        <Button
          variant="link-danger"
          onClick={(e) => {
            e.stopPropagation(); // the row click opens the edit modal
            setError("");
            setDeleteTarget({ id: row.id, name: row.name });
          }}
        >
          Delete
        </Button>
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
        <Button onClick={() => setShowModal(true)} size="sm">
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

/* ── Machine Section ────────────────────────────────────── */

// SodaTracker tracks one machine: this edits it, or adds it on a fresh install.
function MachineSection() {
  const queryClient = useQueryClient();
  const { machine } = useMachine();

  const [modal, setModal] = useState<"add" | "edit" | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [error, setError] = useState("");

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteMachine(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["machines"] });
      setConfirmDelete(false);
      setModal(null);
    },
    onError: (err: Error) => setError(err.message),
  });

  const field = (label: string, value: string | number) => (
    <div>
      <dt className="text-xs text-text-tertiary">{label}</dt>
      <dd className="mt-0.5 text-sm font-medium text-text-primary">{value}</dd>
    </div>
  );

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
          Machine
        </p>
        <Button
          variant={machine ? "secondary" : "primary"}
          onClick={() => setModal(machine ? "edit" : "add")}
          size="sm"
        >
          {machine ? "Edit" : "Add Machine"}
        </Button>
      </div>

      {machine ? (
        <dl className="grid grid-cols-3 gap-4">
          {field("Name", machine.name)}
          {field("Location", machine.location || "\u2014")}
          {field("Slots", machine.slotCount)}
        </dl>
      ) : (
        <p className="text-sm text-text-tertiary text-center py-6">
          No machine yet.
        </p>
      )}

      {error && <p className="text-sm text-status-empty mt-2">{error}</p>}

      {modal && (
        <MachineModal
          machine={modal === "edit" ? machine : undefined}
          onClose={() => setModal(null)}
          onSaved={() => setModal(null)}
          onDeleteClick={modal === "edit" ? () => setConfirmDelete(true) : undefined}
        />
      )}

      <ConfirmDialog
        open={confirmDelete}
        title="Delete Machine"
        message={`Are you sure you want to delete "${machine?.name}"? All slots, dispenses, restocks, and alerts for this machine will be permanently deleted.`}
        onConfirm={() => machine && deleteMut.mutate(machine.id)}
        onCancel={() => setConfirmDelete(false)}
        isPending={deleteMut.isPending}
      />
    </Card>
  );
}

/* ── Slot Config Section ────────────────────────────────── */

function SlotConfigSection() {
  const queryClient = useQueryClient();
  const { machine, machineId } = useMachine();
  const { data: products } = useQuery({
    queryKey: ["products"],
    queryFn: listProducts,
  });

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
      </div>

      {!machineId && (
        <p className="text-sm text-text-tertiary">
          Add the machine above to configure slots.
        </p>
      )}

      {machineId && slotCount > 0 && (
        <>
          <div className="overflow-x-auto">
            <table className="block sm:table w-full text-sm">
              <thead className="hidden sm:table-header-group">
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
              <tbody className="block sm:table-row-group divide-y divide-border-primary">
                {Array.from({ length: slotCount }, (_, i) => {
                  const pos = i + 1;
                  const cfg = getSlotConfig(pos);
                  return (
                    // Phones stack each slot: number and product on top, the
                    // three numbers in a row beneath with their own labels.
                    <tr key={pos} className="grid grid-cols-3 gap-2 py-3 sm:table-row sm:py-0">
                      <td className="col-span-3 sm:py-3 text-text-primary font-medium">
                        #{pos}
                      </td>
                      <td className="col-span-3 sm:py-3">
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
                      <td className="sm:py-3">
                        <span className="sm:hidden block mb-1 text-[11px] font-medium uppercase tracking-wider text-text-tertiary">
                          Capacity
                        </span>
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
                      <td className="sm:py-3">
                        <span className="sm:hidden block mb-1 text-[11px] font-medium uppercase tracking-wider text-text-tertiary">
                          Low threshold
                        </span>
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
                      <td className="sm:py-3">
                        <span className="sm:hidden block mb-1 text-[11px] font-medium uppercase tracking-wider text-text-tertiary">
                          GPIO pin
                        </span>
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
            Permanently delete the machine, products, dispenses, restocks, and
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
        message="This will permanently delete ALL data including the machine, products, dispenses, restocks, alerts, and user accounts. This action cannot be undone. Are you sure?"
        onConfirm={() => resetMut.mutate()}
        onCancel={() => setShowConfirm(false)}
        isPending={resetMut.isPending}
      />
    </Card>
  );
}
