import type { InputHTMLAttributes } from "react";

export function Input({
  label,
  error,
  className = "",
  ...props
}: {
  label?: string;
  error?: string;
  className?: string;
} & Omit<InputHTMLAttributes<HTMLInputElement>, "className">) {
  return (
    <div className={className}>
      {label && (
        <label className="block text-xs font-medium uppercase tracking-wider text-text-tertiary mb-1.5">
          {label}
        </label>
      )}
      <input
        className={`w-full rounded-xl border px-3 py-2 text-sm text-text-primary placeholder:text-text-muted bg-surface-primary transition-all duration-150 ease-out-custom focus:outline-none focus:ring-2 focus:ring-accent/40 focus:border-accent ${
          error ? "border-status-empty" : "border-border-primary"
        }`}
        {...props}
      />
      {error && <p className="mt-1 text-xs text-status-empty">{error}</p>}
    </div>
  );
}
