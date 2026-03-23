import React, { useState, useEffect, useCallback } from 'react';
import { useApp } from '../context/AppContext';
import SlidePanel from '../components/SlidePanel';
import {
  BarChart3,
  Plus,
  RefreshCw,
  Pencil,
  Trash2,
  Save,
  Loader2,
  Inbox,
  XCircle,
  X,
} from 'lucide-react';

function formatMetricValue(v) {
  return new Intl.NumberFormat().format(v || 0);
}

function blankForm() {
  return {
    id: null,
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

const metricFlags = [
  ['voluntaryCtxSwitches', 'Voluntary Ctx Switches'],
  ['involuntaryCtxSwitches', 'Involuntary Ctx Switches'],
  ['cpuTimeNs', 'CPU Time (ns)'],
  ['waitTimeNs', 'Wait Time (ns)'],
  ['runCount', 'Run Count'],
  ['cpuMigrations', 'CPU Migrations'],
];

export default function PodMetricsPage() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();

  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const [runtimeItems, setRuntimeItems] = useState([]);
  const [runtimeWarnings, setRuntimeWarnings] = useState([]);
  const [loadingRuntime, setLoadingRuntime] = useState(false);

  const [panelOpen, setPanelOpen] = useState(false);
  const [panelMode, setPanelMode] = useState('create');
  const [form, setForm] = useState(blankForm());

  /* ─── load config items ─── */
  const loadItems = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoading(true);
    setError('');
    try {
      const res = await makeAuthenticatedRequest('/api/v1/pod-scheduling-metrics');
      const data = await res.json();
      if (data.success) {
        setItems(data.data?.items || []);
      } else {
        setError(data.error || 'Failed');
        setItems([]);
      }
    } catch (err) {
      setError(err.message);
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest]);

  /* ─── load runtime metrics ─── */
  const loadRuntime = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoadingRuntime(true);
    try {
      const res = await makeAuthenticatedRequest('/api/v1/pod-scheduling-metrics/runtime');
      const data = await res.json();
      if (data.success) {
        setRuntimeItems(data.data?.items || []);
        setRuntimeWarnings(data.data?.warnings || []);
      } else {
        setRuntimeItems([]);
      }
    } catch {
      setRuntimeItems([]);
    } finally {
      setLoadingRuntime(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest]);

  const refreshAll = useCallback(() => {
    loadItems();
    loadRuntime();
  }, [loadItems, loadRuntime]);

  useEffect(() => {
    if (isAuthenticated) refreshAll();
  }, [isAuthenticated]);

  /* ─── form helpers ─── */
  const uf = (field, value) => setForm((f) => ({ ...f, [field]: value }));
  const updateSelector = (i, field, value) =>
    setForm((f) => {
      const s = [...f.selectors];
      s[i] = { ...s[i], [field]: value };
      return { ...f, selectors: s };
    });
  const addSelector = () =>
    setForm((f) => ({ ...f, selectors: [...f.selectors, { key: '', value: '' }] }));
  const removeSelector = (i) =>
    setForm((f) => {
      const s = f.selectors.filter((_, idx) => idx !== i);
      return { ...f, selectors: s.length ? s : [{ key: '', value: '' }] };
    });
  const toggleMetric = (key) =>
    setForm((f) => ({ ...f, metrics: { ...f.metrics, [key]: !f.metrics[key] } }));

  /* ─── open panel ─── */
  const openCreate = () => {
    setForm(blankForm());
    setPanelMode('create');
    setPanelOpen(true);
  };

  const openEdit = (item) => {
    const selectors = (item.labelSelectors || []).map((s) => ({
      key: s.key || '',
      value: s.value || '',
    }));
    setForm({
      id: item.id,
      selectors: selectors.length ? selectors : [{ key: '', value: '' }],
      k8sNamespaces: (item.k8sNamespaces || []).join(', '),
      commandRegex: item.commandRegex || '',
      collectionIntervalSeconds: item.collectionIntervalSeconds || 10,
      enabled: item.enabled !== undefined ? item.enabled : true,
      metrics: item.metrics || blankForm().metrics,
      scalingEnabled: item.scaling?.enabled || false,
      scalingMetricName: item.scaling?.metricName || '',
      scalingTargetValue: item.scaling?.targetValue || '',
      scalingTargetName: item.scaling?.scaleTargetRef?.name || '',
      scalingTargetKind: item.scaling?.scaleTargetRef?.kind || 'Deployment',
      scalingMinReplicas: item.scaling?.minReplicaCount || 1,
      scalingMaxReplicas: item.scaling?.maxReplicaCount || 10,
      scalingCooldown: item.scaling?.cooldownPeriod || 300,
    });
    setPanelMode('edit');
    setPanelOpen(true);
  };

  /* ─── save ─── */
  const handleSave = async () => {
    const labelSelectors = form.selectors
      .filter((s) => s.key.trim() && s.value.trim())
      .map((s) => ({ key: s.key.trim(), value: s.value.trim() }));
    if (labelSelectors.length === 0) {
      showToast('error', 'At least one label selector required');
      return;
    }
    const payload = {
      labelSelectors,
      commandRegex: form.commandRegex.trim() || undefined,
      collectionIntervalSeconds: parseInt(form.collectionIntervalSeconds, 10) || 10,
      enabled: form.enabled,
      metrics: form.metrics,
    };
    if (panelMode === 'edit') payload.id = form.id;
    const ns = form.k8sNamespaces.split(',').map((s) => s.trim()).filter(Boolean);
    if (ns.length) payload.k8sNamespaces = ns;
    if (form.scalingEnabled) {
      payload.scaling = {
        enabled: true,
        metricName: form.scalingMetricName,
        targetValue: form.scalingTargetValue,
        scaleTargetRef: {
          kind: form.scalingTargetKind || 'Deployment',
          name: form.scalingTargetName,
          apiVersion: 'apps/v1',
        },
        minReplicaCount: parseInt(form.scalingMinReplicas, 10) || 1,
        maxReplicaCount: parseInt(form.scalingMaxReplicas, 10) || 10,
        cooldownPeriod: parseInt(form.scalingCooldown, 10) || 300,
      };
    }
    try {
      const res = await makeAuthenticatedRequest('/api/v1/pod-scheduling-metrics', {
        method: panelMode === 'create' ? 'POST' : 'PUT',
        body: JSON.stringify(payload),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', panelMode === 'create' ? 'Created' : 'Updated');
        setPanelOpen(false);
        loadItems();
      } else {
        showToast('error', data.error || 'Failed');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  /* ─── delete ─── */
  const handleDelete = async (id) => {
    if (!window.confirm('Delete this metrics config?')) return;
    try {
      const res = await makeAuthenticatedRequest('/api/v1/pod-scheduling-metrics', {
        method: 'DELETE',
        body: JSON.stringify({ id }),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', 'Deleted');
        loadItems();
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
          <h1 className="page-title">Pod Metrics</h1>
          <p className="page-subtitle">Configure and monitor pod scheduling metrics collection</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-secondary btn-sm" onClick={refreshAll}>
            <RefreshCw size={14} />
            <span>Refresh</span>
          </button>
          <button className="btn btn-primary btn-sm" onClick={openCreate}>
            <Plus size={14} />
            <span>New Config</span>
          </button>
        </div>
      </div>

      {/* Runtime Metrics */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <BarChart3 size={16} />
            Latest Collected Metrics
          </h3>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {loadingRuntime ? (
            <div className="empty-state">
              <Loader2 size={20} className="spin" />
              <p>Loading...</p>
            </div>
          ) : runtimeItems.length === 0 ? (
            <div className="empty-state">
              <Inbox size={20} />
              <p>No runtime metrics collected yet</p>
            </div>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>NAMESPACE</th>
                  <th>POD</th>
                  <th>NODE</th>
                  <th>VOL CTX SW</th>
                  <th>INVOL CTX SW</th>
                  <th>CPU TIME</th>
                  <th>WAIT TIME</th>
                  <th>RUN COUNT</th>
                  <th>CPU MIGR</th>
                </tr>
              </thead>
              <tbody>
                {runtimeItems.map((item, i) => (
                  <tr key={i}>
                    <td>{item.namespace}</td>
                    <td>{item.podName}</td>
                    <td>{item.nodeID || '--'}</td>
                    <td style={{ fontFamily: 'monospace' }}>{formatMetricValue(item.voluntaryCtxSwitches)}</td>
                    <td style={{ fontFamily: 'monospace' }}>{formatMetricValue(item.involuntaryCtxSwitches)}</td>
                    <td style={{ fontFamily: 'monospace' }}>{formatMetricValue(item.cpuTimeNs)}</td>
                    <td style={{ fontFamily: 'monospace' }}>{formatMetricValue(item.waitTimeNs)}</td>
                    <td style={{ fontFamily: 'monospace' }}>{formatMetricValue(item.runCount)}</td>
                    <td style={{ fontFamily: 'monospace' }}>{formatMetricValue(item.cpuMigrations)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {runtimeWarnings.length > 0 && (
            <div style={{ padding: '8px 16px', fontSize: 12, color: 'var(--color-warning)' }}>
              Warnings: {runtimeWarnings.join('; ')}
            </div>
          )}
        </div>
      </div>

      {/* Saved Configs */}
      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-header">
          <h3 className="card-title">
            <BarChart3 size={16} />
            Metrics Configurations
          </h3>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {loading ? (
            <div className="empty-state">
              <Loader2 size={20} className="spin" />
              <p>Loading...</p>
            </div>
          ) : error ? (
            <div className="empty-state">
              <XCircle size={20} />
              <p>{error}</p>
            </div>
          ) : items.length === 0 ? (
            <div className="empty-state">
              <Inbox size={20} />
              <p>No configurations yet</p>
            </div>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>K8S NS</th>
                  <th>CMD REGEX</th>
                  <th>INTERVAL</th>
                  <th>STATUS</th>
                  <th>LABELS</th>
                  <th>SCALING</th>
                  <th>ACTIONS</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }} title={item.id}>
                      {item.id.slice(-8)}
                    </td>
                    <td>{item.k8sNamespaces?.join(', ') || '--'}</td>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{item.commandRegex || '--'}</td>
                    <td>{item.collectionIntervalSeconds}s</td>
                    <td>
                      <span className={`badge ${item.enabled ? 'badge-success' : 'badge-secondary'}`}>
                        {item.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </td>
                    <td>
                      {(item.labelSelectors || []).map((l, i) => (
                        <span key={i} className="badge badge-secondary" style={{ marginRight: 4, marginBottom: 2 }}>
                          {l.key}={l.value}
                        </span>
                      ))}
                    </td>
                    <td>
                      <span className={`badge ${item.scaling?.enabled ? 'badge-primary' : 'badge-secondary'}`}>
                        {item.scaling?.enabled ? 'On' : 'Off'}
                      </span>
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn btn-ghost btn-sm" onClick={() => openEdit(item)}>
                          <Pencil size={14} />
                        </button>
                        <button className="btn btn-ghost btn-sm" onClick={() => handleDelete(item.id)}>
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Slide Panel */}
      <SlidePanel
        open={panelOpen}
        onClose={() => setPanelOpen(false)}
        title={panelMode === 'create' ? 'New Metrics Config' : 'Edit Metrics Config'}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="form-group">
            <label className="form-label">K8s Namespaces (comma separated)</label>
            <input
              className="form-input"
              placeholder="default, kube-system"
              value={form.k8sNamespaces}
              onChange={(e) => uf('k8sNamespaces', e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Command Regex</label>
            <input
              className="form-input"
              placeholder="e.g., upf|amf"
              value={form.commandRegex}
              onChange={(e) => uf('commandRegex', e.target.value)}
            />
          </div>
          <div style={{ display: 'flex', gap: 12 }}>
            <div className="form-group" style={{ flex: 1 }}>
              <label className="form-label">Collection Interval (s)</label>
              <input
                className="form-input"
                type="number"
                min="1"
                value={form.collectionIntervalSeconds}
                onChange={(e) => uf('collectionIntervalSeconds', e.target.value)}
              />
            </div>
            <div className="form-group" style={{ flex: 1, display: 'flex', alignItems: 'flex-end' }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13, padding: '8px 0' }}>
                <input
                  type="checkbox"
                  checked={form.enabled}
                  onChange={() => uf('enabled', !form.enabled)}
                />
                Enabled
              </label>
            </div>
          </div>

          {/* Label selectors */}
          <div>
            <label className="form-label">Label Selectors</label>
            {form.selectors.map((sel, i) => (
              <div key={i} style={{ display: 'flex', gap: 8, marginBottom: 6 }}>
                <input
                  className="form-input"
                  placeholder="Key"
                  value={sel.key}
                  onChange={(e) => updateSelector(i, 'key', e.target.value)}
                />
                <input
                  className="form-input"
                  placeholder="Value"
                  value={sel.value}
                  onChange={(e) => updateSelector(i, 'value', e.target.value)}
                />
                <button className="btn btn-danger btn-sm" onClick={() => removeSelector(i)}>
                  <X size={14} />
                </button>
              </div>
            ))}
            <button className="btn btn-ghost btn-sm" onClick={addSelector}>
              + Add Selector
            </button>
          </div>

          {/* Metric toggles */}
          <div>
            <label className="form-label">Metrics to Collect</label>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6 }}>
              {metricFlags.map(([key, label]) => (
                <label key={key} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}>
                  <input
                    type="checkbox"
                    checked={form.metrics[key]}
                    onChange={() => toggleMetric(key)}
                  />
                  {label}
                </label>
              ))}
            </div>
          </div>

          {/* Scaling */}
          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13, fontWeight: 500 }}>
              <input
                type="checkbox"
                checked={form.scalingEnabled}
                onChange={() => uf('scalingEnabled', !form.scalingEnabled)}
              />
              Enable KEDA Auto-Scaling
            </label>
            {form.scalingEnabled && (
              <div style={{ marginTop: 12, display: 'flex', flexDirection: 'column', gap: 12 }}>
                <div className="form-group">
                  <label className="form-label">Metric Name</label>
                  <input
                    className="form-input"
                    placeholder="e.g., gthulhu_pod_voluntary_ctx_switches_total"
                    value={form.scalingMetricName}
                    onChange={(e) => uf('scalingMetricName', e.target.value)}
                  />
                </div>
                <div style={{ display: 'flex', gap: 12 }}>
                  <div className="form-group" style={{ flex: 1 }}>
                    <label className="form-label">Target Value</label>
                    <input
                      className="form-input"
                      value={form.scalingTargetValue}
                      onChange={(e) => uf('scalingTargetValue', e.target.value)}
                    />
                  </div>
                  <div className="form-group" style={{ flex: 1 }}>
                    <label className="form-label">Cooldown (s)</label>
                    <input
                      className="form-input"
                      type="number"
                      value={form.scalingCooldown}
                      onChange={(e) => uf('scalingCooldown', e.target.value)}
                    />
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 12 }}>
                  <div className="form-group" style={{ flex: 1 }}>
                    <label className="form-label">Scale Target Name</label>
                    <input
                      className="form-input"
                      value={form.scalingTargetName}
                      onChange={(e) => uf('scalingTargetName', e.target.value)}
                    />
                  </div>
                  <div className="form-group" style={{ flex: 1 }}>
                    <label className="form-label">Scale Target Kind</label>
                    <input
                      className="form-input"
                      value={form.scalingTargetKind}
                      onChange={(e) => uf('scalingTargetKind', e.target.value)}
                    />
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 12 }}>
                  <div className="form-group" style={{ flex: 1 }}>
                    <label className="form-label">Min Replicas</label>
                    <input
                      className="form-input"
                      type="number"
                      min="0"
                      value={form.scalingMinReplicas}
                      onChange={(e) => uf('scalingMinReplicas', e.target.value)}
                    />
                  </div>
                  <div className="form-group" style={{ flex: 1 }}>
                    <label className="form-label">Max Replicas</label>
                    <input
                      className="form-input"
                      type="number"
                      min="1"
                      value={form.scalingMaxReplicas}
                      onChange={(e) => uf('scalingMaxReplicas', e.target.value)}
                    />
                  </div>
                </div>
              </div>
            )}
          </div>

          <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
            <button className="btn btn-secondary" onClick={() => setPanelOpen(false)} style={{ flex: 1 }}>
              Cancel
            </button>
            <button className="btn btn-primary" onClick={handleSave} style={{ flex: 1 }}>
              <Save size={14} />
              <span>{panelMode === 'create' ? 'Create' : 'Update'}</span>
            </button>
          </div>
        </div>
      </SlidePanel>
    </div>
  );
}
