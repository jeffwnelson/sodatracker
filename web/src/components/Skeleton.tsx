export function SkeletonText({ className = "" }: { className?: string }) {
  return (
    <div
      className={`h-4 bg-surface-tertiary rounded animate-pulse ${className}`}
    />
  );
}

export function SkeletonCard() {
  return (
    <div className="bg-surface-primary border border-border-primary rounded-2xl p-6 space-y-4 animate-pulse">
      <div className="h-5 bg-surface-tertiary rounded w-1/3" />
      <div className="h-20 bg-surface-secondary rounded" />
    </div>
  );
}
