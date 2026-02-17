import { Link, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { useQuery } from "@tanstack/react-query";
import { listMachines } from "../api/client";

export function AdminLayout() {
  const { logout } = useAuth();
  const navigate = useNavigate();
  const { data: machines } = useQuery({
    queryKey: ["machines"],
    queryFn: listMachines,
  });

  const handleLogout = () => {
    logout();
    navigate("/admin/login");
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-gray-900 text-white shadow">
        <div className="max-w-6xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <Link to="/admin" className="text-lg font-bold hover:text-gray-300">
              SodaTracker Admin
            </Link>
            <nav className="hidden sm:flex items-center gap-4 text-sm text-gray-300">
              <Link to="/admin" className="hover:text-white">
                Dashboard
              </Link>
              <Link to="/admin/alerts" className="hover:text-white">
                Alerts
              </Link>
              {machines?.map((m) => (
                <Link
                  key={m.id}
                  to={`/admin/machines/${m.id}`}
                  className="hover:text-white"
                >
                  {m.name}
                </Link>
              ))}
            </nav>
          </div>
          <div className="flex items-center gap-4">
            <Link to="/" className="text-sm text-gray-400 hover:text-white">
              Customer View
            </Link>
            <button
              onClick={handleLogout}
              className="text-sm bg-gray-700 hover:bg-gray-600 px-3 py-1.5 rounded"
            >
              Logout
            </button>
          </div>
        </div>
      </header>
      <main className="max-w-6xl mx-auto px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}
