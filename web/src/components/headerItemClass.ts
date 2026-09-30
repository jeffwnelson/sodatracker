/** Style for header nav links and header actions, shared so they match. */
export function headerItemClass(active = false): string {
  return `px-3 py-1.5 rounded-lg text-sm font-medium transition-colors duration-150 ${
    active
      ? "bg-accent-soft text-accent"
      : "text-text-tertiary hover:text-text-primary hover:bg-surface-secondary"
  }`;
}
