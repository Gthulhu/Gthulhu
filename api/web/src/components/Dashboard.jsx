import React from 'react';
import { useApp } from '../context/AppContext';
import { Settings, Activity, Package, RefreshCw } from 'lucide-react';
import HealthCard from './cards/HealthCard';
import UserProfileCard from './cards/UserProfileCard';
import IntentsCard from './cards/IntentsCard';
import StrategiesCard from './cards/StrategiesCard';
import UsersCard from './cards/UsersCard';
import RolesCard from './cards/RolesCard';
import PodsCard from './cards/PodsCard';

export default function Dashboard() {
  const { isAuthenticated, showToast, getApiUrl } = useApp();

  const handleConfigClick = () => {
    window.dispatchEvent(new CustomEvent('openConfigModal'));
  };

  const handleHealthCheck = () => {
    window.dispatchEvent(new CustomEvent('checkHealth'));
  };

  const handleGetVersion = async () => {
    try {
      const response = await fetch(getApiUrl('/version'));
      const data = await response.json();
      showToast('info', 'Version: ' + (data.version || JSON.stringify(data)));
    } catch (error) {
      showToast('error', 'Failed to get version: ' + error.message);
    }
  };

  const handleRefreshAll = () => {
    showToast('info', 'Refreshing all data...');
    window.dispatchEvent(new CustomEvent('checkHealth'));
    if (isAuthenticated) {
      window.dispatchEvent(new CustomEvent('refreshUserProfile'));
      window.dispatchEvent(new CustomEvent('refreshStrategies'));
      window.dispatchEvent(new CustomEvent('refreshIntents'));
      window.dispatchEvent(new CustomEvent('refreshPodPids'));
    }
    setTimeout(() => showToast('success', 'Data refreshed'), 500);
  };

  return (
    <main className="dashboard">
      {/* Quick Actions Bar */}
      <div className="quick-actions">
        <button className="action-chip" onClick={handleConfigClick}>
          <Settings size={14} /> API Config
        </button>
        <button className="action-chip" onClick={handleHealthCheck}>
          <Activity size={14} /> Health Check
        </button>
        <button className="action-chip" onClick={handleGetVersion}>
          <Package size={14} /> Version
        </button>
        <button 
          className="action-chip auth-required" 
          onClick={handleRefreshAll}
          disabled={!isAuthenticated}
        >
          <RefreshCw size={14} /> Refresh All
        </button>
      </div>

      {/* Dashboard Grid */}
      <div className="dashboard-grid">
        <HealthCard />
        <UserProfileCard />
        <IntentsCard />
        <StrategiesCard />
        <UsersCard />
        <RolesCard />
        <PodsCard />
      </div>
    </main>
  );
}
