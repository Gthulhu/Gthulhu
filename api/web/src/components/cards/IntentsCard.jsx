import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useApp } from '../../context/AppContext';
import { ClipboardList, RefreshCw, Lock, Loader2, XCircle, Inbox, ChevronDown, ChevronRight, Server, HelpCircle } from 'lucide-react';

export default function IntentsCard() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast } = useApp();
  const [intents, setIntents] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [expandedIntents, setExpandedIntents] = useState({});
  const [selectedNode, setSelectedNode] = useState(null);

  const getIntents = useCallback(async () => {
    if (!isAuthenticated) return;
    
    setLoading(true);
    setError('');
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/intents/self');
      const data = await response.json();
      
      if (data.success) {
        const loadedIntents = data.data && data.data.intents ? data.data.intents : [];
        setIntents(loadedIntents);
        
        // Auto-select first node if none selected
        if (loadedIntents.length > 0) {
          const nodes = [...new Set(loadedIntents.map(i => i.NodeID))];
          if (!selectedNode || !nodes.includes(selectedNode)) {
            setSelectedNode(nodes[0]);
          }
          showToast('success', `Loaded ${loadedIntents.length} intent(s) across ${nodes.length} node(s)`);
        } else {
          setSelectedNode(null);
          showToast('info', 'No intents found');
        }
      } else {
        setError(data.error || data.message || 'Failed to load intents');
        setIntents([]);
        setSelectedNode(null);
      }
    } catch (error) {
      setError(error.message);
      setIntents([]);
      setSelectedNode(null);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast, selectedNode]);

  // Group intents by node
  const intentsByNode = useMemo(() => {
    const grouped = {};
    intents.forEach(intent => {
      const nodeId = intent.NodeID || 'Unknown';
      if (!grouped[nodeId]) {
        grouped[nodeId] = [];
      }
      grouped[nodeId].push(intent);
    });
    return grouped;
  }, [intents]);

  // Get list of nodes
  const nodes = useMemo(() => Object.keys(intentsByNode), [intentsByNode]);

  // Get intents for selected node
  const filteredIntents = useMemo(() => {
    if (!selectedNode) return [];
    return intentsByNode[selectedNode] || [];
  }, [intentsByNode, selectedNode]);

  useEffect(() => {
    const handleRefresh = () => getIntents();
    window.addEventListener('refreshIntents', handleRefresh);
    return () => window.removeEventListener('refreshIntents', handleRefresh);
  }, [getIntents]);

  // Auto-load intents on mount when authenticated
  useEffect(() => {
    if (isAuthenticated) {
      getIntents();
    }
  }, [isAuthenticated]);



  const toggleExpand = (intentId) => {
    setExpandedIntents(prev => ({
      ...prev,
      [intentId]: !prev[intentId]
    }));
  };

  const getStateLabel = (state) => {
    const states = {
      0: 'Pending',
      1: 'Active', 
      2: 'Applied',
      3: 'Failed'
    };
    return states[state] || `Unknown (${state})`;
  };

  const getStateClass = (state) => {
    const classes = {
      0: 'pending',
      1: 'active',
      2: 'applied',
      3: 'failed'
    };
    return classes[state] || 'unknown';
  };

  return (
    <section className="card intents-card full-width">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><ClipboardList size={18} /></span>
          <h2>Schedule Intents</h2>
          <div className="help-tooltip">
            <HelpCircle size={14} className="help-icon" />
            <div className="tooltip-content">
              <p><strong>Schedule Intents</strong> are automatically generated based on the strategies you define.</p>
              <p className="tooltip-warning">⚠️ Deleting intents is <strong>not recommended</strong> unless for testing purposes, as it may lead to system inconsistencies.</p>
              <p>Intents will be recreated based on existing strategies. Manage strategies directly for system integrity.</p>
            </div>
          </div>
        </div>
        <div className="card-actions">
          <button 
            className="icon-btn auth-required" 
            onClick={getIntents}
            title="Load Intents" 
            disabled={!isAuthenticated}
          >
            <RefreshCw size={16} />
          </button>
        </div>
      </div>
      <div className="card-body">
        {/* Node Selector Tabs */}
        {nodes.length > 0 && (
          <div className="node-tabs">
            <div className="node-tabs-header">
              <Server size={14} />
              <span>Select Node:</span>
            </div>
            <div className="node-tabs-list">
              {nodes.map(nodeId => (
                <button
                  key={nodeId}
                  className={`node-tab ${selectedNode === nodeId ? 'active' : ''}`}
                  onClick={() => setSelectedNode(nodeId)}
                >
                  <span className="node-tab-name">{nodeId}</span>
                  <span className="node-tab-count">{intentsByNode[nodeId].length}</span>
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="intents-list" id="intentsList">
          {!isAuthenticated && (
            <div className="empty-state">
              <span className="empty-icon"><Lock size={24} /></span>
              <p>Authenticate to view schedule intents...</p>
            </div>
          )}
          
          {isAuthenticated && loading && (
            <div className="empty-state">
              <span className="empty-icon"><Loader2 size={24} className="spin" /></span>
              <p>Loading intents...</p>
            </div>
          )}
          
          {isAuthenticated && !loading && error && (
            <div className="empty-state error">
              <span className="empty-icon"><XCircle size={24} /></span>
              <p>Error: {error}</p>
            </div>
          )}
          
          {isAuthenticated && !loading && !error && intents.length === 0 && (
            <div className="empty-state">
              <span className="empty-icon"><Inbox size={24} /></span>
              <p>No intents found. Click the refresh button to load intents.</p>
            </div>
          )}

          {isAuthenticated && !loading && !error && intents.length > 0 && selectedNode && (
            <div className="node-intents-info">
              <Server size={14} />
              <span>Showing {filteredIntents.length} intent(s) on node: <strong>{selectedNode}</strong></span>
            </div>
          )}
          
          {filteredIntents.map((intent) => (
            <div key={intent.ID} className="intent-item">
              <div className="intent-header" onClick={() => toggleExpand(intent.ID)}>
                <div className="intent-title">
                  <span className="expand-icon">{expandedIntents[intent.ID] ? <ChevronDown size={16} /> : <ChevronRight size={16} />}</span>
                  <span className="intent-id">Intent: {intent.ID.slice(-8)}...</span>
                  <span className={`intent-state ${getStateClass(intent.State)}`}>
                    {getStateLabel(intent.State)}
                  </span>
                </div>
                <div className="intent-summary">
                  <span className="intent-priority">Priority: {intent.Priority}</span>
                  <span className="intent-namespace">NS: {intent.K8sNamespace}</span>
                </div>
              </div>
              
              {expandedIntents[intent.ID] && (
                <div className="intent-details">
                  <div className="detail-grid">
                    <div className="detail-item">
                      <span className="detail-label">Intent ID</span>
                      <span className="detail-value">{intent.ID}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Strategy ID</span>
                      <span className="detail-value">{intent.StrategyID}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Pod ID</span>
                      <span className="detail-value">{intent.PodID}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Node ID</span>
                      <span className="detail-value">{intent.NodeID}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">K8s Namespace</span>
                      <span className="detail-value">{intent.K8sNamespace}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Priority</span>
                      <span className="detail-value">{intent.Priority}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Execution Time</span>
                      <span className="detail-value">{intent.ExecutionTime} ns</span>
                    </div>
                    {intent.CommandRegex && (
                      <div className="detail-item">
                        <span className="detail-label">Command Regex</span>
                        <span className="detail-value">{intent.CommandRegex}</span>
                      </div>
                    )}
                  </div>
                  
                  {intent.PodLabels && Object.keys(intent.PodLabels).length > 0 && (
                    <div className="labels-section">
                      <span className="labels-title">Pod Labels</span>
                      <div className="labels-grid">
                        {Object.entries(intent.PodLabels).map(([key, value]) => (
                          <div key={key} className="label-item">
                            <span className="label-key">{key}</span>
                            <span className="label-value">{value}</span>
                          </div>
                        ))}
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
