import React, { useState } from 'react';
import { useApp } from '../context/AppContext';
import { Settings, Save, User, KeyRound } from 'lucide-react';

export default function SettingsPage() {
  const { apiBaseUrl, saveApiConfig, showToast, currentUser, makeAuthenticatedRequest } = useApp();
  const [configUrl, setConfigUrl] = useState(apiBaseUrl);

  /* change password */
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changingPw, setChangingPw] = useState(false);

  const handleSave = () => {
    saveApiConfig(configUrl);
    showToast('success', 'API Base URL updated');
  };

  const handleChangePassword = async () => {
    if (!oldPassword.trim() || !newPassword.trim()) {
      showToast('error', 'Please fill in both old and new password');
      return;
    }
    if (newPassword !== confirmPassword) {
      showToast('error', 'New passwords do not match');
      return;
    }
    if (newPassword.length < 6) {
      showToast('error', 'New password must be at least 6 characters');
      return;
    }
    setChangingPw(true);
    try {
      const res = await makeAuthenticatedRequest('/api/v1/users/self/password', {
        method: 'PUT',
        body: JSON.stringify({ oldPassword, newPassword }),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', 'Password changed successfully');
        setOldPassword('');
        setNewPassword('');
        setConfirmPassword('');
      } else {
        showToast('error', data.error || data.message || 'Failed to change password');
      }
    } catch (err) {
      showToast('error', err.message);
    } finally {
      setChangingPw(false);
    }
  };

  const username = currentUser?.username || 'admin';
  const roles = currentUser?.roles || [];

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Settings</h1>
          <p className="page-subtitle">Application configuration and user profile</p>
        </div>
      </div>

      {/* User profile */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <User size={16} />
            Current User
          </h3>
        </div>
        <div className="card-body">
          <div className="detail-grid">
            <div className="detail-item">
              <span className="detail-label">Username</span>
              <span className="detail-value">{username}</span>
            </div>
            <div className="detail-item">
              <span className="detail-label">Roles</span>
              <span className="detail-value">
                {roles.length > 0 ? (
                  roles.map((r, i) => (
                    <span key={i} className="badge badge-primary" style={{ marginRight: 4 }}>
                      {r}
                    </span>
                  ))
                ) : (
                  <span style={{ color: 'var(--color-text-secondary)' }}>No roles assigned</span>
                )}
              </span>
            </div>
            {currentUser?.id && (
              <div className="detail-item">
                <span className="detail-label">User ID</span>
                <span className="detail-value" style={{ fontFamily: 'monospace', fontSize: 12 }}>
                  {currentUser.id}
                </span>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Change Password */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <KeyRound size={16} />
            Change Password
          </h3>
        </div>
        <div className="card-body" style={{ padding: 'var(--space-xl)' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 400 }}>
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="form-label">Current Password</label>
              <input
                type="password"
                className="form-input"
                placeholder="Enter current password"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
              />
            </div>
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="form-label">New Password</label>
              <input
                type="password"
                className="form-input"
                placeholder="Enter new password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </div>
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="form-label">Confirm New Password</label>
              <input
                type="password"
                className="form-input"
                placeholder="Confirm new password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </div>
            <div>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleChangePassword}
                disabled={changingPw}
              >
                <Save size={14} />
                <span>{changingPw ? 'Changing...' : 'Change Password'}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* API Config */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <Settings size={16} />
            API Configuration
          </h3>
        </div>
        <div className="card-body" style={{ padding: 'var(--space-xl)' }}>
          <div style={{ maxWidth: 400 }}>
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="form-label">API Base URL</label>
              <input
                type="text"
                className="form-input"
                value={configUrl}
                onChange={(e) => setConfigUrl(e.target.value)}
                placeholder="http://localhost:8080"
              />
              <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 4 }}>
                Leave empty to use the current origin.
              </p>
            </div>
            <div style={{ marginTop: 12 }}>
              <button className="btn btn-primary btn-sm" onClick={handleSave}>
                <Save size={14} />
                <span>Save</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
