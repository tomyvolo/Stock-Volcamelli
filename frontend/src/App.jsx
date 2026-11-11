import { Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import ProtectedRoute from "./components/ProtectedRoute";
import RoleRoute from "./components/RoleRoute";
import LoginPage from "./pages/LoginPage";
import StockPage from "./pages/StockPage";
import ProductsPage from "./pages/ProductsPage";
import ProductDetailPage from "./pages/ProductDetailPage";
import ProductFormPage from "./pages/ProductFormPage";
import MovementsPage from "./pages/MovementsPage";
import TransfersListPage from "./pages/TransfersListPage";
import TransferFormPage from "./pages/TransferFormPage";
import TransferDetailPage from "./pages/TransferDetailPage";
import BranchesPage from "./pages/BranchesPage";
import BranchFormPage from "./pages/BranchFormPage";
import UsersPage from "./pages/UsersPage";
import UserFormPage from "./pages/UserFormPage";
import NotFoundPage from "./pages/NotFoundPage";
import { ADMIN_SUCURSAL, DIRECTOR } from "./utils/roles";

function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<ProtectedRoute><StockPage /></ProtectedRoute>} />

        <Route path="/productos" element={<ProtectedRoute><ProductsPage /></ProtectedRoute>} />
        <Route path="/productos/nuevo" element={<RoleRoute roles={[DIRECTOR]}><ProductFormPage /></RoleRoute>} />
        <Route path="/productos/:id" element={<ProtectedRoute><ProductDetailPage /></ProtectedRoute>} />
        <Route path="/productos/:id/editar" element={<RoleRoute roles={[DIRECTOR]}><ProductFormPage /></RoleRoute>} />
        <Route
          path="/sucursales/:branchId/productos/:productId"
          element={<ProtectedRoute><MovementsPage /></ProtectedRoute>}
        />

        <Route path="/transferencias" element={<ProtectedRoute><TransfersListPage /></ProtectedRoute>} />
        <Route path="/transferencias/nueva" element={<ProtectedRoute><TransferFormPage /></ProtectedRoute>} />
        <Route path="/transferencias/:id" element={<ProtectedRoute><TransferDetailPage /></ProtectedRoute>} />

        <Route path="/sucursales" element={<ProtectedRoute><BranchesPage /></ProtectedRoute>} />
        <Route path="/sucursales/nueva" element={<RoleRoute roles={[DIRECTOR]}><BranchFormPage /></RoleRoute>} />
        <Route path="/sucursales/:id/editar" element={<RoleRoute roles={[DIRECTOR]}><BranchFormPage /></RoleRoute>} />

        <Route path="/usuarios" element={<ProtectedRoute><UsersPage /></ProtectedRoute>} />
        <Route
          path="/usuarios/nuevo"
          element={<RoleRoute roles={[DIRECTOR, ADMIN_SUCURSAL]}><UserFormPage /></RoleRoute>}
        />
        <Route path="/usuarios/:id/editar" element={<RoleRoute roles={[DIRECTOR]}><UserFormPage /></RoleRoute>} />

        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}

export default App;
