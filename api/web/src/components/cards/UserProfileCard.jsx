import React, { useState, useEffect, useCallback } from 'react';
import { useApp } from '../../context/AppContext';
import { User, RefreshCw, Shield, BarChart3 } from 'lucide-react';

export default function UserProfileCard() {
  const { isAuthenticated, makeAuthenticatedRequest, currentUser, setCurrentUser, showToast } = useApp();
  const [loading, setLoading] = useState(false);
  const [rawDetails, setRawDetails] = useState('Authenticate to view profile...');
  const [detailsClass, setDetailsClass] = useState('');

  const getUserProfile = useCallback(async () => {
    if (!isAuthenticated) return;
    
    setLoading(true);
    try {
      const response = await makeAuthenticatedRequest('/api/v1/users/self');
      const data = await response.json();
      
      if (data.success && data.data) {
        setCurrentUser(data.data);
        setRawDetails(JSON.stringify(data, null, 2));
        setDetailsClass('success');
      } else {
        throw new Error(data.error || data.message || 'Failed to get user profile');
      }
    } catch (error) {
      setRawDetails('Error: ' + error.message);
      setDetailsClass('error');
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, setCurrentUser]);

  useEffect(() => {
    if (isAuthenticated) {
      getUserProfile();
    }
  }, [isAuthenticated, getUserProfile]);

  useEffect(() => {
    const handleRefresh = () => getUserProfile();
    window.addEventListener('refreshUserProfile', handleRefresh);
    return () => window.removeEventListener('refreshUserProfile', handleRefresh);
  }, [getUserProfile]);

  const displayName = currentUser?.username || currentUser?.email || 'Not Connected';
  const avatarInitial = displayName !== 'Not Connected' ? displayName.charAt(0).toUpperCase() : '?';
  const email = currentUser?.email || 'Authenticate to view profile';
  const role = (currentUser?.roles && currentUser.roles[0]) || '--';
  const status = currentUser ? 'Active' : 'Offline';

  return (
    <section className="card user-card">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><User size={18} /></span>
          <h2>User Profile</h2>
        </div>
        <div className="card-actions">
          <button 
            className="icon-btn auth-required" 
            onClick={getUserProfile}
            title="Refresh Profile" 
            disabled={!isAuthenticated}
          >
            <RefreshCw size={16} />
          </button>
        </div>
      </div>
      <div className="card-body">
        <div className="user-profile-display" id="userProfileDisplay">
          <div className="user-avatar">
            <div className="avatar-circle" id="userAvatar">{avatarInitial}</div>
            <div className={`user-status-indicator ${currentUser ? 'online' : ''}`} id="userStatusIndicator"></div>
          </div>
          <div className="user-main-info">
            <div className="user-name" id="userDisplayName">{displayName}</div>
            <div className="user-email-display" id="userEmailDisplay">{email}</div>
          </div>
          <div className="user-meta-grid">
            <div className="user-meta-item">
              <span className="meta-icon"><Shield size={16} /></span>
              <div className="meta-content">
                <span className="meta-label">Role</span>
                <span className="meta-value" id="userRoleDisplay">{role}</span>
              </div>
            </div>
            <div className="user-meta-item">
              <span className="meta-icon"><BarChart3 size={16} /></span>
              <div className="meta-content">
                <span className="meta-label">Status</span>
                <span className="meta-value" id="userStatusDisplay">{status}</span>
              </div>
            </div>
          </div>
        </div>
        <details className="user-raw-details">
          <summary>Raw Profile Data</summary>
          <pre className={`code-block ${detailsClass}`} id="userDetails">{rawDetails}</pre>
        </details>
      </div>
    </section>
  );
}
