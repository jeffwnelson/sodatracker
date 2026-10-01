import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { AppVersion } from "./AppVersion";
import { LiveIndicator } from "./LiveIndicator";

/**
 * The one page frame every screen uses, so moving between the public page,
 * login and admin never changes the header's height or the content width:
 *
 *   header  56px, max-w-6xl: brand + running version · optional nav · optional actions
 *   main    max-w-6xl, px-6 py-10 (or centered, for the login card)
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
            <div className="flex items-baseline gap-2">
              <Link
                to="/"
                className="text-base font-semibold tracking-tight text-text-primary"
              >
                SodaTracker
              </Link>
              <AppVersion />
            </div>
            {nav}
          </div>
          <div className="flex items-center gap-1">
            <LiveIndicator />
            {actions}
          </div>
        </div>
      </header>
      <main
        className={`flex-1 w-full max-w-6xl mx-auto px-6 py-10 ${
          center ? "flex items-center justify-center" : ""
        }`}
      >
        {children}
      </main>
    </div>
  );
}
