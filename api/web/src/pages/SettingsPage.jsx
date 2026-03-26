import React, { useState, useEffect, useCallback } from 'react';
import { useApp } from '../context/AppContext';
import { Settings, Save, User, KeyRound, Cpu, RefreshCw, CheckCircle, XCircle, AlertCircle } from 'lucide-react';

export default function SettingsPage() {
  const { apiBaseUrl, saveApiConfig, showToast, currentUser, makeAuthenticatedRequest, applyRuntimeConfig, getRuntimeConfigStatus } = useApp();
  const [configUrl, setConfigUrl] = useState(apiBaseUrl);

  /* change password */
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changingPw, setChangingPw] = useState(false);

  /* runtime config */
  const [runtimeConfig, setRuntimeConfig] = useState({
    schedulerEnabled: true,
    monitoringEnabled: true,
    mode: 'gthulhu',
    sliceNsDefault: 20000000,
    sliceNsMin: 1000000,
    kernelMode: true,
    maxTimeWatchdog: true,
    earlyProcessing: false,
    builtinIdle: false,
  });
  const [nodeStatuses, setNodeStatuses] = useState([]);
  const [applying, setApplying] = useState(false);
  const [loadingStatus, setLoadingStatus] = useState(false);

  const handleSave = () => {
    saveApiConfig(configUrl);
    showToast('success', 'API Base URL updated');
  };

  const fetchNodeStatuses = useCallback(async () => {
    setLoadingStatus(true);
    try {
      const results = await getRuntimeConfigStatus();
      setNodeStatuses(results);
    } catch (err) {
      showToast('error', err.message);
    } finally {
      setLoadingStatus(false);
    }
  }, [getRuntimeConfigStatus, showToast]);

  useEffect(() => {
    fetchNodeStatuses();
  }, [fetchNodeStatuses]);

  const handleApplyRuntimeConfig = async () => {
    setApplying(true);
    try {
      const configVersion = new Date().toISOString();
      const results = await applyRuntimeConfig([], { ...runtimeConfig, configVersion });
      setNodeStatuses(results);
      showToast('success', 'Runtime config applied successfully');
    } catch (err) {
      showToast('error', err.message);
    } finally {
      setApplying(false);
    }
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
      {/* Scheduler Runtime Config */}
      <div className="card">
        <div className="card-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h3 className="card-title">
            <Cpu size={16} />
            Scheduler Runtime Config
          </h3>
          <button
            className="btn btn-sm"
            onClick={fetchNodeStatuses}
            disabled={loadingStatus}
            style={{ display: 'flex', alignItems: 'center', gap: 4 }}
            title="Refresh node statuses"
          >
            <RefreshCw size={14} style={{ animation: loadingStatus ? 'spin 1s linear infinite' : 'none' }} />
          </button>
        </div>
        <div className="card-body" style={{ padding: 'var(--space-xl)' }}>
          {/* Toggle row */}
          <div style={{ display: 'flex', gap: 24, marginBottom: 20, flexWrap: 'wrap' }}>
            {[
              { key: 'schedulerEnabled', label: 'Scheduler Enabled' },
              { key: 'monitoringEnabled', label: 'Monitoring Enabled' },
              { key: 'kernelMode', label: 'Kernel Mode' },
              { key: 'maxTimeWatchdog', label: 'Max-Time Watchdog' },
              { key: 'earlyProcessing', label: 'Early Processing' },
              { key: 'builtinIdle', label: 'Built-in Idle' },
            ].map(({ key, label }) => (
              <label key={key} style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', userSelect: 'none' }}>
                <input
                  type="checkbox"
                  checked={runtimeConfig[key]}
                  onChange={(e) => setRuntimeConfig(prev => ({ ...prev, [key]: e.target.checked }))}
                />
                <span style={{ fontSize: 13 }}>{label}</span>
              </label>
            ))}
          </div>

          {/* Form fields */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 16, marginBottom: 20 }}>
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="form-label">Mode</label>
              <select
                className="form-input"
                value={runtimeConfig.mode}
                onChange={(e) => setRuntimeConfig(prev => ({ ...prev, mode: e.target.value }))}
              >
                <option value="gthulhu">gthulhu</option>
                <option value="simple">simple</option>
                <option value="simple-fifo">simple-fifo</option>
              </select>
            </div>
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="form-label">Default Slice (ns)</label>
              <input
                type="number"
                className="form-input"
                step={1000000}
                min={1000000}
                value={runtimeConfig.sliceNsDefault}
                onChange={(e) => setRuntimeConfig(prev => ({ ...prev, sliceNsDefault: Number(e.target.value) }))}
              />
            </div>
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="form-label">Min Slice (ns)</label>
              <input
                type="number"
                className="form-input"
                step={100000}
                min={100000}
                value={runtimeConfig.sliceNsMin}
                onChange={(e) => setRuntimeConfig(prev => ({ ...prev, sliceNsMin: Number(e.target.value) }))}
              />
            </div>
          </div>

          <div style={{ marginBottom: 24 }}>
            <button
              className="btn btn-primary btn-sm"
              onClick={handleApplyRuntimeConfig}
              disabled={applying}
            >
              <Save size={14} />
              <span>{applying ? 'Applying…' : 'Apply to All Nodes'}</span>
            </button>
          </div>

          {/* Per-node status table */}
          {nodeStatuses.length > 0 && (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--color-border)' }}>
                    {['Node ID', 'Host', 'Config Version', 'Applied At', 'Restarts', 'Status'].map(h => (
                      <th key={h} style={{ textAlign: 'left', padding: '6px 10px', color: 'var(--color-text-secondary)', fontWeight: 600 }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {nodeStatuses.map((n, i) => (
                    <tr key={i} style={{ borderBottom: '1px solid var(--color-border-light)' }}>
                      <td style={{ padding: '6px 10px', fontFamily: 'monospace', fontSize: 11 }}>{n.nodeId}</td>
                      <td style={{ padding: '6px 10px' }}>{n.host || '—'}</td>
                      <td style={{ padding: '6px 10px', fontFamily: 'monospace', fontSize: 11 }}>{n.configVersion || '—'}</td>
                      <td style={{ padding: '6px 10px', fontSize: 11 }}>{n.appliedAt ? new Date(n.appliedAt).toLocaleString() : '—'}</td>
                      <td style={{ padding: '6px 10px', textAlign: 'center' }}>{n.restartCount ?? '—'}</td>
                      <td style={{ padding: '6px 10px' }}>
                        {n.success ? (
                          <span style={{ display: 'flex', alignItems: 'center', gap: 4, color: 'var(--color-success)' }}>
                            <CheckCircle size={14} /> OK
                          </span>
                        ) : n.lastError ? (
                          <span style={{ display: 'flex', alignItems: 'center', gap: 4, color: 'var(--color-error)' }} title={n.lastError}>
                            <XCircle size={14} /> {n.lastError.length > 40 ? n.lastError.slice(0, 40) + '…' : n.lastError}
                          </span>
                        ) : (
                          <span style={{ display: 'flex', alignItems: 'center', gap: 4, color: 'var(--color-text-secondary)' }}>
                            <AlertCircle size={14} /> Unknown
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {nodeStatuses.length === 0 && !loadingStatus && (
            <p style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>No node status available.</p>
          )}
        </div>
      </div>
    </div>
  );
}
