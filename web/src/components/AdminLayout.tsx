import type { ReactNode } from "react";
import { Link, Outlet, useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { AppShell } from "./AppShell";
import { headerItemClass } from "./headerItemClass";
import { useActiveAlerts } from "../api/machine";
import { useBackupStatus } from "../api/backup";

// Outline icons for the phone tab bar, 24px on a 24 grid.
const icon = (d: ReactNode) => (
  <svg
    aria-hidden
    viewBox="0 0 24 24"
    className="w-6 h-6"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.8"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    {d}
  </svg>
);

const navLinks = [
  {
    to: "/admin",
    label: "Dashboard",
    end: true,
    icon: icon(<path d="M3 11.5 12 4l9 7.5M5.5 9.5V20h13V9.5" />),
  },
  {
    to: "/admin/analytics",
    label: "Analytics",
    icon: icon(<path d="M4 20V10M10 20V4M16 20v-7M22 20H2" />),
  },
  {
    to: "/admin/alerts",
    label: "Alerts",
    icon: icon(
      <path d="M6 16V11a6 6 0 1 1 12 0v5l1.5 2h-15L6 16ZM10 21h4" />,
    ),
  },
  {
    to: "/admin/settings",
    label: "Settings",
    icon: icon(
      <>
        <circle cx="12" cy="12" r="3" />
        <path d="M12 2.5v2.2M12 19.3v2.2M4.7 4.7l1.6 1.6M17.7 17.7l1.6 1.6M2.5 12h2.2M19.3 12h2.2M4.7 19.3l1.6-1.6M17.7 6.3l1.6-1.6" />
      </>,
    ),
  },
];

function AlertBadge({ count, className = "" }: { count: number; className?: string }) {
  if (count === 0) return null;
  return (
    <span
      className={`min-w-[1.125rem] h-[1.125rem] px-1 rounded-full bg-status-empty text-white text-[11px] font-semibold leading-[1.125rem] text-center tabular-nums ${className}`}
      aria-label={`${count} active alert${count === 1 ? "" : "s"}`}
    >
      {count}
    </span>
  );
}

export function AdminLayout() {
  const { logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  // A failing backup counts as an alert too; the Alerts page shows it.
  const alertCount =
    (useActiveAlerts().data?.length ?? 0) + (useBackupStatus().data?.stale ? 1 : 0);

  const handleLogout = () => {
    logout();
    navigate("/admin/login");
  };

  const isActive = (to: string, end?: boolean) => {
    if (end) return location.pathname === to;
    return location.pathname.startsWith(to);
  };

  return (
    <AppShell
      nav={
        <nav className="hidden sm:flex items-center gap-1">
          {navLinks.map((link) => (
            <Link
              key={link.to}
              to={link.to}
              className={`${headerItemClass(isActive(link.to, link.end))} inline-flex items-center gap-1.5`}
            >
              {link.label}
              {link.to === "/admin/alerts" && <AlertBadge count={alertCount} />}
            </Link>
          ))}
        </nav>
      }
      actions={
        <button onClick={handleLogout} className={headerItemClass()}>
          Logout
        </button>
      }
      tabBar={
        <div className="grid grid-cols-4">
          {navLinks.map((link) => {
            const active = isActive(link.to, link.end);
            return (
              <Link
                key={link.to}
                to={link.to}
                aria-current={active ? "page" : undefined}
                className={`relative flex flex-col items-center gap-0.5 pt-2 pb-1.5 text-[11px] font-medium transition-colors duration-150 ${
                  active ? "text-accent" : "text-text-tertiary"
                }`}
              >
                {link.icon}
                {link.label}
                {link.to === "/admin/alerts" && (
                  <AlertBadge count={alertCount} className="absolute top-1 left-1/2 ml-1.5" />
                )}
              </Link>
            );
          })}
        </div>
      }
    >
      <Outlet />
    </AppShell>
  );
}
