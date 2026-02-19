import type { ReactNode } from "react";

const paddingVariants = {
  none: "",
  sm: "p-4",
  md: "p-6",
  lg: "p-8",
};

export function Card({
  children,
  padding = "md",
  className = "",
}: {
  children: ReactNode;
  padding?: "none" | "sm" | "md" | "lg";
  className?: string;
}) {
  return (
    <div
      className={`bg-surface-primary border border-border-primary rounded-2xl ${paddingVariants[padding]} ${className}`}
    >
      {children}
    </div>
  );
}
