import type { SelectHTMLAttributes, ReactNode } from "react";

export function Select({
  label,
  children,
  className = "",
  ...props
}: {
  label?: string;
  children: ReactNode;
  className?: string;
} & Omit<SelectHTMLAttributes<HTMLSelectElement>, "className">) {
  return (
    <div className={className}>
      {label && (
        <label className="block text-xs font-medium uppercase tracking-wider text-text-tertiary mb-1.5">
          {label}
        </label>
      )}
      <select
        className="w-full rounded-xl border border-border-primary bg-surface-primary px-3 py-2 text-sm text-text-primary transition-all duration-150 ease-out-custom focus:outline-none focus:ring-2 focus:ring-accent/40 focus:border-accent"
        {...props}
      >
        {children}
      </select>
    </div>
  );
}
