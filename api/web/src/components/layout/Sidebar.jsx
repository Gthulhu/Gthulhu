import React from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { useApp } from '../../context/AppContext';
import {
  Server,
  BarChart3,
  Target,
  Users,
  Shield,
  LogOut,
  Settings,
} from 'lucide-react';

const navSections = [
  {
    label: 'CLUSTER',
    items: [
      { to: '/nodes', icon: Server, label: 'Nodes & Health' },
      { to: '/pod-metrics', icon: BarChart3, label: 'Pod Metrics' },
      { to: '/strategies', icon: Target, label: 'Strategies', badge: 'Advanced' },
    ],
  },
  {
    label: 'ACCESS CONTROL',
    items: [
      { to: '/users', icon: Users, label: 'Users' },
      { to: '/roles', icon: Shield, label: 'Roles & Permissions' },
    ],
  },
  {
    label: 'SYSTEM',
    items: [
      { to: '/settings', icon: Settings, label: 'Settings' },
    ],
  },
];

export default function Sidebar() {
  const { currentUser, logout } = useApp();
  const location = useLocation();

  const username = currentUser?.username || 'admin';
  const initial = username.charAt(0).toUpperCase();
  const roles = currentUser?.roles || [];
  const roleLabel = roles.length > 0 ? roles[0] : 'User';

  return (
    <aside className="sidebar">
      {/* Logo */}
      <div className="sidebar-logo">
        <img src="/logo.svg" alt="Gthulhu" className="sidebar-logo-icon" />
        <span className="sidebar-logo-text">Gthulhu</span>
      </div>

      {/* Navigation */}
      <nav className="sidebar-nav">
        {navSections.map((section) => (
          <div key={section.label}>
            <div className="sidebar-section-label">{section.label}</div>
            {section.items.map((item) => {
              const Icon = item.icon;
              const isActive =
                location.pathname === item.to ||
                location.pathname.startsWith(item.to + '/');
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={`sidebar-nav-item${isActive ? ' active' : ''}`}
                >
                  <Icon size={18} />
                  <span>{item.label}</span>
                  {item.badge && <span className="sidebar-nav-badge">{item.badge}</span>}
                </NavLink>
              );
            })}
          </div>
        ))}
      </nav>

      {/* Footer: User + Logout */}
      <div className="sidebar-footer">
        <div className="sidebar-user">
          <div className="sidebar-user-avatar">{initial}</div>
          <div className="sidebar-user-info">
            <div className="sidebar-user-name">{username}</div>
            <div className="sidebar-user-role">{roleLabel}</div>
          </div>
        </div>
        <button className="sidebar-logout-btn" onClick={logout}>
          <LogOut size={14} />
          <span>Logout</span>
        </button>
      </div>
    </aside>
  );
}
