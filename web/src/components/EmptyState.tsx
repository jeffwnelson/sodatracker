export function EmptyState({ message }: { message: string }) {
  return (
    <div className="text-center py-12">
      <p className="text-sm text-text-tertiary">{message}</p>
    </div>
  );
}
