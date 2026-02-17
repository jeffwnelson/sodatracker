import type { SlotStatus } from "../api/types";

const statusColors: Record<string, string> = {
  ok: "bg-green-500",
  low: "bg-yellow-500",
  empty: "bg-red-500",
};

const statusBorders: Record<string, string> = {
  ok: "border-green-600",
  low: "border-yellow-600",
  empty: "border-red-600",
};

export function StockGrid({ slots }: { slots: SlotStatus[] }) {
  if (slots.length === 0) {
    return (
      <p className="text-gray-500 text-sm italic">No slots configured</p>
    );
  }

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
      {slots.map((ss) => (
        <div
          key={ss.slot.position}
          className={`rounded-lg border-2 p-3 text-center ${statusBorders[ss.status]} bg-white`}
        >
          <div
            className={`${statusColors[ss.status]} text-white text-xs font-bold px-2 py-0.5 rounded-full inline-block mb-2`}
          >
            {ss.status.toUpperCase()}
          </div>
          <div className="font-semibold text-gray-900 text-sm truncate">
            {ss.product?.name || `Slot ${ss.slot.position}`}
          </div>
          <div className="text-2xl font-bold text-gray-800 mt-1">
            {ss.currentStock}
            <span className="text-sm text-gray-500 font-normal">
              /{ss.slot.capacity}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}
