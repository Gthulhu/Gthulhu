import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import {
  Server,
  RefreshCw,
  CheckCircle,
  XCircle,
  Loader2,
} from 'lucide-react';

export default function NodesPage() {
  const { isAuthenticated, makeAuthenticatedRequest, getApiUrl, showToast, healthHistory, setHealthHistory } = useApp();
  const navigate = useNavigate();

  // Nodes
  const [nodes, setNodes] = useState([]);
  const [loadingNodes, setLoadingNodes] = useState(false);

  // Health
  const [healthStatus, setHealthStatus] = useState(null); // null | 'healthy' | 'unhealthy'
  const [healthData, setHealthData] = useState(null);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const intervalRef = useRef(null);

  const fetchNodes = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoadingNodes(true);
    try {
      const response = await makeAuthenticatedRequest('/api/v1/nodes');
      const data = await response.json();
      if (data.success && data.data?.nodes) {
        setNodes(data.data.nodes);
      } else {
        setNodes([]);
      }
    } catch (err) {
      showToast('error', 'Failed to fetch nodes: ' + err.message);
      setNodes([]);
    } finally {
      setLoadingNodes(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast]);

  const checkHealth = useCallback(async () => {
    try {
      const response = await fetch(getApiUrl('/health'));
      const data = await response.json();
      const isHealthy = response.ok && data.status === 'healthy';

      setHealthStatus(isHealthy ? 'healthy' : 'unhealthy');
      setHealthData(data);

      setHealthHistory((prev) => {
        const updated = [...prev, { timestamp: new Date().toISOString(), healthy: isHealthy, data }];
        if (updated.length > 10) updated.shift();
        return updated;
      });
    } catch (err) {
      setHealthStatus('unhealthy');
      setHealthData({ error: err.message });

      setHealthHistory((prev) => {
        const updated = [...prev, { timestamp: new Date().toISOString(), healthy: false, error: err.message }];
        if (updated.length > 10) updated.shift();
        return updated;
      });
    }
  }, [getApiUrl, setHealthHistory]);

  useEffect(() => {
    if (isAuthenticated) fetchNodes();
  }, [isAuthenticated, fetchNodes]);

  useEffect(() => {
    checkHealth();
  }, [checkHealth]);

  useEffect(() => {
    if (autoRefresh) {
      intervalRef.current = setInterval(checkHealth, 5000);
    } else {
      if (intervalRef.current) clearInterval(intervalRef.current);
    }
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [autoRefresh, checkHealth]);

  const healthyCount = healthHistory.filter((h) => h.healthy).length;
  const totalChecks = healthHistory.length;

  return (
    <div>
      {/* Page header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">Nodes & Health</h1>
          <p className="page-subtitle">Cluster node overview and system health monitoring</p>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <label className="toggle-switch-label" style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={() => setAutoRefresh((v) => !v)}
            />
            <span style={{ marginLeft: 6 }}>Auto-refresh</span>
          </label>
          <button className="btn btn-secondary btn-sm" onClick={() => { fetchNodes(); checkHealth(); }}>
            <RefreshCw size={14} />
            <span>Refresh</span>
          </button>
        </div>
      </div>

      {/* Stat cards */}
      <div className="stat-cards">
        <div className="stat-card">
          <div className="stat-card-label">Total Nodes</div>
          <div className="stat-card-value">{loadingNodes ? '--' : nodes.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Ready Nodes</div>
          <div className="stat-card-value">
            {loadingNodes ? '--' : nodes.filter((n) => n.status === 'Ready').length}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">API Health</div>
          <div className="stat-card-value" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            {healthStatus === null ? (
              '--'
            ) : healthStatus === 'healthy' ? (
              <>
                <span className="health-dot healthy" />
                OK
              </>
            ) : (
              <>
                <span className="health-dot unhealthy" />
                FAIL
              </>
            )}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Uptime</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 4 }}>
            <div style={{ display: 'flex', gap: 3, alignItems: 'center' }}>
              {Array.from({ length: 10 }).map((_, i) => {
                const entry = healthHistory[i];
                const color = !entry
                  ? 'var(--color-border)'
                  : entry.healthy
                    ? 'var(--color-success, #22c55e)'
                    : 'var(--color-danger, #ef4444)';
                return (
                  <div
                    key={i}
                    title={entry ? `${new Date(entry.timestamp).toLocaleTimeString()} — ${entry.healthy ? 'Healthy' : 'Failed'}` : 'No data'}
                    style={{
                      width: 6,
                      height: 20,
                      borderRadius: 2,
                      background: color,
                      transition: 'background 0.3s',
                    }}
                  />
                );
              })}
            </div>
            <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-primary)' }}>
              {totalChecks === 0 ? '--' : `${Math.round((healthyCount / totalChecks) * 100)}%`}
            </span>
          </div>
        </div>
      </div>

      {/* Nodes table */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <Server size={16} />
            Cluster Nodes
          </h3>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {loadingNodes ? (
            <div className="empty-state">
              <Loader2 size={20} className="spin" />
              <p>Loading nodes...</p>
            </div>
          ) : nodes.length === 0 ? (
            <div className="empty-state">
              <Server size={20} />
              <p>No nodes found</p>
            </div>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>NODE NAME</th>
                  <th>STATUS</th>
                  <th>ACTIONS</th>
                </tr>
              </thead>
              <tbody>
                {nodes.map((node) => (
                  <tr key={node.name}>
                    <td style={{ fontWeight: 500 }}>{node.name}</td>
                    <td>
                      <span className={`badge ${node.status === 'Ready' ? 'badge-success' : 'badge-danger'}`}>
                        {node.status === 'Ready' ? <CheckCircle size={12} /> : <XCircle size={12} />}
                        {node.status}
                      </span>
                    </td>
                    <td>
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => navigate(`/nodes/${encodeURIComponent(node.name)}`)}
                      >
                        View Pods
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>


    </div>
  );
}
