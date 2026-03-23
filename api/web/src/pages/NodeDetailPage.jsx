import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import {
  ArrowLeft,
  RefreshCw,
  Server,
  ChevronDown,
  ChevronRight,
  Loader2,
  Inbox,
} from 'lucide-react';

function truncateText(text, maxLen) {
  if (!text) return '--';
  if (text.length <= maxLen) return text;
  return text.substring(0, maxLen - 3) + '...';
}

export default function NodeDetailPage() {
  const { nodeId } = useParams();
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();

  const [pods, setPods] = useState([]);
  const [nodeName, setNodeName] = useState('--');
  const [totalProcesses, setTotalProcesses] = useState(0);
  const [lastUpdated, setLastUpdated] = useState('--');
  const [loading, setLoading] = useState(false);
  const [expandedPods, setExpandedPods] = useState(new Set());
  const [autoRefresh, setAutoRefresh] = useState(false);
  const intervalRef = useRef(null);

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

  useEffect(() => {
    if (isAuthenticated) fetchPods();
  }, [isAuthenticated, fetchPods]);

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
