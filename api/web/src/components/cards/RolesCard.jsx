import React, { useState, useCallback, useEffect } from 'react';
import { useApp } from '../../context/AppContext';
import { Shield, RefreshCw, ScrollText, Loader2, XCircle, Inbox, ChevronDown, ChevronRight } from 'lucide-react';

export default function RolesCard() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();
  const [roles, setRoles] = useState([]);
  const [permissions, setPermissions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [viewMode, setViewMode] = useState('roles'); // 'roles' or 'permissions'
  const [expandedItems, setExpandedItems] = useState({});

  const getRoles = useCallback(async () => {
    if (!isAuthenticated) return;
    
    setLoading(true);
    setError('');
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/roles');
      const data = await response.json();
      
      if (data.success) {
        const rolesList = data.data && data.data.roles ? data.data.roles : [];
        setRoles(rolesList);
        if (rolesList.length > 0) {
          showToast('success', `Loaded ${rolesList.length} role(s)`);
        } else {
          showToast('info', 'No roles found');
        }
      } else {
        setError(data.error || data.message || 'Failed to load roles');
        setRoles([]);
      }
    } catch (error) {
      setError(error.message);
      setRoles([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast]);

  const getPermissions = useCallback(async () => {
    if (!isAuthenticated) return;
    
    setLoading(true);
    setError('');
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/permissions');
      const data = await response.json();
      
      if (data.success) {
        const permissionsList = data.data && data.data.permissions ? data.data.permissions : [];
        setPermissions(permissionsList);
        if (permissionsList.length > 0) {
          showToast('success', `Loaded ${permissionsList.length} permission(s)`);
        } else {
          showToast('info', 'No permissions found');
        }
      } else {
        setError(data.error || data.message || 'Failed to load permissions');
        setPermissions([]);
      }
    } catch (error) {
      setError(error.message);
      setPermissions([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast]);

  useEffect(() => {
    if (isAuthenticated) {
      getRoles();
      getPermissions();
    }
  }, [isAuthenticated]);

  const toggleExpand = (itemId) => {
    setExpandedItems(prev => ({
      ...prev,
      [itemId]: !prev[itemId]
    }));
  };

  const handleViewModeChange = (mode) => {
    setViewMode(mode);
    setExpandedItems({});
  };

  return (
    <section className="card roles-card">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><Shield size={18} /></span>
          <h2>Roles & Permissions</h2>
        </div>
        <div className="card-actions">
          <button 
            className="icon-btn auth-required" 
            onClick={getRoles}
            title="Load Roles" 
            disabled={!isAuthenticated}
          >
            <RefreshCw size={16} />
          </button>
          <button 
            className="icon-btn auth-required" 
            onClick={getPermissions}
            title="Load Permissions" 
            disabled={!isAuthenticated}
          >
            <ScrollText size={16} />
          </button>
        </div>
      </div>
      <div className="card-body">
        {/* View Mode Tabs */}
        <div className="view-mode-tabs">
          <button 
            className={`view-mode-tab ${viewMode === 'roles' ? 'active' : ''}`}
            onClick={() => handleViewModeChange('roles')}
          >
            <Shield size={14} /> Roles ({roles.length})
          </button>
          <button 
            className={`view-mode-tab ${viewMode === 'permissions' ? 'active' : ''}`}
            onClick={() => handleViewModeChange('permissions')}
          >
            <ScrollText size={14} /> Permissions ({permissions.length})
          </button>
        </div>

        {loading && (
          <div className="empty-state">
            <span className="empty-icon"><Loader2 size={24} className="spin" /></span>
            <p>Loading {viewMode}...</p>
          </div>
        )}
        
        {!loading && error && (
          <div className="empty-state error">
            <span className="empty-icon"><XCircle size={24} /></span>
            <p>Error: {error}</p>
          </div>
        )}
        
        {!loading && !error && viewMode === 'roles' && roles.length === 0 && (
          <div className="empty-state">
            <span className="empty-icon"><Inbox size={24} /></span>
            <p>No roles found. {isAuthenticated ? 'Click the refresh button to load roles.' : 'Authenticate to view roles.'}</p>
          </div>
        )}
        
        {!loading && !error && viewMode === 'permissions' && permissions.length === 0 && (
          <div className="empty-state">
            <span className="empty-icon"><Inbox size={24} /></span>
            <p>No permissions found. {isAuthenticated ? 'Click the permissions button to load.' : 'Authenticate to view permissions.'}</p>
          </div>
        )}
        
        {!loading && !error && viewMode === 'roles' && roles.length > 0 && (
          <div className="roles-list">
            {roles.map((role) => (
              <div key={role.id} className="role-item">
                <div className="role-header" onClick={() => toggleExpand(role.id)}>
                  <span className="expand-icon">
                    {expandedItems[role.id] ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                  </span>
                  <div className="role-info">
                    <span className="role-name">{role.name}</span>
                    {role.rolePolicy && (
                      <span className="permission-count">{role.rolePolicy.length} policy(s)</span>
                    )}
                  </div>
                </div>
                
                {expandedItems[role.id] && (
                  <div className="role-details">
                    <div className="detail-grid">
                      <div className="detail-item">
                        <span className="detail-label">Role ID</span>
                        <span className="detail-value">{role.id}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">Name</span>
                        <span className="detail-value">{role.name}</span>
                      </div>
                      {role.description && (
                        <div className="detail-item">
                          <span className="detail-label">Description</span>
                          <span className="detail-value">{role.description}</span>
                        </div>
                      )}
                      {role.rolePolicy && role.rolePolicy.length > 0 && (
                        <div className="detail-item full-width">
                          <span className="detail-label">Role Policies</span>
                          <div className="permissions-list">
                            {role.rolePolicy.map((policy, index) => (
                              <span key={index} className="permission-badge">{policy.permissionKey}</span>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
        
        {!loading && !error && viewMode === 'permissions' && permissions.length > 0 && (
          <div className="permissions-grid">
            {permissions.map((permission) => (
              <div key={permission.key} className="permission-item">
                <div className="permission-header">
                  <span className="permission-name">{permission.key}</span>
                </div>
                {permission.description && (
                  <div className="permission-description">{permission.description}</div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
