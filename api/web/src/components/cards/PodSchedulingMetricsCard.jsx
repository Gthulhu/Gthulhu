import React, { useState, useEffect, useCallback } from 'react';
import { useApp } from '../../context/AppContext';
import { BarChart3, Download, Trash2, Save, Loader2, XCircle, Inbox, ChevronDown, ChevronRight, HelpCircle, Pencil, Plus } from 'lucide-react';

function formatMetricValue(value) {
  return new Intl.NumberFormat().format(value || 0);
}

export default function PodSchedulingMetricsCard() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();
  const [items, setItems] = useState([]);
  const [expandedItems, setExpandedItems] = useState({});
  const [editingId, setEditingId] = useState(null);
  const [editForm, setEditForm] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [runtimeItems, setRuntimeItems] = useState([]);
  const [runtimeWarnings, setRuntimeWarnings] = useState([]);
  const [loadingRuntime, setLoadingRuntime] = useState(false);
  const [runtimeError, setRuntimeError] = useState('');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [createForm, setCreateForm] = useState(newCreateForm());

  function newCreateForm() {
    return {
      selectors: [{ key: '', value: '' }],
      k8sNamespaces: '',
      commandRegex: '',
      collectionIntervalSeconds: 10,
      enabled: true,
      metrics: {
        voluntaryCtxSwitches: true,
        involuntaryCtxSwitches: true,
        cpuTimeNs: true,
        waitTimeNs: false,
        runCount: false,
        cpuMigrations: false,
      },
      scalingEnabled: false,
      scalingMetricName: '',
      scalingTargetValue: '',
      scalingTargetName: '',
      scalingTargetKind: 'Deployment',
      scalingMinReplicas: 1,
      scalingMaxReplicas: 10,
      scalingCooldown: 300,
    };
  }

  const loadItems = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoading(true);
    setError('');
    try {
      const response = await makeAuthenticatedRequest('/api/v1/pod-scheduling-metrics');
      const data = await response.json();
      if (data.success) {
        const loaded = data.data && data.data.items ? data.data.items : [];
        setItems(loaded);
        if (loaded.length > 0) {
          showToast('success', `Loaded ${loaded.length} PodSchedulingMetrics`);
        } else {
          showToast('info', 'No PodSchedulingMetrics found');
        }
      } else {
        setError(data.error || 'Failed to load');
        setItems([]);
      }
    } catch (err) {
      setError(err.message);
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast]);

  const loadRuntimeMetrics = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoadingRuntime(true);
    setRuntimeError('');
    try {
      const response = await makeAuthenticatedRequest('/api/v1/pod-scheduling-metrics/runtime');
      const data = await response.json();
      if (data.success) {
        setRuntimeItems(data.data && data.data.items ? data.data.items : []);
        setRuntimeWarnings(data.data && data.data.warnings ? data.data.warnings : []);
      } else {
        setRuntimeError(data.error || 'Failed to load runtime metrics');
        setRuntimeItems([]);
        setRuntimeWarnings([]);
      }
    } catch (err) {
      setRuntimeError(err.message);
      setRuntimeItems([]);
      setRuntimeWarnings([]);
    } finally {
      setLoadingRuntime(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest]);

  const refreshAllMetricsData = useCallback(() => {
    loadItems();
    loadRuntimeMetrics();
  }, [loadItems, loadRuntimeMetrics]);

  useEffect(() => {
    const handler = () => refreshAllMetricsData();
    window.addEventListener('refreshPSM', handler);
    return () => window.removeEventListener('refreshPSM', handler);
  }, [refreshAllMetricsData]);

  useEffect(() => {
    if (isAuthenticated) {
      refreshAllMetricsData();
    }
  }, [isAuthenticated, refreshAllMetricsData]);

  const toggleExpand = (id) => setExpandedItems(prev => ({ ...prev, [id]: !prev[id] }));

  // ----- Create -----
  const handleCreate = async () => {
    const labelSelectors = createForm.selectors
      .filter(s => s.key.trim() && s.value.trim())
      .map(s => ({ key: s.key.trim(), value: s.value.trim() }));
    if (labelSelectors.length === 0) {
      showToast('error', 'At least one label selector is required');
      return;
    }
    const payload = {
      labelSelectors,
      commandRegex: createForm.commandRegex.trim() || undefined,
      collectionIntervalSeconds: parseInt(createForm.collectionIntervalSeconds, 10) || 10,
      enabled: createForm.enabled,
      metrics: createForm.metrics,
    };
    const ns = createForm.k8sNamespaces.split(',').map(s => s.trim()).filter(Boolean);
    if (ns.length > 0) payload.k8sNamespaces = ns;
    if (createForm.scalingEnabled) {
      payload.scaling = {
        enabled: true,
        metricName: createForm.scalingMetricName,
        targetValue: createForm.scalingTargetValue,
        scaleTargetRef: {
          kind: createForm.scalingTargetKind || 'Deployment',
          name: createForm.scalingTargetName,
          apiVersion: 'apps/v1',
        },
        minReplicaCount: parseInt(createForm.scalingMinReplicas, 10) || 1,
        maxReplicaCount: parseInt(createForm.scalingMaxReplicas, 10) || 10,
        cooldownPeriod: parseInt(createForm.scalingCooldown, 10) || 300,
      };
    }
    try {
      const response = await makeAuthenticatedRequest('/api/v1/pod-scheduling-metrics', {
        method: 'POST',
        body: JSON.stringify(payload),
      });
      const data = await response.json();
      if (data.success) {
        showToast('success', 'PodSchedulingMetrics created');
        setShowCreateForm(false);
        setCreateForm(newCreateForm());
        loadItems();
      } else {
        showToast('error', data.error || 'Failed to create');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  // ----- Delete -----
  const handleDelete = async (id) => {
    try {
      const response = await makeAuthenticatedRequest('/api/v1/pod-scheduling-metrics', {
        method: 'DELETE',
        body: JSON.stringify({ id }),
      });
      const data = await response.json();
      if (data.success) {
        showToast('success', 'Deleted');
        loadItems();
      } else {
        showToast('error', data.error || 'Failed to delete');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  // ----- Edit -----
  const beginEdit = (item) => {
    const selectors = (item.labelSelectors || []).map(s => ({ key: s.key || '', value: s.value || '' }));
    if (selectors.length === 0) selectors.push({ key: '', value: '' });
    setEditingId(item.id);
    setEditForm({
      id: item.id,
      selectors,
      k8sNamespaces: (item.k8sNamespaces || []).join(', '),
      commandRegex: item.commandRegex || '',
      collectionIntervalSeconds: item.collectionIntervalSeconds || 10,
      enabled: item.enabled !== undefined ? item.enabled : true,
      metrics: item.metrics || {
        voluntaryCtxSwitches: true,
        involuntaryCtxSwitches: true,
        cpuTimeNs: true,
        waitTimeNs: false,
        runCount: false,
        cpuMigrations: false,
      },
      scalingEnabled: item.scaling ? item.scaling.enabled : false,
      scalingMetricName: item.scaling ? item.scaling.metricName || '' : '',
      scalingTargetValue: item.scaling ? item.scaling.targetValue || '' : '',
      scalingTargetName: item.scaling && item.scaling.scaleTargetRef ? item.scaling.scaleTargetRef.name || '' : '',
      scalingTargetKind: item.scaling && item.scaling.scaleTargetRef ? item.scaling.scaleTargetRef.kind || 'Deployment' : 'Deployment',
      scalingMinReplicas: item.scaling ? item.scaling.minReplicaCount || 1 : 1,
      scalingMaxReplicas: item.scaling ? item.scaling.maxReplicaCount || 10 : 10,
      scalingCooldown: item.scaling ? item.scaling.cooldownPeriod || 300 : 300,
    });
    setExpandedItems(prev => ({ ...prev, [item.id]: true }));
  };

  const cancelEdit = () => { setEditingId(null); setEditForm(null); };

  const handleUpdate = async () => {
    if (!editForm) return;
    const labelSelectors = editForm.selectors
      .filter(s => s.key.trim() && s.value.trim())
      .map(s => ({ key: s.key.trim(), value: s.value.trim() }));
    const payload = {
      id: editForm.id,
      labelSelectors,
      commandRegex: editForm.commandRegex.trim() || undefined,
      collectionIntervalSeconds: parseInt(editForm.collectionIntervalSeconds, 10) || 10,
      enabled: editForm.enabled,
      metrics: editForm.metrics,
    };
    const ns = editForm.k8sNamespaces.split(',').map(s => s.trim()).filter(Boolean);
    if (ns.length > 0) payload.k8sNamespaces = ns;
    if (editForm.scalingEnabled) {
      payload.scaling = {
        enabled: true,
        metricName: editForm.scalingMetricName,
        targetValue: editForm.scalingTargetValue,
        scaleTargetRef: {
          kind: editForm.scalingTargetKind || 'Deployment',
          name: editForm.scalingTargetName,
          apiVersion: 'apps/v1',
        },
        minReplicaCount: parseInt(editForm.scalingMinReplicas, 10) || 1,
        maxReplicaCount: parseInt(editForm.scalingMaxReplicas, 10) || 10,
        cooldownPeriod: parseInt(editForm.scalingCooldown, 10) || 300,
      };
    }
    try {
      const response = await makeAuthenticatedRequest('/api/v1/pod-scheduling-metrics', {
        method: 'PUT',
        body: JSON.stringify(payload),
      });
      const data = await response.json();
      if (data.success) {
        showToast('success', 'Updated');
        cancelEdit();
        loadItems();
      } else {
        showToast('error', data.error || 'Failed to update');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  // ----- Selector helpers (shared between create/edit) -----
  const updateFormSelector = (form, setForm, index, field, value) => {
    setForm(prev => {
      const selectors = [...prev.selectors];
      selectors[index] = { ...selectors[index], [field]: value };
      return { ...prev, selectors };
    });
  };
  const addFormSelector = (form, setForm) => {
    setForm(prev => ({ ...prev, selectors: [...prev.selectors, { key: '', value: '' }] }));
  };
  const removeFormSelector = (form, setForm, index) => {
    setForm(prev => {
      const selectors = prev.selectors.filter((_, i) => i !== index);
      if (selectors.length === 0) selectors.push({ key: '', value: '' });
      return { ...prev, selectors };
    });
  };

  // Render a selector list for a form
  function renderSelectors(form, setForm) {
    return (
      <div className="full-width selectors-container">
        <label>Label Selectors</label>
        <div className="selectors-list">
          {form.selectors.map((sel, idx) => (
            <div key={idx} className="selector-row">
              <input type="text" placeholder="Key" value={sel.key}
                onChange={(e) => updateFormSelector(form, setForm, idx, 'key', e.target.value)} />
              <input type="text" placeholder="Value" value={sel.value}
                onChange={(e) => updateFormSelector(form, setForm, idx, 'value', e.target.value)} />
              <button type="button" onClick={() => removeFormSelector(form, setForm, idx)}>✕</button>
            </div>
          ))}
        </div>
        <button type="button" className="add-selector-btn" onClick={() => addFormSelector(form, setForm)}>
          + Add Selector
        </button>
      </div>
    );
  }

  function renderMetricsToggles(metrics, setForm) {
    const flags = [
      ['voluntaryCtxSwitches', 'Voluntary Ctx Switches'],
      ['involuntaryCtxSwitches', 'Involuntary Ctx Switches'],
      ['cpuTimeNs', 'CPU Time (ns)'],
      ['waitTimeNs', 'Wait Time (ns)'],
      ['runCount', 'Run Count'],
      ['cpuMigrations', 'CPU Migrations'],
    ];
    return (
      <div className="full-width">
        <label>Metrics to Collect</label>
        <div className="detail-grid" style={{ gap: '6px' }}>
          {flags.map(([key, label]) => (
            <label key={key} style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.85rem' }}>
              <input type="checkbox" checked={metrics[key]}
                onChange={() => setForm(prev => ({ ...prev, metrics: { ...prev.metrics, [key]: !prev.metrics[key] } }))} />
              {label}
            </label>
          ))}
        </div>
      </div>
    );
  }

  function renderScalingSection(form, setForm) {
    return (
      <div className="full-width">
        <label style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
          <input type="checkbox" checked={form.scalingEnabled}
            onChange={() => setForm(prev => ({ ...prev, scalingEnabled: !prev.scalingEnabled }))} />
          Enable KEDA Auto-Scaling
        </label>
        {form.scalingEnabled && (
          <div className="strategy-form" style={{ marginTop: '8px' }}>
            <div>
              <label>Metric Name</label>
              <input type="text" value={form.scalingMetricName} placeholder="e.g. gthulhu_pod_voluntary_ctx_switches_total"
                onChange={(e) => setForm(prev => ({ ...prev, scalingMetricName: e.target.value }))} />
            </div>
            <div>
              <label>Target Value</label>
              <input type="text" value={form.scalingTargetValue} placeholder="e.g. 100"
                onChange={(e) => setForm(prev => ({ ...prev, scalingTargetValue: e.target.value }))} />
            </div>
            <div>
              <label>Scale Target Name</label>
              <input type="text" value={form.scalingTargetName} placeholder="Deployment name"
                onChange={(e) => setForm(prev => ({ ...prev, scalingTargetName: e.target.value }))} />
            </div>
            <div>
              <label>Scale Target Kind</label>
              <input type="text" value={form.scalingTargetKind} placeholder="Deployment"
                onChange={(e) => setForm(prev => ({ ...prev, scalingTargetKind: e.target.value }))} />
            </div>
            <div>
              <label>Min Replicas</label>
              <input type="number" min="0" value={form.scalingMinReplicas}
                onChange={(e) => setForm(prev => ({ ...prev, scalingMinReplicas: e.target.value }))} />
            </div>
            <div>
              <label>Max Replicas</label>
              <input type="number" min="1" value={form.scalingMaxReplicas}
                onChange={(e) => setForm(prev => ({ ...prev, scalingMaxReplicas: e.target.value }))} />
            </div>
            <div>
              <label>Cooldown (s)</label>
              <input type="number" min="0" value={form.scalingCooldown}
                onChange={(e) => setForm(prev => ({ ...prev, scalingCooldown: e.target.value }))} />
            </div>
          </div>
        )}
      </div>
    );
  }

  return (
    <section className="card strategies-card full-width">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><BarChart3 size={18} /></span>
          <h2>Pod Scheduling Metrics</h2>
          <div className="help-tooltip">
            <HelpCircle size={14} className="help-icon" />
            <div className="tooltip-content">
              <p><strong>PodSchedulingMetrics</strong> define which pods to monitor for scheduling-related metrics (context switches, CPU time, etc.).</p>
              <p>Optionally enable KEDA auto-scaling based on these metrics.</p>
            </div>
          </div>
        </div>
        <div className="card-actions">
          <button className="icon-btn auth-required" onClick={refreshAllMetricsData} title="Refresh" disabled={!isAuthenticated}>
            <Download size={16} />
          </button>
          <button className="primary-btn auth-required" onClick={() => setShowCreateForm(prev => !prev)} disabled={!isAuthenticated}>
            <Plus size={16} /> New
          </button>
        </div>
      </div>
      <div className="card-body">
        {/* Create Form */}
        {showCreateForm && (
          <div className="strategy-item">
            <div className="strategy-header">
              <h4>New PodSchedulingMetrics</h4>
              <button type="button" className="remove-strategy-btn" onClick={() => { setShowCreateForm(false); setCreateForm(newCreateForm()); }}>✕ Cancel</button>
            </div>
            <div className="strategy-form">
              <div className="full-width">
                <label>K8s Namespaces (comma separated)</label>
                <input type="text" placeholder="default, kube-system" value={createForm.k8sNamespaces}
                  onChange={(e) => setCreateForm(prev => ({ ...prev, k8sNamespaces: e.target.value }))} />
              </div>
              <div>
                <label>Command Regex</label>
                <input type="text" placeholder="e.g. upf|amf" value={createForm.commandRegex}
                  onChange={(e) => setCreateForm(prev => ({ ...prev, commandRegex: e.target.value }))} />
              </div>
              <div>
                <label>Collection Interval (s)</label>
                <input type="number" min="1" max="3600" value={createForm.collectionIntervalSeconds}
                  onChange={(e) => setCreateForm(prev => ({ ...prev, collectionIntervalSeconds: e.target.value }))} />
              </div>
              <div>
                <label style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <input type="checkbox" checked={createForm.enabled}
                    onChange={() => setCreateForm(prev => ({ ...prev, enabled: !prev.enabled }))} />
                  Enabled
                </label>
              </div>
              {renderSelectors(createForm, setCreateForm)}
              {renderMetricsToggles(createForm.metrics, setCreateForm)}
              {renderScalingSection(createForm, setCreateForm)}
            </div>
            <div className="strategies-actions" style={{ display: 'flex' }}>
              <button className="danger-btn" onClick={() => { setShowCreateForm(false); setCreateForm(newCreateForm()); }}>
                Cancel
              </button>
              <button className="success-btn auth-required" onClick={handleCreate} disabled={!isAuthenticated}>
                <Save size={16} /> Create
              </button>
            </div>
          </div>
        )}

        <div className="loaded-strategies-section">
          <h3 className="section-title"><BarChart3 size={16} /> Latest Collected Pod Metrics</h3>

          {loadingRuntime && (
            <div className="empty-state">
              <span className="empty-icon"><Loader2 size={24} className="spin" /></span>
              <p>Loading runtime metrics...</p>
            </div>
          )}

          {!loadingRuntime && runtimeError && (
            <div className="empty-state error">
              <span className="empty-icon"><XCircle size={24} /></span>
              <p>Error: {runtimeError}</p>
            </div>
          )}

          {!loadingRuntime && !runtimeError && runtimeWarnings.length > 0 && (
            <div className="labels-section">
              <span className="labels-title">Collection Warnings</span>
              <div className="labels-grid">
                {runtimeWarnings.map((warning, index) => (
                  <span key={index} className="strategy-namespace-badge disabled">{warning}</span>
                ))}
              </div>
            </div>
          )}

          {!loadingRuntime && !runtimeError && runtimeItems.length === 0 && (
            <div className="empty-state">
              <span className="empty-icon"><Inbox size={24} /></span>
              <p>No runtime pod scheduling metrics collected yet.</p>
            </div>
          )}

          {!loadingRuntime && !runtimeError && runtimeItems.length > 0 && (
            <div className="users-list">
              {runtimeItems.map((item) => (
                <div key={`${item.namespace}-${item.podName}-${item.nodeID || 'unknown'}`} className="strategy-loaded-item">
                  <div className="strategy-loaded-header">
                    <div className="strategy-loaded-title">
                      <span className="strategy-id">{item.namespace}/{item.podName}</span>
                      {item.nodeID && <span className="strategy-namespace-badge">Node: {item.nodeID}</span>}
                    </div>
                  </div>
                  <div className="strategy-loaded-details">
                    <div className="detail-grid">
                      <div className="detail-item">
                        <span className="detail-label">Voluntary Ctx Switches</span>
                        <span className="detail-value">{formatMetricValue(item.voluntaryCtxSwitches)}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">Involuntary Ctx Switches</span>
                        <span className="detail-value">{formatMetricValue(item.involuntaryCtxSwitches)}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">CPU Time (ns)</span>
                        <span className="detail-value">{formatMetricValue(item.cpuTimeNs)}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">Wait Time (ns)</span>
                        <span className="detail-value">{formatMetricValue(item.waitTimeNs)}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">Run Count</span>
                        <span className="detail-value">{formatMetricValue(item.runCount)}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">CPU Migrations</span>
                        <span className="detail-value">{formatMetricValue(item.cpuMigrations)}</span>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Loaded Items */}
        <div className="loaded-strategies-section">
          <h3 className="section-title"><BarChart3 size={16} /> Saved Metrics Resources</h3>

          {loading && (
            <div className="empty-state">
              <span className="empty-icon"><Loader2 size={24} className="spin" /></span>
              <p>Loading...</p>
            </div>
          )}

          {!loading && error && (
            <div className="empty-state error">
              <span className="empty-icon"><XCircle size={24} /></span>
              <p>Error: {error}</p>
            </div>
          )}

          {!loading && !error && items.length === 0 && (
            <div className="empty-state">
              <span className="empty-icon"><Inbox size={24} /></span>
              <p>No PodSchedulingMetrics found.</p>
            </div>
          )}

          {items.map((item) => (
            <div key={item.id} className="strategy-loaded-item">
              <div className="strategy-loaded-header">
                <div className="strategy-loaded-title" onClick={() => toggleExpand(item.id)}>
                  <span className="expand-icon">{expandedItems[item.id] ? <ChevronDown size={16} /> : <ChevronRight size={16} />}</span>
                  <span className="strategy-id">PSM: {item.id.slice(-8)}...</span>
                  <span className={`strategy-namespace-badge ${item.enabled ? '' : 'disabled'}`}>
                    {item.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                </div>
                <div className="strategy-loaded-summary">
                  <span className="strategy-priority">Interval: {item.collectionIntervalSeconds}s</span>
                  {item.k8sNamespaces && item.k8sNamespaces.length > 0 && (
                    <span className="strategy-k8s-ns">NS: {item.k8sNamespaces.join(', ')}</span>
                  )}
                  <button className="secondary-btn-small" onClick={(e) => { e.stopPropagation(); beginEdit(item); }}
                    title="Edit" disabled={!isAuthenticated}>
                    <Pencil size={14} />
                  </button>
                  <button className="danger-btn-small" onClick={(e) => { e.stopPropagation(); handleDelete(item.id); }} title="Delete">
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>

              {expandedItems[item.id] && (
                <div className="strategy-loaded-details">
                  <div className="detail-grid">
                    <div className="detail-item">
                      <span className="detail-label">ID</span>
                      <span className="detail-value">{item.id}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Collection Interval</span>
                      <span className="detail-value">{item.collectionIntervalSeconds}s</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Enabled</span>
                      <span className="detail-value">{item.enabled ? 'Yes' : 'No'}</span>
                    </div>
                    {item.commandRegex && (
                      <div className="detail-item">
                        <span className="detail-label">Command Regex</span>
                        <span className="detail-value">{item.commandRegex}</span>
                      </div>
                    )}
                    {item.k8sNamespaces && item.k8sNamespaces.length > 0 && (
                      <div className="detail-item">
                        <span className="detail-label">K8s Namespaces</span>
                        <span className="detail-value">{item.k8sNamespaces.join(', ')}</span>
                      </div>
                    )}
                  </div>

                  {item.labelSelectors && item.labelSelectors.length > 0 && (
                    <div className="labels-section">
                      <span className="labels-title">Label Selectors</span>
                      <div className="labels-grid">
                        {item.labelSelectors.map((sel, idx) => (
                          <div key={idx} className="label-item">
                            <span className="label-key">{sel.key}</span>
                            <span className="label-value">{sel.value}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {item.metrics && (
                    <div className="labels-section">
                      <span className="labels-title">Collected Metrics</span>
                      <div className="labels-grid">
                        {item.metrics.voluntaryCtxSwitches && <span className="strategy-namespace-badge">Voluntary Ctx Switches</span>}
                        {item.metrics.involuntaryCtxSwitches && <span className="strategy-namespace-badge">Involuntary Ctx Switches</span>}
                        {item.metrics.cpuTimeNs && <span className="strategy-namespace-badge">CPU Time</span>}
                        {item.metrics.waitTimeNs && <span className="strategy-namespace-badge">Wait Time</span>}
                        {item.metrics.runCount && <span className="strategy-namespace-badge">Run Count</span>}
                        {item.metrics.cpuMigrations && <span className="strategy-namespace-badge">CPU Migrations</span>}
                      </div>
                    </div>
                  )}

                  {item.scaling && item.scaling.enabled && (
                    <div className="labels-section">
                      <span className="labels-title">KEDA Auto-Scaling</span>
                      <div className="detail-grid">
                        <div className="detail-item">
                          <span className="detail-label">Metric</span>
                          <span className="detail-value">{item.scaling.metricName}</span>
                        </div>
                        <div className="detail-item">
                          <span className="detail-label">Target Value</span>
                          <span className="detail-value">{item.scaling.targetValue}</span>
                        </div>
                        {item.scaling.scaleTargetRef && (
                          <div className="detail-item">
                            <span className="detail-label">Scale Target</span>
                            <span className="detail-value">{item.scaling.scaleTargetRef.kind}/{item.scaling.scaleTargetRef.name}</span>
                          </div>
                        )}
                        <div className="detail-item">
                          <span className="detail-label">Replicas</span>
                          <span className="detail-value">{item.scaling.minReplicaCount} - {item.scaling.maxReplicaCount}</span>
                        </div>
                        <div className="detail-item">
                          <span className="detail-label">Cooldown</span>
                          <span className="detail-value">{item.scaling.cooldownPeriod}s</span>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* Inline Edit Form */}
                  {editingId === item.id && editForm && (
                    <div className="strategy-edit-form">
                      <h4>Edit PodSchedulingMetrics</h4>
                      <div className="strategy-form">
                        <div className="full-width">
                          <label>K8s Namespaces (comma separated)</label>
                          <input type="text" value={editForm.k8sNamespaces}
                            onChange={(e) => setEditForm(prev => ({ ...prev, k8sNamespaces: e.target.value }))} />
                        </div>
                        <div>
                          <label>Command Regex</label>
                          <input type="text" value={editForm.commandRegex}
                            onChange={(e) => setEditForm(prev => ({ ...prev, commandRegex: e.target.value }))} />
                        </div>
                        <div>
                          <label>Collection Interval (s)</label>
                          <input type="number" min="1" max="3600" value={editForm.collectionIntervalSeconds}
                            onChange={(e) => setEditForm(prev => ({ ...prev, collectionIntervalSeconds: e.target.value }))} />
                        </div>
                        <div>
                          <label style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                            <input type="checkbox" checked={editForm.enabled}
                              onChange={() => setEditForm(prev => ({ ...prev, enabled: !prev.enabled }))} />
                            Enabled
                          </label>
                        </div>
                        {renderSelectors(editForm, setEditForm)}
                        {renderMetricsToggles(editForm.metrics, setEditForm)}
                        {renderScalingSection(editForm, setEditForm)}
                      </div>
                      <div className="strategies-actions" style={{ display: 'flex' }}>
                        <button className="danger-btn" onClick={cancelEdit}>Cancel</button>
                        <button className="success-btn auth-required" onClick={handleUpdate} disabled={!isAuthenticated}>
                          <Save size={16} /> Update
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
