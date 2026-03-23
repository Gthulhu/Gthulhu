import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useApp } from '../context/AppContext';
import SlidePanel from '../components/SlidePanel';
import {
  Target,
  Plus,
  RefreshCw,
  Trash2,
  Pencil,
  Save,
  Loader2,
  Inbox,
  XCircle,
  X,
} from 'lucide-react';

/* ─── helpers ─── */
function SelectorRows({ selectors, onChange, onAdd, onRemove }) {
  return (
    <div>
      <label className="form-label">Label Selectors</label>
      {selectors.map((sel, i) => (
        <div key={i} className="selector-row" style={{ display: 'flex', gap: 8, marginBottom: 6 }}>
          <input
            className="form-input"
            placeholder="Key"
            value={sel.key}
            onChange={(e) => onChange(i, 'key', e.target.value)}
          />
          <input
            className="form-input"
            placeholder="Value"
            value={sel.value}
            onChange={(e) => onChange(i, 'value', e.target.value)}
          />
          <button className="btn btn-danger btn-sm" onClick={() => onRemove(i)}>
            <X size={14} />
          </button>
        </div>
      ))}
      <button className="btn btn-ghost btn-sm" onClick={onAdd} style={{ marginTop: 4 }}>
        + Add Selector
      </button>
    </div>
  );
}

function blankForm() {
  return {
    id: null,
    strategyNamespace: '',
    priority: 10,
    executionTime: 20000000,
    commandRegex: '',
    k8sNamespace: '',
    selectors: [{ key: '', value: '' }],
  };
}

