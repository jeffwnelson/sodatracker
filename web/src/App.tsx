import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider } from "./context/AuthContext";
import { LiveUpdatesProvider } from "./context/LiveUpdates";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { AdminLayout } from "./components/AdminLayout";
import { CustomerDashboard } from "./pages/CustomerDashboard";
import { LoginPage } from "./pages/LoginPage";
import { AdminDashboard } from "./pages/AdminDashboard";
import { StatusPage } from "./pages/StatusPage";
import { SettingsPage } from "./pages/SettingsPage";
import { AnalyticsLayout } from "./pages/analytics/AnalyticsLayout";
import { ConsumptionPage } from "./pages/analytics/ConsumptionPage";
import { ProductsPage } from "./pages/analytics/ProductsPage";
import { InventoryPage } from "./pages/analytics/InventoryPage";
import { SpendPage } from "./pages/analytics/SpendPage";
import { PredictionsPage } from "./pages/analytics/PredictionsPage";
import { HighlightsPage } from "./pages/analytics/HighlightsPage";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5_000,
      retry: 1,
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <LiveUpdatesProvider>
          <BrowserRouter>
            <Routes>
              <Route path="/" element={<CustomerDashboard />} />
              <Route path="/admin/login" element={<LoginPage />} />
              <Route
                path="/admin"
                element={
                  <ProtectedRoute>
                    <AdminLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<AdminDashboard />} />
                <Route path="restock" element={<Navigate to="/admin/settings" replace />} />
                {/* Old machine detail and restock links */}
                <Route path="machines/*" element={<Navigate to="/admin" replace />} />
                <Route path="analytics" element={<AnalyticsLayout />}>
                  <Route index element={<Navigate to="consumption" replace />} />
                  <Route path="consumption" element={<ConsumptionPage />} />
                  <Route path="products" element={<ProductsPage />} />
                  <Route path="inventory" element={<InventoryPage />} />
                  <Route path="spend" element={<SpendPage />} />
                  <Route path="predictions" element={<PredictionsPage />} />
                  <Route path="highlights" element={<HighlightsPage />} />
                  {/* Uptime moved to the Status tab. */}
                  <Route path="uptime" element={<Navigate to="/admin/status" replace />} />
                </Route>
                <Route path="status" element={<StatusPage />} />
                <Route path="alerts" element={<Navigate to="/admin/status" replace />} />
                <Route path="settings" element={<SettingsPage />} />
                {/* Old bookmarks */}
                <Route path="setup" element={<Navigate to="/admin/settings" replace />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </LiveUpdatesProvider>
      </AuthProvider>
    </QueryClientProvider>
  );
}

export default App;
