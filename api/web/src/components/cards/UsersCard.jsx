import React, { useState, useCallback, useEffect } from 'react';
import { useApp } from '../../context/AppContext';
import { Users, RefreshCw, Loader2, XCircle, Inbox, ChevronDown, ChevronRight } from 'lucide-react';

export default function UsersCard() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [expandedUsers, setExpandedUsers] = useState({});

  const getUsers = useCallback(async () => {
    if (!isAuthenticated) return;
    
    setLoading(true);
    setError('');
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/users');
      const data = await response.json();
      
      if (data.success) {
        const usersList = data.data && data.data.users ? data.data.users : [];
        setUsers(usersList);
        if (usersList.length > 0) {
          showToast('success', `Loaded ${usersList.length} user(s)`);
        } else {
          showToast('info', 'No users found');
        }
      } else {
        setError(data.error || data.message || 'Failed to load users');
        setUsers([]);
      }
    } catch (error) {
      setError(error.message);
      setUsers([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast]);

  useEffect(() => {
    if (isAuthenticated) {
      getUsers();
    }
  }, [isAuthenticated]);

  const toggleExpand = (userId) => {
    setExpandedUsers(prev => ({
      ...prev,
      [userId]: !prev[userId]
    }));
  };

  return (
    <section className="card users-card">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><Users size={18} /></span>
          <h2>Users</h2>
        </div>
        <div className="card-actions">
          <button 
            className="icon-btn auth-required" 
            onClick={getUsers}
            title="Load Users" 
            disabled={!isAuthenticated}
          >
            <RefreshCw size={16} />
          </button>
        </div>
      </div>
      <div className="card-body">
        {loading && (
          <div className="empty-state">
            <span className="empty-icon"><Loader2 size={24} className="spin" /></span>
            <p>Loading users...</p>
          </div>
        )}
        
        {!loading && error && (
          <div className="empty-state error">
            <span className="empty-icon"><XCircle size={24} /></span>
            <p>Error: {error}</p>
          </div>
        )}
        
        {!loading && !error && users.length === 0 && (
          <div className="empty-state">
            <span className="empty-icon"><Inbox size={24} /></span>
            <p>No users found. {isAuthenticated ? 'Click the refresh button to load users.' : 'Authenticate to view users.'}</p>
          </div>
        )}
        
        {!loading && !error && users.length > 0 && (
          <div className="users-list">
            {users.map((user) => (
              <div key={user.id} className="user-item">
                <div className="user-header" onClick={() => toggleExpand(user.id)}>
                  <span className="expand-icon">
                    {expandedUsers[user.id] ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                  </span>
                  <div className="user-info">
                    <span className="user-name">{user.username}</span>
                    {user.roles && user.roles.length > 0 && (
                      <span className="user-role-badge">{user.roles[0]}</span>
                    )}
                  </div>
                </div>
                
                {expandedUsers[user.id] && (
                  <div className="user-details">
                    <div className="detail-grid">
                      <div className="detail-item">
                        <span className="detail-label">User ID</span>
                        <span className="detail-value">{user.id}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">Username</span>
                        <span className="detail-value">{user.username}</span>
                      </div>
                      {user.status !== undefined && (
                        <div className="detail-item">
                          <span className="detail-label">Status</span>
                          <span className="detail-value">{user.status}</span>
                        </div>
                      )}
                      {user.roles && user.roles.length > 0 && (
                        <div className="detail-item full-width">
                          <span className="detail-label">Roles</span>
                          <div className="permissions-list">
                            {user.roles.map((role, index) => (
                              <span key={index} className="permission-badge">{role}</span>
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
      </div>
    </section>
  );
}
