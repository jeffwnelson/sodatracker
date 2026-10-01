import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider } from "./context/AuthContext";
import { LiveUpdatesProvider } from "./context/LiveUpdates";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { AdminLayout } from "./components/AdminLayout";
import { CustomerDashboard } from "./pages/CustomerDashboard";
import { LoginPage } from "./pages/LoginPage";
import { AdminDashboard } from "./pages/AdminDashboard";
import { MachineDetailPage } from "./pages/MachineDetailPage";
import { RestockPage } from "./pages/RestockPage";
import { AlertsPage } from "./pages/AlertsPage";
import { SetupPage } from "./pages/SetupPage";
import { AnalyticsLayout } from "./pages/analytics/AnalyticsLayout";
import { ConsumptionPage } from "./pages/analytics/ConsumptionPage";
import { ProductsPage } from "./pages/analytics/ProductsPage";
import { InventoryPage } from "./pages/analytics/InventoryPage";
import { SpendPage } from "./pages/analytics/SpendPage";
import { PredictionsPage } from "./pages/analytics/PredictionsPage";

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
                <Route path="machines/:id" element={<MachineDetailPage />} />
                <Route
                  path="machines/:id/restock"
                  element={<RestockPage />}
                />
                <Route path="analytics" element={<AnalyticsLayout />}>
                  <Route index element={<Navigate to="consumption" replace />} />
                  <Route path="consumption" element={<ConsumptionPage />} />
                  <Route path="products" element={<ProductsPage />} />
                  <Route path="inventory" element={<InventoryPage />} />
                  <Route path="spend" element={<SpendPage />} />
                  <Route path="predictions" element={<PredictionsPage />} />
                </Route>
                <Route path="alerts" element={<AlertsPage />} />
                <Route path="setup" element={<SetupPage />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </LiveUpdatesProvider>
      </AuthProvider>
    </QueryClientProvider>
  );
}

export default App;
