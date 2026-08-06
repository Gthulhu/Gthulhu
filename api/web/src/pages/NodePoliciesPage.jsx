import React, { useCallback, useEffect, useState } from 'react';
import { Cpu, Inbox, Loader2, Pencil, Plus, RefreshCw, Save, Trash2, X } from 'lucide-react';
import { useApp } from '../context/AppContext';
import SlidePanel from '../components/SlidePanel';

const emptyPair = () => ({ key: '', value: '' });
const blankForm = () => ({
  id: '',
  nodeNames: '',
  nodeSelectors: [emptyPair()],
  draSelectors: [{ deviceClass: '', attributes: [emptyPair()] }],
  commandRegex: '',
  priority: 10,
  executionTime: 20000000,
});

function PairRows({ label, rows, onChange, onAdd, onRemove }) {
  return (
    <div>
      <label className="form-label">{label}</label>
      {rows.map((row, index) => (
        <div key={index} style={{ display: 'flex', gap: 8, marginBottom: 6 }}>
          <input
            className="form-input"
            placeholder="Key"
            value={row.key}
            onChange={(event) => onChange(index, 'key', event.target.value)}
          />
          <input
            className="form-input"
            placeholder="Value"
            value={row.value}
            onChange={(event) => onChange(index, 'value', event.target.value)}
          />
          <button className="btn btn-danger btn-sm" type="button" onClick={() => onRemove(index)}>
            <X size={14} />
          </button>
        </div>
      ))}
      <button className="btn btn-ghost btn-sm" type="button" onClick={onAdd}>+ Add Selector</button>
    </div>
  );
}

