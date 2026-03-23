import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useApp } from '../context/AppContext';
import {
  ClipboardList,
  RefreshCw,
  Loader2,
  Inbox,
  Server,
} from 'lucide-react';

export default function IntentsPage() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();
  const [intents, setIntents] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [selectedNode, setSelectedNode] = useState(null);

  const fetchIntents = useCallback(async () => {
    if (!isAuthenticated) return;
    setLoading(true);
    setError('');
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
        setError(data.error || 'Failed');
        setIntents([]);
      }
    } catch (err) {
      setError(err.message);
      setIntents([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, selectedNode]);

  useEffect(() => {
    if (isAuthenticated) fetchIntents();
  }, [isAuthenticated]);

  useEffect(() => {
    const h = () => fetchIntents();
    window.addEventListener('refreshIntents', h);
    return () => window.removeEventListener('refreshIntents', h);
  }, [fetchIntents]);

  const intentsByNode = useMemo(() => {
    const grouped = {};
    intents.forEach((i) => {
      const n = i.NodeID || 'Unknown';
      (grouped[n] = grouped[n] || []).push(i);
    });
    return grouped;
  }, [intents]);
  const nodes = useMemo(() => Object.keys(intentsByNode), [intentsByNode]);
  const filtered = useMemo(
    () => (selectedNode ? intentsByNode[selectedNode] || [] : []),
    [intentsByNode, selectedNode]
  );

  const stateLabel = { 0: 'Pending', 1: 'Active', 2: 'Applied', 3: 'Failed' };
  const stateBadge = { 0: 'badge-warning', 1: 'badge-primary', 2: 'badge-success', 3: 'badge-danger' };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Intents</h1>
          <p className="page-subtitle">View automatically generated schedule intents across nodes</p>
        </div>
        <button className="btn btn-secondary btn-sm" onClick={fetchIntents}>
          <RefreshCw size={14} />
          <span>Refresh</span>
        </button>
      </div>

      {/* Stat cards */}
      <div className="stat-cards">
        <div className="stat-card">
          <div className="stat-card-label">Total Intents</div>
          <div className="stat-card-value">{intents.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Nodes</div>
          <div className="stat-card-value">{nodes.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Applied</div>
          <div className="stat-card-value">{intents.filter((i) => i.State === 2).length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Failed</div>
          <div className="stat-card-value">{intents.filter((i) => i.State === 3).length}</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <ClipboardList size={16} />
            Schedule Intents
          </h3>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {/* Node tabs */}
          {nodes.length > 0 && (
            <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--color-border)', display: 'flex', gap: 6, flexWrap: 'wrap', alignItems: 'center' }}>
              <Server size={14} style={{ color: 'var(--color-text-secondary)' }} />
              {nodes.map((n) => (
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

          {loading ? (
            <div className="empty-state">
              <Loader2 size={20} className="spin" />
              <p>Loading intents...</p>
            </div>
          ) : error ? (
            <div className="empty-state">
              <p>{error}</p>
            </div>
          ) : intents.length === 0 ? (
            <div className="empty-state">
              <Inbox size={20} />
              <p>No intents found. Intents are automatically generated from strategies.</p>
            </div>
          ) : filtered.length === 0 ? (
            <div className="empty-state">
              <p>Select a node above</p>
            </div>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>INTENT ID</th>
                  <th>STRATEGY ID</th>
                  <th>POD</th>
                  <th>NAMESPACE</th>
                  <th>PRIORITY</th>
                  <th>EXEC TIME</th>
                  <th>CMD REGEX</th>
                  <th>STATE</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((intent) => (
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
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{intent.CommandRegex || '--'}</td>
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
    </div>
  );
}
