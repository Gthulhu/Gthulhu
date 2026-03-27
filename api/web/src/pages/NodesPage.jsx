import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import {
  Server,
  RefreshCw,
  CheckCircle,
  XCircle,
  Loader2,
  Cpu,
  Save,
  AlertCircle,
  ChevronDown,
  ChevronRight,
  Eye,
} from 'lucide-react';

export default function NodesPage() {
  const { isAuthenticated, makeAuthenticatedRequest, getApiUrl, showToast, healthHistory, setHealthHistory, applyRuntimeConfig, getRuntimeConfigStatus } = useApp();
  const navigate = useNavigate();

  // Nodes
  const [nodes, setNodes] = useState([]);
  const [loadingNodes, setLoadingNodes] = useState(false);

  // Health
  const [healthStatus, setHealthStatus] = useState(null); // null | 'healthy' | 'unhealthy'
  const [healthData, setHealthData] = useState(null);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const intervalRef = useRef(null);

  // Runtime config state
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
  const [expandedConfigNodes, setExpandedConfigNodes] = useState(new Set());

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

  useEffect(() => {
    if (isAuthenticated) fetchNodes();
    fetchNodeStatuses();
  }, [isAuthenticated, fetchNodes, fetchNodeStatuses]);

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
                        <Eye size={14} />
                        View Details
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
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

          {/* Per-node status list (expandable) */}
          <h4 style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-secondary)', marginBottom: 12 }}>Node Config Status</h4>
          {nodeStatuses.length > 0 ? (
            <div style={{ border: '1px solid var(--color-border)', borderRadius: 8, overflow: 'hidden' }}>
              {nodeStatuses.map((n, i) => {
                const isExpanded = expandedConfigNodes.has(n.nodeId);
                const toggleExpand = () => {
                  setExpandedConfigNodes(prev => {
                    const next = new Set(prev);
                    next.has(n.nodeId) ? next.delete(n.nodeId) : next.add(n.nodeId);
                    return next;
                  });
                };
                return (
                  <div key={i} style={{ borderBottom: i < nodeStatuses.length - 1 ? '1px solid var(--color-border)' : 'none' }}>
                    {/* Summary row */}
                    <div
                      onClick={toggleExpand}
                      style={{
                        padding: '10px 14px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        cursor: 'pointer',
                        transition: 'background 0.15s',
                        fontSize: 13,
                      }}
                      onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--color-page-bg)')}
                      onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                        {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                        <span style={{ fontFamily: 'monospace', fontSize: 12, fontWeight: 500 }}>{n.nodeId}</span>
                        {n.host && <span style={{ color: 'var(--color-text-secondary)', fontSize: 12 }}>({n.host})</span>}
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                        {n.configVersion && (
                          <span style={{ fontSize: 11, color: 'var(--color-text-secondary)', fontFamily: 'monospace' }}>v{n.configVersion}</span>
                        )}
                        {n.success ? (
                          <span className="badge badge-success" style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11 }}>
                            <CheckCircle size={12} /> OK
                          </span>
                        ) : n.lastError ? (
                          <span className="badge badge-danger" style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11 }}>
                            <XCircle size={12} /> Error
                          </span>
                        ) : (
                          <span className="badge badge-secondary" style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11 }}>
                            <AlertCircle size={12} /> Unknown
                          </span>
                        )}
                      </div>
                    </div>
                    {/* Expanded config detail */}
                    {isExpanded && (
                      <div style={{ padding: '0 14px 14px 36px', fontSize: 13 }}>
                        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 10, marginBottom: 10 }}>
                          {n.config ? (
                            <>
                              <div className="detail-item">
                                <span className="detail-label">Mode</span>
                                <span className="detail-value">{n.config.mode || '—'}</span>
                              </div>
                              <div className="detail-item">
                                <span className="detail-label">Scheduler Enabled</span>
                                <span className="detail-value">{n.config.schedulerEnabled ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="detail-item">
                                <span className="detail-label">Monitoring Enabled</span>
                                <span className="detail-value">{n.config.monitoringEnabled ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="detail-item">
                                <span className="detail-label">Default Slice (ns)</span>
                                <span className="detail-value" style={{ fontFamily: 'monospace' }}>{n.config.sliceNsDefault?.toLocaleString() || '—'}</span>
                              </div>
                              <div className="detail-item">
                                <span className="detail-label">Min Slice (ns)</span>
                                <span className="detail-value" style={{ fontFamily: 'monospace' }}>{n.config.sliceNsMin?.toLocaleString() || '—'}</span>
                              </div>
                              <div className="detail-item">
                                <span className="detail-label">Kernel Mode</span>
                                <span className="detail-value">{n.config.kernelMode ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="detail-item">
                                <span className="detail-label">Max-Time Watchdog</span>
                                <span className="detail-value">{n.config.maxTimeWatchdog ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="detail-item">
                                <span className="detail-label">Early Processing</span>
                                <span className="detail-value">{n.config.earlyProcessing ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="detail-item">
                                <span className="detail-label">Built-in Idle</span>
                                <span className="detail-value">{n.config.builtinIdle ? 'Yes' : 'No'}</span>
                              </div>
                            </>
                          ) : (
                            <p style={{ color: 'var(--color-text-secondary)', gridColumn: '1 / -1' }}>
                              No config has been applied to this node yet. Use the form above to apply a config.
                            </p>
                          )}
                        </div>
                        <div style={{ display: 'flex', gap: 20, fontSize: 12, color: 'var(--color-text-secondary)', flexWrap: 'wrap' }}>
                          {n.appliedAt && <span>Applied: {new Date(n.appliedAt).toLocaleString()}</span>}
                          {n.restartCount != null && <span>Restarts: {n.restartCount}</span>}
                        </div>
                        {n.lastError && (
                          <div style={{ marginTop: 8, padding: '6px 10px', background: 'var(--color-danger-bg, rgba(239,68,68,0.1))', borderRadius: 6, fontSize: 12, color: 'var(--color-error)' }}>
                            <strong>Error:</strong> {n.lastError}
                          </div>
                        )}
                        <div style={{ marginTop: 10 }}>
                          <button
                            className="btn btn-ghost btn-sm"
                            onClick={() => navigate(`/nodes/${encodeURIComponent(n.nodeId)}`)}
                          >
                            <Eye size={14} />
                            View Full Node Details
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          ) : !loadingStatus ? (
            <p style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>No node status available.</p>
          ) : null}
        </div>
      </div>

    </div>
  );
}