export default function StrategiesPage() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();

  /* ─── strategies state ─── */
  const [strategies, setStrategies] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  /* ─── slide panel ─── */
  const [panelOpen, setPanelOpen] = useState(false);
  const [panelMode, setPanelMode] = useState('create'); // create | edit
  const [form, setForm] = useState(blankForm());

  /* ─── intents ─── */
  const [intents, setIntents] = useState([]);
  const [loadingIntents, setLoadingIntents] = useState(false);
  const [selectedNode, setSelectedNode] = useState(null);

  /* ─── fetch strategies ─── */
  const fetchStrategies = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoading(true);
    setError('');
    try {
      const res = await makeAuthenticatedRequest('/api/v1/strategies/self');
      const data = await res.json();
      if (data.success) {
        setStrategies(data.data?.strategies || []);
      } else {
        setError(data.error || 'Failed');
        setStrategies([]);
      }
    } catch (err) {
      setError(err.message);
      setStrategies([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest]);

  /* ─── fetch intents ─── */
  const fetchIntents = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoadingIntents(true);
    try {
      const res = await makeAuthenticatedRequest('/api/v1/intents/self');
      const data = await res.json();
      if (data.success) {
        const loaded = data.data?.intents || [];
        setIntents(loaded);
        const nodes = [...new Set(loaded.map((i) => i.NodeID))];
        if (!selectedNode || !nodes.includes(selectedNode)) {
          setSelectedNode(nodes[0] || null);
        }
      } else {
        setIntents([]);
      }
    } catch {
      setIntents([]);
    } finally {
      setLoadingIntents(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, selectedNode]);

  useEffect(() => {
    if (isAuthenticated) {
      fetchStrategies();
      fetchIntents();
    }
  }, [isAuthenticated]);

  useEffect(() => {
    const h1 = () => fetchStrategies();
    const h2 = () => fetchIntents();
    window.addEventListener('refreshStrategies', h1);
    window.addEventListener('refreshIntents', h2);
    return () => {
      window.removeEventListener('refreshStrategies', h1);
      window.removeEventListener('refreshIntents', h2);
    };
  }, [fetchStrategies, fetchIntents]);

  /* ─── intents grouping ─── */
  const intentsByNode = useMemo(() => {
    const grouped = {};
    intents.forEach((i) => {
      const n = i.NodeID || 'Unknown';
      (grouped[n] = grouped[n] || []).push(i);
    });
    return grouped;
  }, [intents]);
  const intentNodes = useMemo(() => Object.keys(intentsByNode), [intentsByNode]);
  const filteredIntents = useMemo(
    () => (selectedNode ? intentsByNode[selectedNode] || [] : []),
    [intentsByNode, selectedNode]
  );

  /* ─── form helpers ─── */
  const updateFormField = (field, value) => setForm((f) => ({ ...f, [field]: value }));
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

  /* ─── open panel ─── */
  const openCreate = () => {
    setForm(blankForm());
    setPanelMode('create');
    setPanelOpen(true);
  };
  const openEdit = (strategy) => {
    const selectors = (strategy.LabelSelectors || []).map((s) => ({
      key: s.key || '',
      value: s.value || '',
    }));
    setForm({
      id: strategy.ID,
      strategyNamespace: strategy.StrategyNamespace || '',
      priority: strategy.Priority || 0,
      executionTime: strategy.ExecutionTime || 0,
      commandRegex: strategy.CommandRegex || '',
      k8sNamespace: strategy.K8sNamespace ? strategy.K8sNamespace.join(', ') : '',
      selectors: selectors.length ? selectors : [{ key: '', value: '' }],
    });
    setPanelMode('edit');
    setPanelOpen(true);
  };

  /* ─── save ─── */
  const handleSave = async () => {
    const payload = {};
    if (panelMode === 'edit') payload.strategyId = form.id;
    if (form.strategyNamespace.trim()) payload.strategyNamespace = form.strategyNamespace.trim();
    if (form.priority !== '') payload.priority = parseInt(form.priority, 10);
    if (form.executionTime !== '') payload.executionTime = parseInt(form.executionTime, 10);
    if (form.commandRegex.trim()) payload.commandRegex = form.commandRegex.trim();
    const ns = form.k8sNamespace
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    if (ns.length) payload.k8sNamespace = ns;
    const selectors = form.selectors
      .filter((s) => s.key.trim() && s.value.trim())
      .map((s) => ({ key: s.key.trim(), value: s.value.trim() }));
    if (selectors.length) payload.labelSelectors = selectors;

    try {
      const res = await makeAuthenticatedRequest('/api/v1/strategies', {
        method: panelMode === 'create' ? 'POST' : 'PUT',
        body: JSON.stringify(payload),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', panelMode === 'create' ? 'Strategy created' : 'Strategy updated');
        setPanelOpen(false);
        fetchStrategies();
        fetchIntents();
      } else {
        showToast('error', data.error || data.message || 'Failed');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  /* ─── delete ─── */
  const handleDelete = async (strategyId) => {
    if (!window.confirm('Delete this strategy and its intents?')) return;
    try {
      const res = await makeAuthenticatedRequest('/api/v1/strategies', {
        method: 'DELETE',
        body: JSON.stringify({ strategyId }),
      });
      const data = await res.json();
      if (data.success) {
        showToast('success', 'Strategy deleted');
        fetchStrategies();
        fetchIntents();
      } else {
        showToast('error', data.error || 'Failed');
      }
    } catch (err) {
      showToast('error', err.message);
    }
  };

  const stateLabel = { 0: 'Pending', 1: 'Active', 2: 'Applied', 3: 'Failed' };
  const stateBadge = { 0: 'badge-warning', 1: 'badge-primary', 2: 'badge-success', 3: 'badge-danger' };

  return (
    <div>
      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">Strategies</h1>
          <p className="page-subtitle">Manage scheduling strategies and view generated intents</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-secondary btn-sm" onClick={() => { fetchStrategies(); fetchIntents(); }}>
            <RefreshCw size={14} />
            <span>Refresh</span>
          </button>
          <button className="btn btn-primary btn-sm" onClick={openCreate}>
            <Plus size={14} />
            <span>New Strategy</span>
          </button>
        </div>
      </div>

      {/* Stat cards */}
      <div className="stat-cards">
        <div className="stat-card">
          <div className="stat-card-label">Strategies</div>
          <div className="stat-card-value">{strategies.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Total Intents</div>
          <div className="stat-card-value">{intents.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Active Intents</div>
          <div className="stat-card-value">{intents.filter((i) => i.State === 1).length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Failed Intents</div>
          <div className="stat-card-value">{intents.filter((i) => i.State === 3).length}</div>
        </div>
      </div>

      {/* Strategies Table */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <Target size={16} />
            Saved Strategies
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
          ) : strategies.length === 0 ? (
            <div className="empty-state">
              <Inbox size={20} />
              <p>No strategies yet</p>
            </div>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>NAMESPACE</th>
                  <th>PRIORITY</th>
                  <th>EXEC TIME</th>
                  <th>CMD REGEX</th>
                  <th>K8S NS</th>
                  <th>LABELS</th>
                  <th>ACTIONS</th>
                </tr>
              </thead>
              <tbody>
                {strategies.map((s) => (
                  <tr key={s.ID}>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }} title={s.ID}>
                      {s.ID.slice(-8)}
                    </td>
                    <td>{s.StrategyNamespace || '--'}</td>
                    <td>{s.Priority}</td>
                    <td>{s.ExecutionTime} ns</td>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{s.CommandRegex || '--'}</td>
                    <td>{s.K8sNamespace?.join(', ') || '--'}</td>
                    <td>
                      {(s.LabelSelectors || []).map((l, i) => (
                        <span key={i} className="badge badge-secondary" style={{ marginRight: 4, marginBottom: 2 }}>
                          {l.key}={l.value}
                        </span>
                      ))}
                      {(!s.LabelSelectors || s.LabelSelectors.length === 0) && '--'}
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn btn-ghost btn-sm" onClick={() => openEdit(s)}>
                          <Pencil size={14} />
                        </button>
                        <button className="btn btn-ghost btn-sm" onClick={() => handleDelete(s.ID)}>
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

      {/* Intents section */}
      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-header">
          <h3 className="card-title">Schedule Intents</h3>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {/* Node tabs */}
          {intentNodes.length > 0 && (
            <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--color-border)', display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              {intentNodes.map((n) => (
                <button
                  key={n}
                  className={`btn btn-sm ${selectedNode === n ? 'btn-primary' : 'btn-secondary'}`}
                  onClick={() => setSelectedNode(n)}
                >
                  {n} ({intentsByNode[n].length})
                </button>
              ))}
            </div>
          )}

          {loadingIntents ? (
            <div className="empty-state">
              <Loader2 size={20} className="spin" />
              <p>Loading intents...</p>
            </div>
          ) : intents.length === 0 ? (
            <div className="empty-state">
              <Inbox size={20} />
              <p>No intents generated yet</p>
            </div>
          ) : filteredIntents.length === 0 ? (
            <div className="empty-state">
              <p>Select a node above</p>
            </div>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>STRATEGY</th>
                  <th>POD</th>
                  <th>NAMESPACE</th>
                  <th>PRIORITY</th>
                  <th>EXEC TIME</th>
                  <th>STATE</th>
                </tr>
              </thead>
              <tbody>
                {filteredIntents.map((intent) => (
                  <tr key={intent.ID}>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }} title={intent.ID}>
                      {intent.ID.slice(-8)}
                    </td>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }} title={intent.StrategyID}>
                      {intent.StrategyID?.slice(-8) || '--'}
                    </td>
                    <td>{intent.PodName || intent.PodID || '--'}</td>
                    <td>{intent.K8sNamespace || '--'}</td>
                    <td>{intent.Priority}</td>
                    <td>{intent.ExecutionTime} ns</td>
                    <td>
                      <span className={`badge ${stateBadge[intent.State] || 'badge-secondary'}`}>
                        {stateLabel[intent.State] || `Unknown(${intent.State})`}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Slide Panel for Create/Edit */}
      <SlidePanel
        open={panelOpen}
        onClose={() => setPanelOpen(false)}
        title={panelMode === 'create' ? 'New Strategy' : 'Edit Strategy'}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="form-group">
            <label className="form-label">Strategy Namespace</label>
            <input
              className="form-input"
              placeholder="e.g., default, trading, ml"
              value={form.strategyNamespace}
              onChange={(e) => updateFormField('strategyNamespace', e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Priority</label>
            <input
              className="form-input"
              type="number"
              min="0"
              max="20"
              value={form.priority}
              onChange={(e) => updateFormField('priority', e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Execution Time (ns)</label>
            <input
              className="form-input"
              type="number"
              value={form.executionTime}
              onChange={(e) => updateFormField('executionTime', e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Command Regex</label>
            <input
              className="form-input"
              placeholder="e.g., nr-gnb|ping"
              value={form.commandRegex}
              onChange={(e) => updateFormField('commandRegex', e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label">K8s Namespaces (comma separated)</label>
            <input
              className="form-input"
              placeholder="default, kube-system"
              value={form.k8sNamespace}
              onChange={(e) => updateFormField('k8sNamespace', e.target.value)}
            />
          </div>

          <SelectorRows
            selectors={form.selectors}
            onChange={updateSelector}
            onAdd={addSelector}
            onRemove={removeSelector}
          />

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
