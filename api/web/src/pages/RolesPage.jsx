import React, { useState, useEffect, useCallback } from 'react';
import { useApp } from '../context/AppContext';
import SlidePanel from '../components/SlidePanel';
import {
  Shield,
  ScrollText,
  Plus,
  RefreshCw,
  Pencil,
  Trash2,
  Save,
  Loader2,
  Inbox,
  ChevronDown,
  ChevronRight,
  X,
} from 'lucide-react';

function blankRoleForm() {
  return {
    id: null,
    name: '',
    description: '',
    policies: [{ permissionKey: '', self: false, k8sNamespace: '', policyNamespace: '' }],
  };
}

export default function RolesPage() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();
  const [roles, setRoles] = useState([]);
  const [permissions, setPermissions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [activeTab, setActiveTab] = useState('roles');
  const [expandedRoles, setExpandedRoles] = useState(new Set());

  /* panel */
  const [panelOpen, setPanelOpen] = useState(false);
  const [panelMode, setPanelMode] = useState('create');
  const [form, setForm] = useState(blankRoleForm());

  const fetchRoles = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoading(true);
    setError('');
    try {
      const res = await makeAuthenticatedRequest('/api/v1/roles');
      const data = await res.json();
      if (data.success) {
        setRoles(data.data?.roles || []);
      } else {
        setError(data.error || 'Failed');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest]);

  const fetchPermissions = useCallback(async () => {
    if (!isAuthenticated) return;
    try {
      const res = await makeAuthenticatedRequest('/api/v1/permissions');
      const data = await res.json();
      if (data.success) {
        setPermissions(data.data?.permissions || []);
      }
    } catch {
      /* silent */
    }
  }, [isAuthenticated, makeAuthenticatedRequest]);

  useEffect(() => {
    if (isAuthenticated) {
      fetchRoles();
      fetchPermissions();
    }
  }, [isAuthenticated, fetchRoles, fetchPermissions]);

  const toggleRole = (id) => {
    setExpandedRoles((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  };

  /* ─── form helpers ─── */
  const updatePolicy = (i, field, value) =>
    setForm((f) => {
      const p = [...f.policies];
      p[i] = { ...p[i], [field]: value };
      return { ...f, policies: p };
    });
  const addPolicy = () =>
    setForm((f) => ({
      ...f,
      policies: [...f.policies, { permissionKey: '', self: false, k8sNamespace: '', policyNamespace: '' }],
    }));
  const removePolicy = (i) =>
    setForm((f) => {
      const p = f.policies.filter((_, idx) => idx !== i);
      return { ...f, policies: p.length ? p : [{ permissionKey: '', self: false, k8sNamespace: '', policyNamespace: '' }] };
    });

  /* ─── open panel ─── */
  const openCreate = () => {
    setForm(blankRoleForm());
    setPanelMode('create');
    setPanelOpen(true);
  };

  const openEdit = (role) => {
    const policies = (role.rolePolicy || []).map((p) => ({
      permissionKey: p.permissionKey || '',
      self: p.self || false,
      k8sNamespace: p.k8sNamespace || '',
      policyNamespace: p.policyNamespace || '',
    }));
    setForm({
      id: role.id,
      name: role.name || '',
      description: role.description || '',
      policies: policies.length
        ? policies
        : [{ permissionKey: '', self: false, k8sNamespace: '', policyNamespace: '' }],
    });
    setPanelMode('edit');
    setPanelOpen(true);
  };

  /* ─── save ─── */
  const handleSave = async () => {
    if (!form.name.trim()) {
      showToast('error', 'Role name is required');
      return;
    }
    const rolePolicies = form.policies
      .filter((p) => p.permissionKey.trim())
      .map((p) => ({
        permissionKey: p.permissionKey.trim(),
        self: p.self,
        k8sNamespace: p.k8sNamespace.trim() || undefined,
        policyNamespace: p.policyNamespace.trim() || undefined,
      }));

    const payload = {
      name: form.name.trim(),
      description: form.description.trim() || undefined,
      rolePolicies,
    };
    if (panelMode === 'edit') {
      payload.id = form.id;
      // backend uses rolePolicy for update
      payload.rolePolicy = rolePolicies;
      delete payload.rolePolicies;
    }

    try {
      const res = await makeAuthenticatedRequest('/api/v1/roles', {
        method: panelMode === 'create' ? 'POST' : 'PUT',
        body: JSON.stringify(payload),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', panelMode === 'create' ? 'Role created' : 'Role updated');
        setPanelOpen(false);
        fetchRoles();
      } else {
        showToast('error', data.error || data.message || 'Failed');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  /* ─── delete ─── */
  const handleDelete = async (roleId) => {
    if (!window.confirm('Delete this role? Users with this role will lose its permissions.')) return;
    try {
      const res = await makeAuthenticatedRequest('/api/v1/roles', {
        method: 'DELETE',
        body: JSON.stringify({ id: roleId }),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', 'Role deleted');
        fetchRoles();
      } else {
        showToast('error', data.error || 'Failed');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Roles & Permissions</h1>
          <p className="page-subtitle">Manage role definitions and view available permission keys</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-secondary btn-sm" onClick={() => { fetchRoles(); fetchPermissions(); }}>
            <RefreshCw size={14} />
            <span>Refresh</span>
          </button>
          <button className="btn btn-primary btn-sm" onClick={openCreate}>
            <Plus size={14} />
            <span>New Role</span>
          </button>
        </div>
      </div>

      <div className="stat-cards">
        <div className="stat-card">
          <div className="stat-card-label">Roles</div>
          <div className="stat-card-value">{roles.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Permissions</div>
          <div className="stat-card-value">{permissions.length}</div>
        </div>
      </div>

      {/* Tabs */}
      <div className="tabs" style={{ marginBottom: 0 }}>
        <button
          className={`tab${activeTab === 'roles' ? ' active' : ''}`}
          onClick={() => setActiveTab('roles')}
        >
          <Shield size={14} />
          Roles ({roles.length})
        </button>
        <button
          className={`tab${activeTab === 'permissions' ? ' active' : ''}`}
          onClick={() => setActiveTab('permissions')}
        >
          <ScrollText size={14} />
          Permissions ({permissions.length})
        </button>
      </div>

      {/* Roles tab */}
      {activeTab === 'roles' && (
        <div className="card" style={{ borderTopLeftRadius: 0 }}>
          <div className="card-body" style={{ padding: 0 }}>
            {loading ? (
              <div className="empty-state">
                <Loader2 size={20} className="spin" />
                <p>Loading roles...</p>
              </div>
            ) : error ? (
              <div className="empty-state">
                <p>{error}</p>
              </div>
            ) : roles.length === 0 ? (
              <div className="empty-state">
                <Inbox size={20} />
                <p>No roles found</p>
              </div>
            ) : (
              <div>
                {roles.map((role) => {
                  const isExpanded = expandedRoles.has(role.id);
                  return (
                    <div key={role.id} style={{ borderBottom: '1px solid var(--color-border)' }}>
                      <div
                        onClick={() => toggleRole(role.id)}
                        style={{
                          padding: '12px 16px',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          cursor: 'pointer',
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          {isExpanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                          <span style={{ fontWeight: 500, fontSize: 14 }}>{role.name}</span>
                          {role.rolePolicy && (
                            <span className="badge badge-secondary">
                              {role.rolePolicy.length} policies
                            </span>
                          )}
                        </div>
                        <div style={{ display: 'flex', gap: 4 }} onClick={(e) => e.stopPropagation()}>
                          <button className="btn btn-ghost btn-sm" title="Edit role" onClick={() => openEdit(role)}>
                            <Pencil size={14} />
                          </button>
                          <button className="btn btn-ghost btn-sm" title="Delete role" onClick={() => handleDelete(role.id)}>
                            <Trash2 size={14} />
                          </button>
                        </div>
                      </div>
                      {isExpanded && (
                        <div style={{ padding: '0 16px 16px 40px' }}>
                          <div className="detail-grid">
                            <div className="detail-item">
                              <span className="detail-label">Role ID</span>
                              <span className="detail-value" style={{ fontFamily: 'monospace', fontSize: 12 }}>
                                {role.id}
                              </span>
                            </div>
                            {role.description && (
                              <div className="detail-item">
                                <span className="detail-label">Description</span>
                                <span className="detail-value">{role.description}</span>
                              </div>
                            )}
                          </div>
                          {role.rolePolicy && role.rolePolicy.length > 0 && (
                            <div style={{ marginTop: 12 }}>
                              <div style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--color-text-secondary)', marginBottom: 6 }}>
                                Policies
                              </div>
                              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                                {role.rolePolicy.map((policy, i) => (
                                  <span key={i} className="badge badge-primary">
                                    {policy.permissionKey}
                                    {policy.self && ' (self)'}
                                  </span>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Permissions tab */}
      {activeTab === 'permissions' && (
        <div className="card" style={{ borderTopLeftRadius: 0 }}>
          <div className="card-body" style={{ padding: 0 }}>
            {permissions.length === 0 ? (
              <div className="empty-state">
                <Inbox size={20} />
                <p>No permissions found</p>
              </div>
            ) : (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>PERMISSION KEY</th>
                    <th>DESCRIPTION</th>
                  </tr>
                </thead>
                <tbody>
                  {permissions.map((perm) => (
                    <tr key={perm.key}>
                      <td>
                        <span className="badge badge-secondary" style={{ fontFamily: 'monospace' }}>
                          {perm.key}
                        </span>
                      </td>
                      <td>{perm.description || '--'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}

      {/* Create/Edit Role Panel */}
      <SlidePanel
        open={panelOpen}
        onClose={() => setPanelOpen(false)}
        title={panelMode === 'create' ? 'New Role' : 'Edit Role'}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="form-group">
            <label className="form-label">Role Name</label>
            <input
              className="form-input"
              placeholder="e.g., admin, viewer, operator"
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Description</label>
            <input
              className="form-input"
              placeholder="Role description"
              value={form.description}
              onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
            />
          </div>

          {/* Policies */}
          <div>
            <label className="form-label">Policies</label>
            {form.policies.map((policy, i) => (
              <div key={i} style={{ marginBottom: 10, padding: 12, background: 'var(--color-page-bg)', borderRadius: 8 }}>
                <div style={{ display: 'flex', gap: 8, marginBottom: 6 }}>
                  <div className="form-group" style={{ flex: 1, marginBottom: 0 }}>
                    <select
                      className="form-input"
                      value={policy.permissionKey}
                      onChange={(e) => updatePolicy(i, 'permissionKey', e.target.value)}
                    >
                      <option value="">Select permission...</option>
                      {permissions.map((p) => (
                        <option key={p.key} value={p.key}>
                          {p.key}
                        </option>
                      ))}
                    </select>
                  </div>
                  <button className="btn btn-danger btn-sm" onClick={() => removePolicy(i)}>
                    <X size={14} />
                  </button>
                </div>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
                    <input
                      type="checkbox"
                      checked={policy.self}
                      onChange={() => updatePolicy(i, 'self', !policy.self)}
                    />
                    Self only
                  </label>
                  <input
                    className="form-input"
                    placeholder="K8s namespace"
                    value={policy.k8sNamespace}
                    onChange={(e) => updatePolicy(i, 'k8sNamespace', e.target.value)}
                    style={{ flex: 1, fontSize: 12 }}
                  />
                  <input
                    className="form-input"
                    placeholder="Policy namespace"
                    value={policy.policyNamespace}
                    onChange={(e) => updatePolicy(i, 'policyNamespace', e.target.value)}
                    style={{ flex: 1, fontSize: 12 }}
                  />
                </div>
              </div>
            ))}
            <button className="btn btn-ghost btn-sm" onClick={addPolicy}>
              + Add Policy
            </button>
          </div>

          <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
            <button className="btn btn-secondary" onClick={() => setPanelOpen(false)} style={{ flex: 1 }}>
              Cancel
            </button>
            <button className="btn btn-primary" onClick={handleSave} style={{ flex: 1 }}>
              <Save size={14} />
              <span>{panelMode === 'create' ? 'Create Role' : 'Update Role'}</span>
            </button>
          </div>
        </div>
      </SlidePanel>
    </div>
  );
}
