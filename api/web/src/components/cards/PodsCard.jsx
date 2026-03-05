import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useApp } from '../../context/AppContext';
import { Container, RefreshCw, Maximize2, Minimize2, Server, ChevronDown } from 'lucide-react';

function truncateText(text, maxLen) {
  if (!text) return '--';
  if (text.length <= maxLen) return text;
  return text.substring(0, maxLen - 3) + '...';
}

function formatTimestamp(isoString) {
  try {
    const date = new Date(isoString);
    return date.toLocaleTimeString();
  } catch (e) {
    return isoString;
  }
}

export default function PodsCard() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();
  const [pods, setPods] = useState([]);
  const [nodes, setNodes] = useState([]);
  const [nodeName, setNodeName] = useState('--');
  const [nodeID, setNodeID] = useState('');
  const [totalProcesses, setTotalProcesses] = useState(0);
  const [lastUpdated, setLastUpdated] = useState('--');
  const [rawResult, setRawResult] = useState('Select a node to view pod-PID mappings...');
  const [resultClass, setResultClass] = useState('');
  const [expandedPods, setExpandedPods] = useState(new Set());
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadingNodes, setLoadingNodes] = useState(false);
  const intervalRef = useRef(null);

  // Fetch available nodes
  const fetchNodes = useCallback(async () => {
    if (!isAuthenticated) return;
    
    setLoadingNodes(true);
    try {
      const response = await makeAuthenticatedRequest('/api/v1/nodes');
      const data = await response.json();
      
      if (data.success && data.data && data.data.nodes) {
        setNodes(data.data.nodes);
        showToast('success', `Found ${data.data.nodes.length} node(s)`);
      } else {
        setNodes([]);
      }
    } catch (error) {
      console.error('Failed to fetch nodes:', error);
      setNodes([]);
    } finally {
      setLoadingNodes(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast]);

  // Fetch nodes when authenticated
  useEffect(() => {
    if (isAuthenticated) {
      fetchNodes();
    }
  }, [isAuthenticated, fetchNodes]);

  const getPodPids = useCallback(async (targetNodeID) => {
    const nodeToQuery = targetNodeID || nodeID;
    if (!isAuthenticated) return;
    if (!nodeToQuery) {
      showToast('warning', 'Please select a node');
      return;
    }
    
    setLoading(true);
    try {
      const response = await makeAuthenticatedRequest(`/api/v1/nodes/${encodeURIComponent(nodeToQuery)}/pods/pids`);
      const data = await response.json();
      
      if (data.success && data.data) {
        const loadedPods = data.data.pods || [];
        let processCount = 0;
        
        loadedPods.forEach(pod => {
          processCount += (pod.processes || []).length;
        });
        
        setPods(loadedPods);
        setNodeName(data.data.node_name || 'Unknown');
        setTotalProcesses(processCount);
        setLastUpdated(data.data.timestamp ? formatTimestamp(data.data.timestamp) : 'Now');
        setRawResult(JSON.stringify(data, null, 2));
        setResultClass('success');
        
        showToast('success', `Loaded ${loadedPods.length} pod(s) with ${processCount} process(es)`);
      } else {
        throw new Error(data.error || data.message || 'Failed to get pod-PID mappings');
      }
    } catch (error) {
      console.error('Pod-PID mapping error:', error);
      setPods([]);
      setNodeName('--');
      setTotalProcesses(0);
      setLastUpdated('--');
      setRawResult('Error: ' + error.message);
      setResultClass('error');
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast, nodeID]);

  const handleNodeChange = useCallback((e) => {
    const selectedNodeID = e.target.value;
    setNodeID(selectedNodeID);
    if (selectedNodeID) {
      getPodPids(selectedNodeID);
    } else {
      // Clear data when no node selected
      setPods([]);
      setNodeName('--');
      setTotalProcesses(0);
      setLastUpdated('--');
      setRawResult('Select a node to view pod-PID mappings...');
      setResultClass('');
    }
  }, [getPodPids]);

  useEffect(() => {
    const handleRefresh = () => getPodPids();
    window.addEventListener('refreshPodPids', handleRefresh);
    return () => window.removeEventListener('refreshPodPids', handleRefresh);
  }, [getPodPids]);

  useEffect(() => {
    if (autoRefresh && isAuthenticated && nodeID) {
      intervalRef.current = setInterval(() => getPodPids(), 5000);
      getPodPids();
      showToast('info', 'Pod-PID auto-refresh enabled (5s interval)');
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
  }, [autoRefresh, isAuthenticated, nodeID, getPodPids, showToast]);

  const togglePod = (podUid) => {
    setExpandedPods(prev => {
      const next = new Set(prev);
      if (next.has(podUid)) {
        next.delete(podUid);
      } else {
        next.add(podUid);
      }
      return next;
    });
  };

  const toggleAllPods = (expand) => {
    if (expand) {
      setExpandedPods(new Set(pods.map(p => p.pod_uid)));
    } else {
      setExpandedPods(new Set());
    }
    showToast('info', expand ? 'Expanded all pods' : 'Collapsed all pods');
  };

  const handleToggleAutoRefresh = () => {
    setAutoRefresh(prev => {
      if (prev) {
        showToast('info', 'Pod-PID auto-refresh disabled');
      }
      return !prev;
    });
  };

  return (
    <section className="card pods-card full-width">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><Container size={18} /></span>
          <h2>Pod-PID Mapping</h2>
        </div>
        <div className="card-actions">
          <button className="icon-btn" onClick={() => toggleAllPods(true)} title="Expand All">
            <Maximize2 size={16} />
          </button>
          <button className="icon-btn" onClick={() => toggleAllPods(false)} title="Collapse All">
            <Minimize2 size={16} />
          </button>
          <div className="auto-refresh-toggle">
            <input 
              type="checkbox" 
              id="podPidsAutoRefresh"
              checked={autoRefresh}
              onChange={handleToggleAutoRefresh}
              disabled={!nodeID}
            />
            <label htmlFor="podPidsAutoRefresh">Auto</label>
          </div>
        </div>
      </div>
      <div className="card-body">
        {/* Node Selector Section */}
        <div className="node-selector">
          <div className="node-input-group">
            <Server size={16} className="node-input-icon" />
            <div className="node-select-wrapper">
              <select
                className="node-select"
                value={nodeID}
                onChange={handleNodeChange}
                disabled={!isAuthenticated || loading || loadingNodes}
              >
                <option value="">
                  {loadingNodes ? 'Loading nodes...' : '-- Select a Node --'}
                </option>
                {nodes.map((node) => (
                  <option key={node.name} value={node.name}>
                    {node.name} ({node.status})
                  </option>
                ))}
              </select>
              <ChevronDown size={16} className="select-chevron" />
            </div>
            <button 
              className="btn btn-secondary btn-sm"
              onClick={fetchNodes}
              disabled={!isAuthenticated || loadingNodes}
              title="Refresh node list"
            >
              {loadingNodes ? <RefreshCw size={14} className="spin" /> : <RefreshCw size={14} />}
            </button>
            <button 
              className="btn btn-primary btn-sm"
              onClick={() => getPodPids()}
              disabled={!isAuthenticated || loading || !nodeID}
              title="Refresh Pod-PID mappings"
            >
              {loading ? <RefreshCw size={14} className="spin" /> : <RefreshCw size={14} />}
              <span>Load</span>
            </button>
          </div>
        </div>

        <div className="pods-summary" id="podsSummary">
          <div className="summary-item">
            <span className="summary-value" id="nodeNameDisplay">{nodeName}</span>
            <span className="summary-label">Node Name</span>
          </div>
          <div className="summary-item">
            <span className="summary-value" id="podsCount">{pods.length || '--'}</span>
            <span className="summary-label">Pods</span>
          </div>
          <div className="summary-item">
            <span className="summary-value" id="processesCount">{totalProcesses || '--'}</span>
            <span className="summary-label">Processes</span>
          </div>
          <div className="summary-item">
            <span className="summary-value" id="lastUpdated">{lastUpdated}</span>
            <span className="summary-label">Last Updated</span>
          </div>
        </div>
        <div className="pods-accordion" id="podsAccordion">
          {pods.length === 0 ? (
            <div className="pods-empty-state">
              {!isAuthenticated 
                ? 'Authenticate to view pod-PID mappings...' 
                : !nodeID 
                  ? 'Select a node above to view pod-PID mappings'
                  : 'No pods found on this node'}
            </div>
          ) : (
            pods.map((pod, podIndex) => {
              const processes = pod.processes || [];
              const podUid = pod.pod_uid || '--';
              const podId = pod.pod_id || '--';
              const isExpanded = expandedPods.has(podUid);
              
              return (
                <div 
                  key={podUid}
                  className={`pod-accordion-item ${isExpanded ? 'expanded' : ''}`}
                  data-pod-uid={podUid}
                >
                  <div 
                    className="pod-accordion-header"
                    onClick={() => togglePod(podUid)}
                  >
                    <div className="pod-accordion-info">
                      <span className="pod-accordion-toggle">▶</span>
                      <div className="pod-accordion-title">
                        <span className="pod-uid-full" title="Pod UID">{podUid}</span>
                        <span className="pod-id-badge">{podId}</span>
                      </div>
                    </div>
                    <div className="pod-accordion-meta">
                      <span className="process-count-badge">
                        {processes.length} process{processes.length !== 1 ? 'es' : ''}
                      </span>
                    </div>
                  </div>
                  <div className="pod-accordion-content">
                    {processes.length === 0 ? (
                      <div className="pods-empty-state">No processes in this pod</div>
                    ) : (
                      <table className="pod-processes-table">
                        <thead>
                          <tr>
                            <th>PID</th>
                            <th>Command</th>
                            <th>PPID</th>
                            <th>Container ID</th>
                          </tr>
                        </thead>
                        <tbody>
                          {processes.map((proc, procIndex) => (
                            <tr key={procIndex}>
                              <td className="pid-cell">{proc.pid || '--'}</td>
                              <td className="command-cell" title={proc.command || ''}>
                                {proc.command || '--'}
                              </td>
                              <td>{proc.ppid || '--'}</td>
                              <td className="container-id-cell" title={proc.container_id || ''}>
                                {truncateText(proc.container_id, 16)}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    )}
                  </div>
                </div>
              );
            })
          )}
        </div>
        <div className="pods-result">
          <details>
            <summary>Raw JSON Response</summary>
            <pre className={`code-block ${resultClass}`} id="podsResult">{rawResult}</pre>
          </details>
        </div>
      </div>
    </section>
  );
}
