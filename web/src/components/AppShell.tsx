import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { AppVersion } from "./AppVersion";
import { LiveIndicator } from "./LiveIndicator";

/**
 * The one page frame every screen uses, so moving between the public page,
 * login and admin never changes the header's height or the content width:
 *
 *   header  56px, max-w-6xl: brand + running version · optional nav · optional actions
 *   main    max-w-6xl, px-6 py-10 (px-4 py-6 on phones), or centered for the login card
 *   tabBar  optional, phones only: fixed to the bottom, above the home indicator
 *
 * Safe-area insets keep the header and tab bar clear of the notch and the
 * home indicator when the app runs full screen from the home screen.
 */
export function AppShell({
  nav,
  actions,
  center = false,
  tabBar,
  children,
}: {
  nav?: ReactNode;
  actions?: ReactNode;
  center?: boolean;
  /** Phone navigation, shown below the sm breakpoint in place of nav. */
  tabBar?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="min-h-dvh flex flex-col bg-surface-secondary">
      <header className="sticky top-0 z-30 bg-surface-primary border-b border-border-primary pt-[env(safe-area-inset-top)] px-[env(safe-area-inset-left)]">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 flex items-center justify-between h-14">
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
        className={`flex-1 w-full max-w-6xl mx-auto px-4 sm:px-6 py-6 sm:py-10 ${
          center ? "flex items-center justify-center" : ""
        } ${tabBar ? "pb-[calc(5rem+env(safe-area-inset-bottom))] sm:pb-10" : ""}`}
      >
        {children}
      </main>
      {tabBar && (
        <nav
          className="sm:hidden fixed inset-x-0 bottom-0 z-30 bg-surface-primary/95 backdrop-blur border-t border-border-primary pb-[env(safe-area-inset-bottom)]"
          aria-label="Main"
        >
          {tabBar}
        </nav>
      )}
    </div>
  );
}
