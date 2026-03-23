import React from 'react';
import { HashRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AppProvider } from './context/AppContext';
import Layout from './components/layout/Layout';
import LoginPage from './pages/LoginPage';
import NodesPage from './pages/NodesPage';
import NodeDetailPage from './pages/NodeDetailPage';
import PodMetricsPage from './pages/PodMetricsPage';
import StrategiesPage from './pages/StrategiesPage';
import IntentsPage from './pages/IntentsPage';
import UsersPage from './pages/UsersPage';
import RolesPage from './pages/RolesPage';
import SettingsPage from './pages/SettingsPage';

function App() {
  return (
    <AppProvider>
      <HashRouter>
        <Routes>
          {/* Public */}
          <Route path="/login" element={<LoginPage />} />

          {/* Authenticated layout */}
          <Route element={<Layout />}>
            <Route path="/nodes" element={<NodesPage />} />
            <Route path="/nodes/:nodeId" element={<NodeDetailPage />} />
            <Route path="/pod-metrics" element={<PodMetricsPage />} />
            <Route path="/strategies" element={<StrategiesPage />} />
            <Route path="/intents" element={<IntentsPage />} />
            <Route path="/users" element={<UsersPage />} />
            <Route path="/roles" element={<RolesPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>

          {/* Fallback */}
          <Route path="*" element={<Navigate to="/nodes" replace />} />
        </Routes>
      </HashRouter>
    </AppProvider>
  );
}

export default App;
