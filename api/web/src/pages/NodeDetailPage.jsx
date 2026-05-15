import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, Link, useLocation } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import {
  ArrowLeft,
  RefreshCw,
  Server,
  ChevronDown,
  ChevronRight,
  Loader2,
  Inbox,
  Cpu,
  CheckCircle,
  XCircle,
  AlertCircle,
  Save,
  ExternalLink,
} from 'lucide-react';

const SCX_SCHEDULERS = [
  'scx_bpfland',
  'scx_cake',
  'scx_lavd',
  'scx_layered',
  'scx_rustland',
  'scx_rusty',
  'scx_tickless',
  'scx_timely',
  'scx_wd40',
];

const DEFAULT_RUNTIME_CONFIG = {
  schedulerEnabled: true,
  monitoringEnabled: true,
  mode: 'gthulhu',
  schedulerName: 'scx_bpfland',
  sliceNsDefault: 20000000,
  sliceNsMin: 1000000,
  kernelMode: true,
  maxTimeWatchdog: true,
  earlyProcessing: false,
  builtinIdle: false,
};

function truncateText(text, maxLen) {
  if (!text) return '--';
  if (text.length <= maxLen) return text;
  return text.substring(0, maxLen - 3) + '...';
}

export default function NodeDetailPage() {
  const { nodeId } = useParams();
  const location = useLocation();
  const { isAuthenticated, makeAuthenticatedRequest, showToast, getRuntimeConfigStatus, applyRuntimeConfig } = useApp();

  const [pods, setPods] = useState([]);
  const [nodeName, setNodeName] = useState('--');
  const [totalProcesses, setTotalProcesses] = useState(0);
  const [lastUpdated, setLastUpdated] = useState('--');
  const [loading, setLoading] = useState(false);
  const [expandedPods, setExpandedPods] = useState(new Set());
  const [autoRefresh, setAutoRefresh] = useState(false);
  const intervalRef = useRef(null);

  // Node scheduler runtime config
  const [nodeConfig, setNodeConfig] = useState(null);
  const [loadingConfig, setLoadingConfig] = useState(false);
  const [editing, setEditing] = useState(false);
  const [applying, setApplying] = useState(false);
  const [form, setForm] = useState(DEFAULT_RUNTIME_CONFIG);

  // Open editor automatically when navigated with ?edit=1 query param.
  useEffect(() => {
    const params = new URLSearchParams(location.search);
    if (params.get('edit') === '1') {
      setEditing(true);
    }
  }, [location.search]);

  const fetchPods = useCallback(async () => {
    if (!isAuthenticated || !nodeId) return;
    setLoading(true);
    try {
      const response = await makeAuthenticatedRequest(
        `/api/v1/nodes/${encodeURIComponent(nodeId)}/pods/pids`
      );
      const data = await response.json();
      if (data.success && data.data) {
        const loadedPods = data.data.pods || [];
        let processCount = 0;
        loadedPods.forEach((pod) => {
          processCount += (pod.processes || []).length;
        });
        setPods(loadedPods);
        setNodeName(data.data.node_name || 'Unknown');
        setTotalProcesses(processCount);
        setLastUpdated(
          data.data.timestamp
            ? new Date(data.data.timestamp).toLocaleTimeString()
            : new Date().toLocaleTimeString()
        );
      } else {
        throw new Error(data.error || data.message || 'Failed');
      }
    } catch (err) {
      showToast('error', err.message);
      setPods([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, nodeId, showToast]);

  const fetchNodeConfig = useCallback(async () => {
    if (!isAuthenticated || !nodeId) return;
    setLoadingConfig(true);
    try {
      const results = await getRuntimeConfigStatus([nodeId]);
      setNodeConfig(results.length > 0 ? results[0] : null);
    } catch (err) {
      // Silently handle — config may not be available for all nodes
      setNodeConfig(null);
    } finally {
      setLoadingConfig(false);
    }
  }, [isAuthenticated, nodeId, getRuntimeConfigStatus]);

  // Seed edit form from the latest nodeConfig whenever it changes (and not actively editing).
  useEffect(() => {
    if (editing) return;
    const cfg = nodeConfig?.config || nodeConfig?.desiredConfig;
    if (cfg) {
      setForm({
        schedulerEnabled: cfg.schedulerEnabled ?? true,
        monitoringEnabled: cfg.monitoringEnabled ?? true,
        mode: cfg.mode || 'gthulhu',
        schedulerName: cfg.schedulerName || 'scx_bpfland',
        sliceNsDefault: cfg.sliceNsDefault ?? 20000000,
        sliceNsMin: cfg.sliceNsMin ?? 1000000,
        kernelMode: cfg.kernelMode ?? true,
        maxTimeWatchdog: cfg.maxTimeWatchdog ?? true,
        earlyProcessing: cfg.earlyProcessing ?? false,
        builtinIdle: cfg.builtinIdle ?? false,
      });
    } else {
      setForm(DEFAULT_RUNTIME_CONFIG);
    }
  }, [nodeConfig, editing]);

  const updateForm = (key, value) => setForm((prev) => ({ ...prev, [key]: value }));

  const handleApplyNodeConfig = async () => {
    if (!nodeId) return;
    setApplying(true);
    try {
      const configVersion = new Date().toISOString();
      const config = {
        ...form,
        configVersion,
        schedulerEnabled: form.mode !== 'none',
        schedulerName: form.mode === 'scx' ? form.schedulerName : '',
      };
      const results = await applyRuntimeConfig([nodeId], config);
      setNodeConfig(results.length > 0 ? results[0] : null);
      setEditing(false);
      showToast('success', `Applied runtime config to ${nodeId}`);
    } catch (err) {
      showToast('error', err.message);
    } finally {
      setApplying(false);
    }
  };

  const cancelEdit = () => {
    setEditing(false);
    // Re-seed form by triggering the effect via a no-op state change.
    const cfg = nodeConfig?.config || nodeConfig?.desiredConfig;
    if (cfg) {
      setForm({
        schedulerEnabled: cfg.schedulerEnabled ?? true,
        monitoringEnabled: cfg.monitoringEnabled ?? true,
        mode: cfg.mode || 'gthulhu',
        schedulerName: cfg.schedulerName || 'scx_bpfland',
        sliceNsDefault: cfg.sliceNsDefault ?? 20000000,
        sliceNsMin: cfg.sliceNsMin ?? 1000000,
        kernelMode: cfg.kernelMode ?? true,
        maxTimeWatchdog: cfg.maxTimeWatchdog ?? true,
        earlyProcessing: cfg.earlyProcessing ?? false,
        builtinIdle: cfg.builtinIdle ?? false,
      });
    } else {
      setForm(DEFAULT_RUNTIME_CONFIG);
    }
  };

  useEffect(() => {
    if (isAuthenticated) {
      fetchPods();
      fetchNodeConfig();
    }
  }, [isAuthenticated, fetchPods, fetchNodeConfig]);

  useEffect(() => {
    if (autoRefresh && isAuthenticated) {
      intervalRef.current = setInterval(fetchPods, 5000);
    } else {
      if (intervalRef.current) clearInterval(intervalRef.current);
    }
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [autoRefresh, isAuthenticated, fetchPods]);

  const togglePod = (uid) => {
    setExpandedPods((prev) => {
      const next = new Set(prev);
      next.has(uid) ? next.delete(uid) : next.add(uid);
      return next;
    });
  };

  const expandAll = () => setExpandedPods(new Set(pods.map((p) => p.pod_uid)));
  const collapseAll = () => setExpandedPods(new Set());

  return (
    <div>
      {/* Page header */}
      <div className="page-header">
        <div>
          <Link to="/nodes" className="btn btn-ghost btn-sm" style={{ marginBottom: 4 }}>
            <ArrowLeft size={14} />
            <span>Back to Nodes</span>
          </Link>
          <h1 className="page-title">Node: {nodeId}</h1>
          <p className="page-subtitle">Pod-PID mapping browser</p>
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
          <button className="btn btn-secondary btn-sm" onClick={fetchPods} disabled={loading}>
            <RefreshCw size={14} className={loading ? 'spin' : ''} />
            <span>Refresh</span>
          </button>
        </div>
      </div>

      {/* Stat cards */}
      <div className="stat-cards">
        <div className="stat-card">
          <div className="stat-card-label">Node Name</div>
          <div className="stat-card-value" style={{ fontSize: 16 }}>{nodeName}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Pods</div>
          <div className="stat-card-value">{pods.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Processes</div>
          <div className="stat-card-value">{totalProcesses}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Last Updated</div>
          <div className="stat-card-value" style={{ fontSize: 14 }}>{lastUpdated}</div>
        </div>
      </div>

      {/* Scheduler Runtime Config for this node */}
      <div className="card">
        <div className="card-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h3 className="card-title">
            <Cpu size={16} />
            Scheduler Runtime Config
          </h3>
          <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
            <a
              href="https://wiki.cachyos.org/configuration/sched-ext/"
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-ghost btn-sm"
              title="Learn more about scx schedulers"
              style={{ display: 'flex', alignItems: 'center', gap: 4 }}
            >
              <ExternalLink size={14} />
              <span>About scx schedulers</span>
            </a>
            {!editing && (
              <button
                className="btn btn-secondary btn-sm"
                onClick={() => setEditing(true)}
                disabled={loadingConfig}
                title="Edit config for this node"
              >
                Edit
              </button>
            )}
            <button
              className="btn btn-ghost btn-sm"
              onClick={fetchNodeConfig}
              disabled={loadingConfig}
              title="Refresh config"
            >
              <RefreshCw size={14} className={loadingConfig ? 'spin' : ''} />
            </button>
          </div>
        </div>
        <div className="card-body" style={{ padding: 'var(--space-xl)' }}>
          {loadingConfig ? (
            <div className="empty-state">
              <Loader2 size={20} className="spin" />
              <p>Loading scheduler config...</p>
            </div>
          ) : editing ? (
            <div>
              <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 16 }}>
                Editing runtime config for node <strong>{nodeId}</strong> only. Changes will be applied to this node when you click <em>Apply to this Node</em>.
              </p>

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
                      checked={form[key]}
                      onChange={(e) => updateForm(key, e.target.checked)}
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
                    value={form.mode}
                    onChange={(e) => updateForm('mode', e.target.value)}
                  >
                    <option value="none">none</option>
                    <option value="gthulhu">gthulhu</option>
                    <option value="simple">simple</option>
                    <option value="scx">scx</option>
                  </select>
                </div>
                {form.mode === 'scx' && (
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label className="form-label">SCX Scheduler</label>
                    <select
                      className="form-input"
                      value={form.schedulerName}
                      onChange={(e) => updateForm('schedulerName', e.target.value)}
                    >
                      {SCX_SCHEDULERS.map((name) => (
                        <option key={name} value={name}>{name}</option>
                      ))}
                    </select>
                  </div>
                )}
                <div className="form-group" style={{ marginBottom: 0 }}>
                  <label className="form-label">Default Slice (ns)</label>
                  <input
                    type="number"
                    className="form-input"
                    step={1000000}
                    min={1000000}
                    value={form.sliceNsDefault}
                    onChange={(e) => updateForm('sliceNsDefault', Number(e.target.value))}
                  />
                </div>
                <div className="form-group" style={{ marginBottom: 0 }}>
                  <label className="form-label">Min Slice (ns)</label>
                  <input
                    type="number"
                    className="form-input"
                    step={100000}
                    min={100000}
                    value={form.sliceNsMin}
                    onChange={(e) => updateForm('sliceNsMin', Number(e.target.value))}
                  />
                </div>
              </div>

              <div style={{ display: 'flex', gap: 8 }}>
                <button className="btn btn-secondary btn-sm" onClick={cancelEdit} disabled={applying}>
                  Cancel
                </button>
                <button
                  className="btn btn-primary btn-sm"
                  onClick={handleApplyNodeConfig}
                  disabled={applying}
                >
                  <Save size={14} />
                  <span>{applying ? 'Applying…' : 'Apply to this Node'}</span>
                </button>
              </div>
            </div>
          ) : nodeConfig ? (
            <div>
              {/* Status banner */}
              <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 8 }}>
                {nodeConfig.success ? (
                  <span className="badge badge-success" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                    <CheckCircle size={12} /> Applied
                  </span>
                ) : nodeConfig.lastError ? (
                  <span className="badge badge-danger" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                    <XCircle size={12} /> Error
                  </span>
                ) : (
                  <span className="badge badge-secondary" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                    <AlertCircle size={12} /> Unknown
                  </span>
                )}
                {nodeConfig.configVersion && (
                  <span style={{ fontSize: 12, color: 'var(--color-text-secondary)', fontFamily: 'monospace' }}>
                    v{nodeConfig.configVersion}
                  </span>
                )}
                {nodeConfig.drift && (
                  <span className="badge badge-warning" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                    <AlertCircle size={12} /> Drift
                  </span>
                )}
              </div>

              {/* Config details grid */}
              <div className="detail-grid" style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 12 }}>
                {nodeConfig.config ? (
                  <>
                    <div className="detail-item">
                      <span className="detail-label">Mode</span>
                      <span className="detail-value">{nodeConfig.config.mode || '—'}</span>
                    </div>
                    {nodeConfig.config.schedulerName && (
                      <div className="detail-item">
                        <span className="detail-label">Scheduler</span>
                        <span className="detail-value">{nodeConfig.config.schedulerName}</span>
                      </div>
                    )}
                    <div className="detail-item">
                      <span className="detail-label">Scheduler Enabled</span>
                      <span className="detail-value">{nodeConfig.config.schedulerEnabled ? 'Yes' : 'No'}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Monitoring Enabled</span>
                      <span className="detail-value">{nodeConfig.config.monitoringEnabled ? 'Yes' : 'No'}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Default Slice (ns)</span>
                      <span className="detail-value" style={{ fontFamily: 'monospace' }}>{nodeConfig.config.sliceNsDefault?.toLocaleString() || '—'}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Min Slice (ns)</span>
                      <span className="detail-value" style={{ fontFamily: 'monospace' }}>{nodeConfig.config.sliceNsMin?.toLocaleString() || '—'}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Kernel Mode</span>
                      <span className="detail-value">{nodeConfig.config.kernelMode ? 'Yes' : 'No'}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Max-Time Watchdog</span>
                      <span className="detail-value">{nodeConfig.config.maxTimeWatchdog ? 'Yes' : 'No'}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Early Processing</span>
                      <span className="detail-value">{nodeConfig.config.earlyProcessing ? 'Yes' : 'No'}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Built-in Idle</span>
                      <span className="detail-value">{nodeConfig.config.builtinIdle ? 'Yes' : 'No'}</span>
                    </div>
                  </>
                ) : (
                  <div style={{ gridColumn: '1 / -1' }}>
                    <p style={{ color: 'var(--color-text-secondary)', marginBottom: 12 }}>
                      No config has been applied to this node yet. Apply a config from the <a href="#/nodes" style={{ color: 'var(--color-primary)' }}>Nodes & Health</a> page.
                    </p>
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 12 }}>
                      {nodeConfig.host && (
                        <div className="detail-item">
                          <span className="detail-label">Host</span>
                          <span className="detail-value">{nodeConfig.host}</span>
                        </div>
                      )}
                      <div className="detail-item">
                        <span className="detail-label">Applied At</span>
                        <span className="detail-value">{nodeConfig.appliedAt ? new Date(nodeConfig.appliedAt).toLocaleString() : '—'}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">Restarts</span>
                        <span className="detail-value">{nodeConfig.restartCount ?? '—'}</span>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Meta info row */}
              {nodeConfig.config && (
                <div style={{ marginTop: 16, display: 'flex', gap: 24, fontSize: 12, color: 'var(--color-text-secondary)' }}>
                  {nodeConfig.host && <span>Host: {nodeConfig.host}</span>}
                  {nodeConfig.appliedAt && <span>Applied: {new Date(nodeConfig.appliedAt).toLocaleString()}</span>}
                  {nodeConfig.restartCount != null && <span>Restarts: {nodeConfig.restartCount}</span>}
                </div>
              )}

              {nodeConfig.desiredConfig && (
                <div style={{ marginTop: 8, fontSize: 12, color: 'var(--color-text-secondary)' }}>
                  Desired: {nodeConfig.desiredConfig.mode || '—'}{nodeConfig.desiredConfig.schedulerName ? ` / ${nodeConfig.desiredConfig.schedulerName}` : ''}
                </div>
              )}

              {nodeConfig.lastError && (
                <div style={{ marginTop: 12, padding: '8px 12px', background: 'var(--color-danger-bg, rgba(239,68,68,0.1))', borderRadius: 6, fontSize: 12, color: 'var(--color-error)' }}>
                  <strong>Error:</strong> {nodeConfig.lastError}
                </div>
              )}
            </div>
          ) : (
            <div className="empty-state">
              <Cpu size={20} />
              <p>No scheduler config available for this node</p>
              <button
                className="btn btn-primary btn-sm"
                style={{ marginTop: 8 }}
                onClick={() => setEditing(true)}
              >
                Create config
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Pods */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <Server size={16} />
            Pods
          </h3>
          <div style={{ display: 'flex', gap: 6 }}>
            <button className="btn btn-ghost btn-sm" onClick={expandAll}>Expand All</button>
            <button className="btn btn-ghost btn-sm" onClick={collapseAll}>Collapse All</button>
          </div>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {loading && pods.length === 0 ? (
            <div className="empty-state">
              <Loader2 size={20} className="spin" />
              <p>Loading pods...</p>
            </div>
          ) : pods.length === 0 ? (
            <div className="empty-state">
              <Inbox size={20} />
              <p>No pods found on this node</p>
            </div>
          ) : (
            <div>
              {pods.map((pod) => {
                const processes = pod.processes || [];
                const uid = pod.pod_uid || '--';
                const podId = pod.pod_id || '--';
                const isExpanded = expandedPods.has(uid);

                return (
                  <div key={uid} style={{ borderBottom: '1px solid var(--color-border)' }}>
                    <div
                      onClick={() => togglePod(uid)}
                      style={{
                        padding: '12px 16px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        cursor: 'pointer',
                        transition: 'background 0.15s',
                      }}
                      onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--color-page-bg)')}
                      onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                        {isExpanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                        <span style={{ fontWeight: 500, fontSize: 13 }}>{podId}</span>
                        <span style={{
                          fontSize: 11,
                          color: 'var(--color-text-secondary)',
                          fontFamily: 'monospace',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                        }}>
                          {uid}
                        </span>
                      </div>
                      <span className="badge badge-secondary">
                        {processes.length} process{processes.length !== 1 ? 'es' : ''}
                      </span>
                    </div>
                    {isExpanded && (
                      <div style={{ padding: '0 16px 12px 40px' }}>
                        {processes.length === 0 ? (
                          <p style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>No processes</p>
                        ) : (
                          <table className="data-table" style={{ fontSize: 12 }}>
                            <thead>
                              <tr>
                                <th>PID</th>
                                <th>COMMAND</th>
                                <th>PPID</th>
                                <th>CONTAINER ID</th>
                              </tr>
                            </thead>
                            <tbody>
                              {processes.map((proc, i) => (
                                <tr key={i}>
                                  <td style={{ fontFamily: 'monospace' }}>{proc.pid || '--'}</td>
                                  <td title={proc.command || ''}>{proc.command || '--'}</td>
                                  <td style={{ fontFamily: 'monospace' }}>{proc.ppid || '--'}</td>
                                  <td title={proc.container_id || ''} style={{ fontFamily: 'monospace' }}>
                                    {truncateText(proc.container_id, 16)}
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
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
    </div>
  );
}
