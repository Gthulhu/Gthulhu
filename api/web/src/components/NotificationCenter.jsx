import React from 'react';
import { X, Trash2, CheckCircle, AlertTriangle, XCircle, Info } from 'lucide-react';
import { useApp } from '../context/AppContext';

const iconMap = {
  success: CheckCircle,
  warning: AlertTriangle,
  error: XCircle,
  info: Info,
};

export default function NotificationCenter({ open, onClose }) {
  const { toasts, removeToast, clearToasts } = useApp();

  return (
    <>
      {open && <div className="slide-panel-overlay" onClick={onClose} />}
      <div className={`notification-center${open ? ' open' : ''}`}>
        <div className="notification-center-header">
          <h3>Notifications</h3>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            {toasts.length > 0 && (
              <button className="btn btn-ghost btn-sm" onClick={clearToasts}>
                <Trash2 size={14} />
                <span>Clear all</span>
              </button>
            )}
            <button className="btn-icon" onClick={onClose}>
              <X size={18} />
            </button>
          </div>
        </div>
        <div className="notification-center-body">
          {toasts.length === 0 ? (
            <div className="empty-state">
              <p>No notifications</p>
            </div>
          ) : (
            toasts.map((toast) => {
              const Icon = iconMap[toast.type] || Info;
              return (
                <div
                  key={toast.id}
                  className={`notification-item notification-${toast.type}`}
                >
                  <Icon size={16} />
                  <span className="notification-message">{toast.message}</span>
                  <button
                    className="btn-icon"
                    onClick={() => removeToast(toast.id)}
                  >
                    <X size={14} />
                  </button>
                </div>
              );
            })
          )}
        </div>
      </div>
    </>
  );
}
