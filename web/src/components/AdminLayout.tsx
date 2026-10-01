import { Link, Outlet, useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { AppShell } from "./AppShell";
import { headerItemClass } from "./headerItemClass";

const navLinks = [
  { to: "/admin", label: "Dashboard", end: true },
  { to: "/admin/analytics", label: "Analytics" },
  { to: "/admin/alerts", label: "Alerts" },
  { to: "/admin/settings", label: "Settings" },
];

export function AdminLayout() {
  const { logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

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
              className={headerItemClass(isActive(link.to, link.end))}
            >
              {link.label}
            </Link>
          ))}
        </nav>
      }
      actions={
        <button onClick={handleLogout} className={headerItemClass()}>
          Logout
        </button>
      }
    >
      <Outlet />
    </AppShell>
  );
}
