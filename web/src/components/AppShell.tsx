import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { AppFooter } from "./AppFooter";

/**
 * The one page frame every screen uses, so moving between the public page,
 * login and admin never changes the header's height or the content width:
 *
 *   header  56px, max-w-6xl: brand · optional nav · optional actions
 *   main    max-w-6xl, px-6 py-10 (or centered, for the login card)
 *   footer  max-w-6xl: brand · running version
 */
export function AppShell({
  nav,
  actions,
  center = false,
  children,
}: {
  nav?: ReactNode;
  actions?: ReactNode;
  center?: boolean;
  children: ReactNode;
}) {
  return (
    <div className="min-h-screen flex flex-col bg-surface-secondary">
      <header className="bg-surface-primary border-b border-border-primary">
        <div className="max-w-6xl mx-auto px-6 flex items-center justify-between h-14">
          <div className="flex items-center gap-8">
            <Link
              to="/"
              className="text-base font-semibold tracking-tight text-text-primary"
            >
              SodaTracker
            </Link>
            {nav}
          </div>
          {actions && <div className="flex items-center gap-1">{actions}</div>}
        </div>
      </header>
      <main
        className={`flex-1 w-full max-w-6xl mx-auto px-6 py-10 ${
          center ? "flex items-center justify-center" : ""
        }`}
      >
        {children}
      </main>
      <AppFooter />
    </div>
  );
}
