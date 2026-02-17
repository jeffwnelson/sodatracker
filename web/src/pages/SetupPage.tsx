import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listMachines,
  listProducts,
  createProduct,
  createMachine,
  setSlots,
  getMachineStatus,
} from "../api/client";
import type { Product } from "../api/types";

export function SetupPage() {
  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold text-gray-900">Setup</h2>
      <ProductsSection />
      <MachinesSection />
      <SlotConfigSection />
    </div>
  );
}

function ProductsSection() {
  const queryClient = useQueryClient();
  const { data: products } = useQuery({
    queryKey: ["products"],
    queryFn: listProducts,
  });

  const [name, setName] = useState("");
  const [upc, setUpc] = useState("");
  const [costPerCan, setCostPerCan] = useState("");
  const [error, setError] = useState("");

  const mutation = useMutation({
    mutationFn: () =>
      createProduct({
        name,
        upc,
        ...(costPerCan ? { costPerCan: Number(costPerCan) } : {}),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["products"] });
      setName("");
      setUpc("");
      setCostPerCan("");
      setError("");
    },
    onError: (err: Error) => setError(err.message),
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !upc.trim()) return;
    mutation.mutate();
  };

  return (
    <div className="bg-white rounded-xl shadow p-6">
      <h3 className="text-sm font-medium text-gray-700 mb-4">Products</h3>

      {products && products.length > 0 && (
        <div className="overflow-x-auto mb-4">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-gray-500">
                <th className="pb-2 font-medium">Name</th>
                <th className="pb-2 font-medium">UPC</th>
                <th className="pb-2 font-medium">Cost/Can</th>
              </tr>
            </thead>
            <tbody>
              {products.map((p) => (
                <tr key={p.id} className="border-b last:border-0">
                  <td className="py-2 text-gray-700">{p.name}</td>
                  <td className="py-2 text-gray-500 font-mono text-xs">
                    {p.upc}
                  </td>
                  <td className="py-2 text-gray-700">
                    {p.costPerCan != null ? `$${p.costPerCan.toFixed(2)}` : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <form onSubmit={handleSubmit} className="flex items-end gap-3 pt-2 border-t">
        <div>
          <label className="block text-xs text-gray-500 mb-1">Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Diet Coke"
            className="border rounded px-2 py-1.5 text-sm w-36"
            required
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">UPC</label>
          <input
            type="text"
            value={upc}
            onChange={(e) => setUpc(e.target.value)}
            placeholder="049000006346"
            className="border rounded px-2 py-1.5 text-sm w-36"
            required
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">Cost/Can</label>
          <input
            type="number"
            step="0.01"
            min="0"
            value={costPerCan}
            onChange={(e) => setCostPerCan(e.target.value)}
            placeholder="0.35"
            className="border rounded px-2 py-1.5 text-sm w-24"
          />
        </div>
        <button
          type="submit"
          disabled={mutation.isPending}
          className="bg-gray-900 text-white px-4 py-1.5 rounded text-sm font-medium hover:bg-gray-800 disabled:opacity-50"
        >
          Add Product
        </button>
      </form>
      {error && <p className="text-sm text-red-600 mt-2">{error}</p>}
    </div>
  );
}

function MachinesSection() {
  const queryClient = useQueryClient();
  const { data: machines } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
  });

  const [name, setName] = useState("");
  const [location, setLocation] = useState("");
  const [slotCount, setSlotCount] = useState("");
  const [error, setError] = useState("");

  const mutation = useMutation({
    mutationFn: () =>
      createMachine({
        name,
        ...(location ? { location } : {}),
        slotCount: Number(slotCount),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["machines"] });
      setName("");
      setLocation("");
      setSlotCount("");
      setError("");
    },
    onError: (err: Error) => setError(err.message),
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !slotCount) return;
    mutation.mutate();
  };

  return (
    <div className="bg-white rounded-xl shadow p-6">
      <h3 className="text-sm font-medium text-gray-700 mb-4">Machines</h3>

      {machines && machines.length > 0 && (
        <div className="overflow-x-auto mb-4">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-gray-500">
                <th className="pb-2 font-medium">Name</th>
                <th className="pb-2 font-medium">Location</th>
                <th className="pb-2 font-medium">Slots</th>
              </tr>
            </thead>
            <tbody>
              {machines.map((m) => (
                <tr key={m.id} className="border-b last:border-0">
                  <td className="py-2 text-gray-700">{m.name}</td>
                  <td className="py-2 text-gray-500">{m.location || "—"}</td>
                  <td className="py-2 text-gray-700">{m.slotCount}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <form onSubmit={handleSubmit} className="flex items-end gap-3 pt-2 border-t">
        <div>
          <label className="block text-xs text-gray-500 mb-1">Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Break Room"
            className="border rounded px-2 py-1.5 text-sm w-36"
            required
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">Location</label>
          <input
            type="text"
            value={location}
            onChange={(e) => setLocation(e.target.value)}
            placeholder="2nd Floor"
            className="border rounded px-2 py-1.5 text-sm w-36"
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">Slot Count</label>
          <input
            type="number"
            min="1"
            value={slotCount}
            onChange={(e) => setSlotCount(e.target.value)}
            placeholder="6"
            className="border rounded px-2 py-1.5 text-sm w-24"
            required
          />
        </div>
        <button
          type="submit"
          disabled={mutation.isPending}
          className="bg-gray-900 text-white px-4 py-1.5 rounded text-sm font-medium hover:bg-gray-800 disabled:opacity-50"
        >
          Add Machine
        </button>
      </form>
      {error && <p className="text-sm text-red-600 mt-2">{error}</p>}
    </div>
  );
}

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

  // Slot form state: array of slot configs indexed by position (1-based)
  const slotCount = machine?.slotCount ?? 0;
  const [slotConfigs, setSlotConfigs] = useState<
    Record<
      number,
      { productId: string; capacity: string; lowThreshold: string; gpioPin: string }
    >
  >({});
  const [error, setError] = useState("");

  // Initialize slot configs from current status when machine changes
  const initSlots = (id: string) => {
    setSelectedMachine(id);
    setSlotConfigs({});
    setError("");
  };

  // Build config for a slot position, merging current status data as defaults
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

  const updateSlot = (
    pos: number,
    field: string,
    value: string
  ) => {
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
      queryClient.invalidateQueries({ queryKey: ["machineStatus", machineId] });
      queryClient.invalidateQueries({ queryKey: ["machines"] });
      setError("");
    },
    onError: (err: Error) => setError(err.message),
  });

  return (
    <div className="bg-white rounded-xl shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-medium text-gray-700">Slot Configuration</h3>
        {machines && machines.length > 0 && (
          <select
            value={machineId}
            onChange={(e) => initSlots(e.target.value)}
            className="border rounded-lg px-3 py-2 text-sm"
          >
            {machines.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        )}
      </div>

      {!machineId && (
        <p className="text-gray-400 text-sm">
          Add a machine above to configure slots.
        </p>
      )}

      {machineId && slotCount > 0 && (
        <>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-gray-500">
                  <th className="pb-2 font-medium w-16">Slot</th>
                  <th className="pb-2 font-medium">Product</th>
                  <th className="pb-2 font-medium w-24">Capacity</th>
                  <th className="pb-2 font-medium w-24">Low Threshold</th>
                  <th className="pb-2 font-medium w-24">GPIO Pin</th>
                </tr>
              </thead>
              <tbody>
                {Array.from({ length: slotCount }, (_, i) => {
                  const pos = i + 1;
                  const cfg = getSlotConfig(pos);
                  return (
                    <tr key={pos} className="border-b last:border-0">
                      <td className="py-2 text-gray-700 font-medium">#{pos}</td>
                      <td className="py-2">
                        <select
                          value={cfg.productId}
                          onChange={(e) =>
                            updateSlot(pos, "productId", e.target.value)
                          }
                          className="border rounded px-2 py-1.5 text-sm w-full"
                        >
                          <option value="">— Select product —</option>
                          {products?.map((p: Product) => (
                            <option key={p.id} value={p.id}>
                              {p.name}
                            </option>
                          ))}
                        </select>
                      </td>
                      <td className="py-2">
                        <input
                          type="number"
                          min="1"
                          value={cfg.capacity}
                          onChange={(e) =>
                            updateSlot(pos, "capacity", e.target.value)
                          }
                          className="border rounded px-2 py-1.5 text-sm w-full"
                        />
                      </td>
                      <td className="py-2">
                        <input
                          type="number"
                          min="0"
                          value={cfg.lowThreshold}
                          onChange={(e) =>
                            updateSlot(pos, "lowThreshold", e.target.value)
                          }
                          className="border rounded px-2 py-1.5 text-sm w-full"
                        />
                      </td>
                      <td className="py-2">
                        <input
                          type="number"
                          min="0"
                          value={cfg.gpioPin}
                          onChange={(e) =>
                            updateSlot(pos, "gpioPin", e.target.value)
                          }
                          placeholder="—"
                          className="border rounded px-2 py-1.5 text-sm w-full"
                        />
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="flex items-center gap-3 mt-4 pt-4 border-t">
            <button
              onClick={() => mutation.mutate()}
              disabled={mutation.isPending}
              className="bg-gray-900 text-white px-4 py-1.5 rounded text-sm font-medium hover:bg-gray-800 disabled:opacity-50"
            >
              {mutation.isPending ? "Saving..." : "Save Slots"}
            </button>
            {mutation.isSuccess && (
              <span className="text-sm text-green-600">Saved.</span>
            )}
          </div>
          {error && <p className="text-sm text-red-600 mt-2">{error}</p>}
        </>
      )}
    </div>
  );
}
