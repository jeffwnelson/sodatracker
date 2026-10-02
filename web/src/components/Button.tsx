import type { ButtonHTMLAttributes, ReactNode } from "react";

const variants = {
  primary:
    "bg-accent text-white hover:bg-accent-hover focus-visible:ring-accent/40",
  secondary:
    "bg-surface-primary border border-border-primary text-text-primary hover:bg-surface-secondary focus-visible:ring-accent/40",
  ghost:
    "text-text-secondary hover:text-text-primary hover:bg-surface-secondary focus-visible:ring-accent/40",
  danger:
    "bg-status-empty text-white hover:bg-status-empty/90 focus-visible:ring-status-empty/40",
  // Text-only actions inside a card or table row: "No color", "Delete".
  link: "text-text-tertiary hover:text-text-primary focus-visible:ring-accent/40",
  "link-danger":
    "text-status-empty hover:text-status-empty/80 focus-visible:ring-status-empty/40",
};

type Variant = keyof typeof variants;

const isLink = (v: Variant) => v === "link" || v === "link-danger";

// sm is for actions in a card's header row; md is everything else. Link
// variants take only the text size, so they sit inline with other text.
const sizes = {
  sm: { box: "rounded-xl px-3 py-1.5 text-xs", link: "rounded text-xs" },
  md: { box: "rounded-xl px-4 py-2 text-sm", link: "rounded text-sm" },
};

export function Button({
  variant = "primary",
  size = "md",
  loading = false,
  children,
  className = "",
  type = "button",
  ...props
}: {
  variant?: Variant;
  size?: keyof typeof sizes;
  loading?: boolean;
  children: ReactNode;
  className?: string;
} & Omit<ButtonHTMLAttributes<HTMLButtonElement>, "className">) {
  const shape = isLink(variant) ? sizes[size].link : sizes[size].box;
  return (
    <button
      // Defaults to "button", not the browser's "submit", so a button in a
      // form submits only when it says so.
      type={type}
      className={`inline-flex items-center justify-center gap-2 font-medium transition-all duration-150 ease-out-custom focus-visible:outline-none focus-visible:ring-2 disabled:opacity-50 disabled:pointer-events-none ${shape} ${variants[variant]} ${className}`}
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
