import React, { useState, useEffect, useCallback } from 'react';
import { useApp } from '../context/AppContext';
import SlidePanel from '../components/SlidePanel';
import {
  Users as UsersIcon,
  Plus,
  RefreshCw,
  Pencil,
  Save,
  Loader2,
  Inbox,
  KeyRound,
} from 'lucide-react';

export default function UsersPage() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();
  const [users, setUsers] = useState([]);
  const [roles, setRoles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  /* panels */
  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [resetPwOpen, setResetPwOpen] = useState(false);

  /* create form */
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');

  /* edit form */
  const [editUser, setEditUser] = useState(null);
  const [editRoles, setEditRoles] = useState([]);
  const [editStatus, setEditStatus] = useState(1);

  /* reset password form */
  const [resetUserId, setResetUserId] = useState('');
  const [resetUsername, setResetUsername] = useState('');
  const [resetNewPw, setResetNewPw] = useState('');

  const fetchUsers = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoading(true);
    setError('');
    try {
      const res = await makeAuthenticatedRequest('/api/v1/users');
      const data = await res.json();
      if (data.success) {
        setUsers(data.data?.users || []);
      } else {
        setError(data.error || 'Failed');
        setUsers([]);
      }
    } catch (err) {
      setError(err.message);
      setUsers([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest]);

  const fetchRoles = useCallback(async () => {
    if (!isAuthenticated) return;
    try {
      const res = await makeAuthenticatedRequest('/api/v1/roles');
      const data = await res.json();
      if (data.success) {
        setRoles(data.data?.roles || []);
      }
    } catch {
      /* silent */
    }
  }, [isAuthenticated, makeAuthenticatedRequest]);

  useEffect(() => {
    if (isAuthenticated) {
      fetchUsers();
      fetchRoles();
    }
  }, [isAuthenticated, fetchUsers, fetchRoles]);

  /* ─── create user ─── */
  const openCreate = () => {
    setNewUsername('');
    setNewPassword('');
    setCreateOpen(true);
  };

  const handleCreate = async () => {
    if (!newUsername.trim() || !newPassword.trim()) {
      showToast('error', 'Username and password are required');
      return;
    }
    try {
      const res = await makeAuthenticatedRequest('/api/v1/users', {
        method: 'POST',
        body: JSON.stringify({ username: newUsername.trim(), password: newPassword }),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', 'User created');
        setCreateOpen(false);
        fetchUsers();
      } else {
        showToast('error', data.error || data.message || 'Failed');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  /* ─── edit user (roles & status) ─── */
  const openEdit = (user) => {
    setEditUser(user);
    setEditRoles(user.roles || []);
    setEditStatus(user.status || 1);
    setEditOpen(true);
  };

  const toggleEditRole = (roleName) => {
    setEditRoles((prev) =>
      prev.includes(roleName) ? prev.filter((r) => r !== roleName) : [...prev, roleName]
    );
  };

  const handleUpdateUser = async () => {
    if (!editUser) return;
    const payload = { userID: editUser.id };
    payload.roles = editRoles;
    payload.status = parseInt(editStatus, 10);
    try {
      const res = await makeAuthenticatedRequest('/api/v1/users/permissions', {
        method: 'PUT',
        body: JSON.stringify(payload),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', 'User updated');
        setEditOpen(false);
        fetchUsers();
      } else {
        showToast('error', data.error || 'Failed');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  /* ─── reset password ─── */
  const openResetPw = (user) => {
    setResetUserId(user.id);
    setResetUsername(user.username);
    setResetNewPw('');
    setResetPwOpen(true);
  };

  const handleResetPassword = async () => {
    if (!resetNewPw.trim()) {
      showToast('error', 'New password is required');
      return;
    }
    try {
      const res = await makeAuthenticatedRequest('/api/v1/users/password/reset', {
        method: 'PUT',
        body: JSON.stringify({ userID: resetUserId, newPassword: resetNewPw }),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', `Password reset for ${resetUsername}`);
        setResetPwOpen(false);
      } else {
        showToast('error', data.error || 'Failed');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  const statusLabel = { 1: 'Active', 2: 'Inactive', 3: 'Pending' };
  const statusBadge = { 1: 'badge-success', 2: 'badge-secondary', 3: 'badge-warning' };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Users</h1>
          <p className="page-subtitle">Manage system users and their assigned roles</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-secondary btn-sm" onClick={fetchUsers}>
            <RefreshCw size={14} />
            <span>Refresh</span>
          </button>
          <button className="btn btn-primary btn-sm" onClick={openCreate}>
            <Plus size={14} />
            <span>New User</span>
          </button>
        </div>
      </div>

      <div className="stat-cards">
        <div className="stat-card">
          <div className="stat-card-label">Total Users</div>
          <div className="stat-card-value">{users.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Active Users</div>
          <div className="stat-card-value">
            {users.filter((u) => u.status === 1 || u.status === undefined).length}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Inactive Users</div>
          <div className="stat-card-value">
            {users.filter((u) => u.status === 2).length}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Available Roles</div>
          <div className="stat-card-value">{roles.length}</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <UsersIcon size={16} />
            System Users
          </h3>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {loading ? (
            <div className="empty-state">
              <Loader2 size={20} className="spin" />
              <p>Loading users...</p>
            </div>
          ) : error ? (
            <div className="empty-state">
              <p>{error}</p>
            </div>
          ) : users.length === 0 ? (
            <div className="empty-state">
              <Inbox size={20} />
              <p>No users found</p>
            </div>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>USERNAME</th>
                  <th>USER ID</th>
                  <th>STATUS</th>
                  <th>ROLES</th>
                  <th>ACTIONS</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => (
                  <tr key={user.id}>
                    <td style={{ fontWeight: 500 }}>{user.username}</td>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{user.id}</td>
                    <td>
                      <span className={`badge ${statusBadge[user.status] || 'badge-success'}`}>
                        {statusLabel[user.status] || 'Active'}
                      </span>
                    </td>
                    <td>
                      {(user.roles || []).map((role, i) => (
                        <span key={i} className="badge badge-primary" style={{ marginRight: 4 }}>
                          {role}
                        </span>
                      ))}
                      {(!user.roles || user.roles.length === 0) && (
                        <span style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>No roles</span>
                      )}
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn btn-ghost btn-sm" title="Edit roles & status" onClick={() => openEdit(user)}>
                          <Pencil size={14} />
                        </button>
                        <button className="btn btn-ghost btn-sm" title="Reset password" onClick={() => openResetPw(user)}>
                          <KeyRound size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Create User Panel */}
      <SlidePanel open={createOpen} onClose={() => setCreateOpen(false)} title="New User">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="form-group">
            <label className="form-label">Username</label>
            <input
              className="form-input"
              placeholder="Enter username"
              value={newUsername}
              onChange={(e) => setNewUsername(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Password</label>
            <input
              className="form-input"
              type="password"
              placeholder="Enter password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
            />
          </div>
          <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
            <button className="btn btn-secondary" onClick={() => setCreateOpen(false)} style={{ flex: 1 }}>
              Cancel
            </button>
            <button className="btn btn-primary" onClick={handleCreate} style={{ flex: 1 }}>
              <Save size={14} />
              <span>Create User</span>
            </button>
          </div>
        </div>
      </SlidePanel>

      {/* Edit User Panel */}
      <SlidePanel open={editOpen} onClose={() => setEditOpen(false)} title={`Edit User: ${editUser?.username || ''}`}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="form-group">
            <label className="form-label">Status</label>
            <select
              className="form-input"
              value={editStatus}
              onChange={(e) => setEditStatus(e.target.value)}
            >
              <option value={1}>Active</option>
              <option value={2}>Inactive</option>
              <option value={3}>Pending Password Change</option>
            </select>
          </div>

          <div>
            <label className="form-label">Assigned Roles</label>
            {roles.length === 0 ? (
              <p style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>No roles available. Create roles first.</p>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                {roles.map((role) => (
                  <label key={role.id} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, padding: '6px 0' }}>
                    <input
                      type="checkbox"
                      checked={editRoles.includes(role.name)}
                      onChange={() => toggleEditRole(role.name)}
                    />
                    <span style={{ fontWeight: 500 }}>{role.name}</span>
                    {role.description && (
                      <span style={{ color: 'var(--color-text-secondary)', fontSize: 12 }}>
                        -- {role.description}
                      </span>
                    )}
                  </label>
                ))}
              </div>
            )}
          </div>

          <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
            <button className="btn btn-secondary" onClick={() => setEditOpen(false)} style={{ flex: 1 }}>
              Cancel
            </button>
            <button className="btn btn-primary" onClick={handleUpdateUser} style={{ flex: 1 }}>
              <Save size={14} />
              <span>Update User</span>
            </button>
          </div>
        </div>
      </SlidePanel>

      {/* Reset Password Panel */}
      <SlidePanel open={resetPwOpen} onClose={() => setResetPwOpen(false)} title={`Reset Password: ${resetUsername}`}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="form-group">
            <label className="form-label">New Password</label>
            <input
              className="form-input"
              type="password"
              placeholder="Enter new password"
              value={resetNewPw}
              onChange={(e) => setResetNewPw(e.target.value)}
            />
          </div>
          <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
            <button className="btn btn-secondary" onClick={() => setResetPwOpen(false)} style={{ flex: 1 }}>
              Cancel
            </button>
            <button className="btn btn-primary" onClick={handleResetPassword} style={{ flex: 1 }}>
              <Save size={14} />
              <span>Reset Password</span>
            </button>
          </div>
        </div>
      </SlidePanel>
    </div>
  );
}
