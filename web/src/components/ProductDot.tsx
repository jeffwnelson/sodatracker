// A small dot in the product's chosen color. Renders nothing for products
// without one, so names line up the same as before colors existed.
export function ProductDot({ color }: { color?: string }) {
  if (!color) return null;
  return (
    <span
      aria-hidden
      className="inline-block w-2.5 h-2.5 shrink-0 rounded-full ring-1 ring-black/10"
      style={{ background: color }}
    />
  );
}
