import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useApp } from '../../context/AppContext';
import { Activity } from 'lucide-react';

export default function HealthCard() {
  const { getApiUrl, healthHistory, setHealthHistory, showToast } = useApp();
  const [healthStatus, setHealthStatus] = useState('--');
  const [healthClass, setHealthClass] = useState('');
  const [healthDetails, setHealthDetails] = useState('Awaiting health check...');
  const [detailsClass, setDetailsClass] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(true);
  const intervalRef = useRef(null);

  const checkHealth = useCallback(async () => {
    setHealthStatus('...');
    setHealthClass('');
    
    try {
      const response = await fetch(getApiUrl('/health'));
      const data = await response.json();
      const isHealthy = response.ok && data.status === 'healthy';
      
      const newEntry = {
        timestamp: new Date().toISOString(),
        healthy: isHealthy,
        data: data
      };
      
      setHealthHistory(prev => {
        const updated = [...prev, newEntry];
        if (updated.length > 10) updated.shift();
        return updated;
      });
      
      setHealthStatus(isHealthy ? 'OK' : 'FAIL');
      setHealthClass(isHealthy ? 'healthy' : 'unhealthy');
      setHealthDetails(JSON.stringify(data, null, 2));
      setDetailsClass(isHealthy ? 'success' : 'error');
      
    } catch (error) {
      console.error('Health check error:', error);
      
      const newEntry = {
        timestamp: new Date().toISOString(),
        healthy: false,
        error: error.message
      };
      
      setHealthHistory(prev => {
        const updated = [...prev, newEntry];
        if (updated.length > 10) updated.shift();
        return updated;
      });
      
      setHealthStatus('ERR');
      setHealthClass('unhealthy');
      setHealthDetails('Error: ' + error.message + '\n\nTip: Configure the API Base URL if running from a different origin.');
      setDetailsClass('error');
    }
  }, [getApiUrl, setHealthHistory]);

  useEffect(() => {
    // Listen for health check events
    const handleCheckHealth = () => checkHealth();
    window.addEventListener('checkHealth', handleCheckHealth);
    
    // Initial health check
    const timer = setTimeout(checkHealth, 500);
    
    return () => {
      window.removeEventListener('checkHealth', handleCheckHealth);
      clearTimeout(timer);
    };
  }, [checkHealth]);

  useEffect(() => {
    if (autoRefresh) {
      intervalRef.current = setInterval(checkHealth, 5000);
      checkHealth();
      showToast('info', 'Auto-refresh enabled (5s interval)');
    } else {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    }
    
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [autoRefresh, checkHealth, showToast]);

  const handleToggleAutoRefresh = () => {
    setAutoRefresh(prev => !prev);
    if (autoRefresh) {
      showToast('info', 'Auto-refresh disabled');
    }
  };

  return (
    <section className="card health-card">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><Activity size={18} /></span>
          <h2>System Health</h2>
        </div>
        <div className="card-actions">
          <div className="auto-refresh-toggle">
            <input 
              type="checkbox" 
              id="healthAutoRefresh"
              checked={autoRefresh}
              onChange={handleToggleAutoRefresh}
            />
            <label htmlFor="healthAutoRefresh">Auto</label>
          </div>
        </div>
      </div>
      <div className="card-body">
        <div className="health-display">
          <div className="health-indicator" id="healthIndicator">
            <div className="health-ring">
              <svg viewBox="0 0 100 100">
                <circle cx="50" cy="50" r="45" className="ring-bg"/>
                <circle 
                  cx="50" 
                  cy="50" 
                  r="45" 
                  className={`ring-progress ${healthClass}`}
                />
              </svg>
              <div className={`health-status ${healthClass}`}>{healthStatus}</div>
            </div>
          </div>
          <div className="health-history">
            <span className="history-label">History (Last 10)</span>
            <div className="history-grid" id="healthGrid">
              {/* Empty slots */}
              {Array(Math.max(0, 10 - healthHistory.length)).fill(null).map((_, i) => (
                <div key={`empty-${i}`} className="history-dot" title="No data">-</div>
              ))}
              {/* History dots */}
              {healthHistory.map((result, i) => (
                <div 
                  key={`health-${i}`}
                  className={`history-dot ${result.healthy ? 'healthy' : 'unhealthy'}`}
                  title={`${new Date(result.timestamp).toLocaleTimeString()}: ${result.healthy ? 'Healthy' : 'Unhealthy'}`}
                >
                  {result.healthy ? '✓' : '✗'}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
