import React, { useState } from 'react';
import { Outlet, Navigate } from 'react-router-dom';
import { useApp } from '../../context/AppContext';
import Sidebar from './Sidebar';
import TopBar from './TopBar';
import NotificationCenter from '../NotificationCenter';

export default function Layout() {
  const { isAuthenticated, toasts } = useApp();
  const [showNotifications, setShowNotifications] = useState(false);

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  const unreadCount = toasts.length;

  return (
    <div className="app-shell">
      <Sidebar />
      <div className="main-content">
        <TopBar
          onToggleNotifications={() => setShowNotifications((v) => !v)}
          unreadCount={unreadCount}
        />
        <main className="page-content">
          <Outlet />
        </main>
      </div>
      <NotificationCenter
        open={showNotifications}
        onClose={() => setShowNotifications(false)}
      />
    </div>
  );
}
