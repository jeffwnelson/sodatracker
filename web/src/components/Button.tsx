import type { ButtonHTMLAttributes, ReactNode } from "react";

const variants = {
  primary:
    "bg-accent text-white hover:bg-accent-hover focus-visible:ring-accent/40",
  secondary:
    "bg-surface-primary border border-border-primary text-text-primary hover:bg-surface-secondary focus-visible:ring-accent/40",
  ghost:
    "text-text-secondary hover:text-text-primary hover:bg-surface-secondary focus-visible:ring-accent/40",
  danger:
    "bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-500/40",
};

export function Button({
  variant = "primary",
  loading = false,
  children,
  className = "",
  ...props
}: {
  variant?: keyof typeof variants;
  loading?: boolean;
  children: ReactNode;
  className?: string;
} & Omit<ButtonHTMLAttributes<HTMLButtonElement>, "className">) {
  return (
    <button
      className={`inline-flex items-center justify-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all duration-150 ease-out-custom focus-visible:outline-none focus-visible:ring-2 disabled:opacity-50 disabled:pointer-events-none ${variants[variant]} ${className}`}
      disabled={loading || props.disabled}
      {...props}
    >
      {loading && (
        <svg
          className="animate-spin h-4 w-4"
          viewBox="0 0 24 24"
          fill="none"
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="4"
          />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
          />
        </svg>
      )}
      {children}
    </button>
  );
}
