import React, { useMemo, useState } from 'react';
import { Bell, Trash2, X } from 'lucide-react';
import { useApp } from '../context/AppContext';

export default function ToastContainer() {
  const { toasts, removeToast, clearToasts } = useApp();
  const [isOpen, setIsOpen] = useState(true);

  const icons = {
    success: '✓',
    error: '✕',
    info: 'ℹ',
    warning: '⚠'
  };

  const unreadCount = toasts.length;
  const emptyMessage = useMemo(() => {
    return 'No notifications yet.';
  }, []);

  return (
    <div className="notification-center" aria-live="polite">
      <button
        className="notification-toggle"
        onClick={() => setIsOpen(prev => !prev)}
        title={isOpen ? 'Hide notifications' : 'Show notifications'}
        aria-expanded={isOpen}
      >
        <Bell size={16} />
        <span>Notifications</span>
        {unreadCount > 0 && (
          <span className="notification-count">{unreadCount}</span>
        )}
      </button>

      {isOpen && (
        <div className="notification-panel">
          <div className="notification-header">
            <span className="notification-title">Notifications</span>
            <div className="notification-actions">
              <button
                className="notification-clear"
                onClick={clearToasts}
                disabled={toasts.length === 0}
                title="Clear all"
              >
                <Trash2 size={14} />
              </button>
              <button
                className="notification-close"
                onClick={() => setIsOpen(false)}
                title="Close"
              >
                <X size={14} />
              </button>
            </div>
          </div>

          {toasts.length === 0 && (
            <div className="notification-empty">{emptyMessage}</div>
          )}

          {toasts.length > 0 && (
            <div className="notification-list">
              {toasts.map(toast => (
                <div key={toast.id} className={`notification-item ${toast.type}`}>
                  <div className="notification-icon">{icons[toast.type] || 'ℹ'}</div>
                  <div className="notification-body">
                    <div className="notification-message">{toast.message}</div>
                    <div className="notification-time">
                      {toast.timestamp ? new Date(toast.timestamp).toLocaleTimeString() : ''}
                    </div>
                  </div>
                  <button
                    className="notification-dismiss"
                    onClick={() => removeToast(toast.id)}
                    title="Dismiss"
                  >
                    ×
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
