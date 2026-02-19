import type { ReactNode } from "react";

const variants = {
  ok: "bg-status-ok-bg text-status-ok",
  low: "bg-status-low-bg text-status-low",
  empty: "bg-status-empty-bg text-status-empty",
  info: "bg-accent-soft text-accent",
  neutral: "bg-surface-tertiary text-text-secondary",
};

export function Badge({
  variant = "neutral",
  children,
}: {
  variant?: keyof typeof variants;
  children: ReactNode;
}) {
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 text-xs font-medium rounded-full ${variants[variant]}`}
    >
      {children}
    </span>
  );
}
