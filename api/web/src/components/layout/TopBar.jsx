import React from 'react';
import { useLocation, Link } from 'react-router-dom';
import { Bell } from 'lucide-react';

const routeTitles = {
  '/nodes': 'Nodes & Health',
  '/pod-metrics': 'Pod Metrics',
  '/strategies': 'Strategies',
  '/intents': 'Intents',
  '/users': 'Users',
  '/roles': 'Roles & Permissions',
  '/settings': 'Settings',
};

function getBreadcrumb(pathname) {
  if (pathname.startsWith('/nodes/')) {
    const nodeId = decodeURIComponent(pathname.split('/')[2]);
    return [
      { label: 'Nodes & Health', to: '/nodes' },
      { label: nodeId, to: null },
    ];
  }
  const title = routeTitles[pathname];
  if (title) {
    return [{ label: title, to: null }];
  }
  return [{ label: 'Dashboard', to: null }];
}

export default function TopBar({ onToggleNotifications, unreadCount = 0 }) {
  const location = useLocation();
  const crumbs = getBreadcrumb(location.pathname);

  return (
    <header className="topbar">
      <div className="topbar-breadcrumb">
        {crumbs.map((crumb, i) => (
          <React.Fragment key={i}>
            {i > 0 && <span className="breadcrumb-sep">/</span>}
            {crumb.to ? (
              <Link to={crumb.to}>{crumb.label}</Link>
            ) : (
              <span className="breadcrumb-current">{crumb.label}</span>
            )}
          </React.Fragment>
        ))}
      </div>
      <div className="topbar-right">
        <button
          className="topbar-icon-btn"
          onClick={onToggleNotifications}
          title="Notifications"
        >
          <Bell size={18} />
          {unreadCount > 0 && <span className="notification-dot" />}
        </button>
      </div>
    </header>
  );
}