export default function NodePoliciesPage() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();
  const [policies, setPolicies] = useState([]);
  const [intents, setIntents] = useState([]);
  const [loading, setLoading] = useState(false);
  const [panelOpen, setPanelOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState(blankForm());

  const fetchData = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoading(true);
    try {
      const [policyResponse, intentResponse] = await Promise.all([
        makeAuthenticatedRequest('/api/v1/node-scheduling-policies/self'),
        makeAuthenticatedRequest('/api/v1/node-scheduling-intents/self'),
      ]);
      const [policyData, intentData] = await Promise.all([policyResponse.json(), intentResponse.json()]);
      if (!policyData.success) throw new Error(policyData.error || 'Failed to load node policies');
      if (!intentData.success) throw new Error(intentData.error || 'Failed to load node intents');
      setPolicies(policyData.data?.policies || []);
      setIntents(intentData.data?.intents || []);
    } catch (error) {
      showToast('error', error.message);
      setPolicies([]);
      setIntents([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const openCreate = () => {
    setForm(blankForm());
    setEditing(false);
    setPanelOpen(true);
  };

  const openEdit = (policy) => {
    setForm({
      id: policy.id,
      nodeNames: (policy.nodeNames || []).join(', '),
      nodeSelectors: policy.nodeSelectors?.length ? policy.nodeSelectors : [emptyPair()],
      draSelectors: policy.draSelectors?.length
        ? policy.draSelectors.map((selector) => ({
            deviceClass: selector.deviceClass || '',
            attributes: selector.attributes?.length ? selector.attributes : [emptyPair()],
          }))
        : [{ deviceClass: '', attributes: [emptyPair()] }],
      commandRegex: policy.commandRegex || '',
      priority: policy.priority ?? 10,
      executionTime: policy.executionTime ?? 20000000,
    setEditing(true);
    setPanelOpen(true);
  };

  const updatePair = (field, index, key, value) => {
    setForm((current) => {
      const rows = [...current[field]];
      rows[index] = { ...rows[index], [key]: value };
      return { ...current, [field]: rows };
    });
  };

  const removePair = (field, index) => {
    setForm((current) => {
      const rows = current[field].filter((_, rowIndex) => rowIndex !== index);
      return { ...current, [field]: rows.length ? rows : [emptyPair()] };
    });
  };

  const updateDra = (index, value) => {
    setForm((current) => {
      const selectors = [...current.draSelectors];
      selectors[index] = { ...selectors[index], deviceClass: value };
      return { ...current, draSelectors: selectors };
    });
  };

  const updateDraAttribute = (draIndex, attrIndex, key, value) => {
    setForm((current) => {
      const selectors = [...current.draSelectors];
      const attributes = [...selectors[draIndex].attributes];
      attributes[attrIndex] = { ...attributes[attrIndex], [key]: value };
      selectors[draIndex] = { ...selectors[draIndex], attributes };
      return { ...current, draSelectors: selectors };
    });
  };

  const removeDraAttribute = (draIndex, attrIndex) => {
    setForm((current) => {
      const selectors = [...current.draSelectors];
      const attributes = selectors[draIndex].attributes.filter((_, index) => index !== attrIndex);
      selectors[draIndex] = { ...selectors[draIndex], attributes: attributes.length ? attributes : [emptyPair()] };
      return { ...current, draSelectors: selectors };
    });
  };

  const handleSave = async () => {
    const nodeSelectors = form.nodeSelectors.filter((item) => item.key.trim() && item.value.trim());
    const draSelectors = form.draSelectors
      .filter((item) => item.deviceClass.trim())
      .map((item) => ({
        deviceClass: item.deviceClass.trim(),
        attributes: item.attributes.filter((attr) => attr.key.trim() && attr.value.trim()),
      }));
    const payload = {
      nodeNames: form.nodeNames.split(',').map((name) => name.trim()).filter(Boolean),
      nodeSelectors,
      draSelectors,
      commandRegex: form.commandRegex.trim(),
      priority: Number(form.priority),
      executionTime: Number(form.executionTime),
    };
    if (editing) payload.policyId = form.id;

    try {
      const response = await makeAuthenticatedRequest('/api/v1/node-scheduling-policies', {
        method: editing ? 'PUT' : 'POST',
        body: JSON.stringify(payload),
      });
      const data = await response.json();
      if (!data.success) throw new Error(data.error || data.message || 'Failed to save node policy');
      showToast('success', editing ? 'Node policy updated' : 'Node policy created');
      setPanelOpen(false);
      fetchData();
    } catch (error) {
      showToast('error', error.message);
    }
  };

  const handleDelete = async (policyId) => {
    if (!window.confirm('Delete this node policy and its intents?')) return;
    try {
      const response = await makeAuthenticatedRequest('/api/v1/node-scheduling-policies', {
        method: 'DELETE',
        body: JSON.stringify({ policyId }),
      });
      const data = await response.json();
      if (!data.success) throw new Error(data.error || 'Failed to delete node policy');
      showToast('success', 'Node policy deleted');
      fetchData();
    } catch (error) {
      showToast('error', error.message);
    }
  };

  const stateLabel = { 0: 'Pending', 1: 'Sent', 2: 'Applied', 3: 'Failed' };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Node Policies</h1>
          <p className="page-subtitle">Schedule host processes on selected nodes</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-secondary btn-sm" onClick={fetchData} disabled={loading}>
            <RefreshCw size={14} className={loading ? 'spin' : ''} /> Refresh
          </button>
          <button className="btn btn-primary btn-sm" onClick={openCreate}>
            <Plus size={14} /> New Node Policy
          </button>
        </div>
      </div>

      <div className="stat-cards">
        <div className="stat-card"><div className="stat-card-label">Policies</div><div className="stat-card-value">{policies.length}</div></div>
        <div className="stat-card"><div className="stat-card-label">Node Intents</div><div className="stat-card-value">{intents.length}</div></div>
      </div>

      <div className="card">
        <div className="card-header"><h3 className="card-title"><Cpu size={16} /> Saved Node Policies</h3></div>
        <div className="card-body" style={{ padding: 0 }}>
          {loading ? (
            <div className="empty-state"><Loader2 size={20} className="spin" /><p>Loading...</p></div>
          ) : policies.length === 0 ? (
            <div className="empty-state"><Inbox size={20} /><p>No node policies yet</p></div>
          ) : (
            <table className="data-table">
              <thead><tr><th>ID</th><th>NODES</th><th>NODE LABELS</th><th>DRA</th><th>COMMAND</th><th>PRIORITY</th><th>EXEC TIME</th><th>ACTIONS</th></tr></thead>
              <tbody>
                {policies.map((policy) => (
                  <tr key={policy.id}>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{policy.id?.slice(-8)}</td>
                    <td>{policy.nodeNames?.join(', ') || '--'}</td>
                    <td>{policy.nodeSelectors?.map((item) => `${item.key}=${item.value}`).join(', ') || '--'}</td>
                    <td>{policy.draSelectors?.map((item) => item.deviceClass).join(', ') || '--'}</td>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{policy.commandRegex || '--'}</td>
                    <td>{policy.priority}</td>
                    <td>{policy.executionTime} ns</td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn btn-ghost btn-sm" onClick={() => openEdit(policy)}><Pencil size={14} /></button>
                        <button className="btn btn-ghost btn-sm" onClick={() => handleDelete(policy.id)}><Trash2 size={14} /></button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-header"><h3 className="card-title">Node Scheduling Intents</h3></div>
        <div className="card-body" style={{ padding: 0 }}>
          {intents.length === 0 ? (
            <div className="empty-state"><Inbox size={20} /><p>No node intents generated yet</p></div>
          ) : (
            <table className="data-table">
              <thead><tr><th>ID</th><th>POLICY</th><th>NODE</th><th>COMMAND</th><th>PRIORITY</th><th>EXEC TIME</th><th>STATE</th></tr></thead>
              <tbody>
                {intents.map((intent) => (
                  <tr key={intent.id}>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{intent.id?.slice(-8)}</td>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{intent.policyId?.slice(-8)}</td>
                    <td>{intent.nodeId}</td>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{intent.commandRegex || '--'}</td>
                    <td>{intent.priority}</td>
                    <td>{intent.executionTime} ns</td>
                    <td><span className="badge badge-secondary">{stateLabel[intent.state] || `Unknown (${intent.state})`}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      <SlidePanel open={panelOpen} onClose={() => setPanelOpen(false)} title={editing ? 'Edit Node Policy' : 'New Node Policy'}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="form-group">
            <label className="form-label">Node Names (comma separated)</label>
            <input className="form-input" placeholder="worker-1, worker-2" value={form.nodeNames} onChange={(event) => setForm({ ...form, nodeNames: event.target.value })} />
          </div>
          <PairRows
            label="Node Label Selectors"
            rows={form.nodeSelectors}
            onChange={(index, key, value) => updatePair('nodeSelectors', index, key, value)}
            onAdd={() => setForm({ ...form, nodeSelectors: [...form.nodeSelectors, emptyPair()] })}
            onRemove={(index) => removePair('nodeSelectors', index)}
          />
          <div>
            <label className="form-label">DRA Selectors</label>
            {form.draSelectors.map((selector, draIndex) => (
              <div key={draIndex} className="card" style={{ marginBottom: 8, padding: 12 }}>
                <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
                  <input className="form-input" placeholder="Device class" value={selector.deviceClass} onChange={(event) => updateDra(draIndex, event.target.value)} />
                  <button className="btn btn-danger btn-sm" type="button" onClick={() => setForm({ ...form, draSelectors: form.draSelectors.filter((_, index) => index !== draIndex) })}><X size={14} /></button>
                </div>
                <PairRows
                  label="Device Attributes"
                  rows={selector.attributes}
                  onChange={(attrIndex, key, value) => updateDraAttribute(draIndex, attrIndex, key, value)}
                  onAdd={() => setForm((current) => {
                    const selectors = [...current.draSelectors];
                    selectors[draIndex] = { ...selectors[draIndex], attributes: [...selectors[draIndex].attributes, emptyPair()] };
                    return { ...current, draSelectors: selectors };
                  })}
                  onRemove={(attrIndex) => removeDraAttribute(draIndex, attrIndex)}
                />
              </div>
            ))}
            <button className="btn btn-ghost btn-sm" type="button" onClick={() => setForm({ ...form, draSelectors: [...form.draSelectors, { deviceClass: '', attributes: [emptyPair()] }] })}>+ Add DRA Selector</button>
          </div>
          <div className="form-group">
            <label className="form-label">Command Regex (/proc/PID/comm)</label>
            <input className="form-input" placeholder="^kworker.*" value={form.commandRegex} onChange={(event) => setForm({ ...form, commandRegex: event.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">Priority</label>
            <input className="form-input" type="number" min="0" value={form.priority} onChange={(event) => setForm({ ...form, priority: event.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">Execution Time (ns)</label>
            <input className="form-input" type="number" min="0" value={form.executionTime} onChange={(event) => setForm({ ...form, executionTime: event.target.value })} />
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-secondary" style={{ flex: 1 }} onClick={() => setPanelOpen(false)}>Cancel</button>
            <button className="btn btn-primary" style={{ flex: 1 }} onClick={handleSave}><Save size={14} /> {editing ? 'Update' : 'Create'}</button>
          </div>
        </div>
      </SlidePanel>
    </div>
  );
}
