import React, { useState, useEffect, useCallback } from 'react';
import { useApp } from '../../context/AppContext';
import { Target, Download, Trash2, Save, FolderOpen, Loader2, XCircle, Inbox, ChevronDown, ChevronRight, HelpCircle, Pencil } from 'lucide-react';

export default function StrategiesCard() {
  const { isAuthenticated, makeAuthenticatedRequest, showToast, strategyCounter, setStrategyCounter } = useApp();
  const [strategies, setStrategies] = useState([]);
  const [loadedStrategies, setLoadedStrategies] = useState([]);
  const [expandedStrategies, setExpandedStrategies] = useState({});
  const [editingStrategyId, setEditingStrategyId] = useState(null);
  const [editStrategy, setEditStrategy] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const getStrategies = useCallback(async () => {
    if (!isAuthenticated) return;
    
    setLoading(true);
    setError('');
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/strategies/self');
      const data = await response.json();
      
      if (data.success) {
        const loaded = data.data && data.data.strategies ? data.data.strategies : [];
        setLoadedStrategies(loaded);
        
        if (loaded.length > 0) {
          showToast('success', `Loaded ${loaded.length} strategy(ies)`);
        } else {
          showToast('info', 'No strategies found');
        }
      } else {
        setError(data.error || data.message || 'Failed to load strategies');
        setLoadedStrategies([]);
      }
    } catch (error) {
      setError(error.message);
      setLoadedStrategies([]);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, makeAuthenticatedRequest, showToast]);

  useEffect(() => {
    const handleRefresh = () => getStrategies();
    window.addEventListener('refreshStrategies', handleRefresh);
    return () => window.removeEventListener('refreshStrategies', handleRefresh);
  }, [getStrategies]);

  // Auto-load strategies on mount when authenticated
  useEffect(() => {
    if (isAuthenticated) {
      getStrategies();
    }
  }, [isAuthenticated]);

  const toggleExpand = (strategyId) => {
    setExpandedStrategies(prev => ({
      ...prev,
      [strategyId]: !prev[strategyId]
    }));
  };

  const beginEditStrategy = (strategy) => {
    const selectors = (strategy.LabelSelectors || []).map(selector => ({
      key: selector.key || '',
      value: selector.value || ''
    }));
    if (selectors.length === 0) {
      selectors.push({ key: '', value: '' });
    }
    setEditingStrategyId(strategy.ID);
    setEditStrategy({
      id: strategy.ID,
      strategyNamespace: strategy.StrategyNamespace || '',
      priority: strategy.Priority || 0,
      executionTime: strategy.ExecutionTime || 0,
      commandRegex: strategy.CommandRegex || '',
      k8sNamespace: strategy.K8sNamespace ? strategy.K8sNamespace.join(', ') : '',
      selectors
    });
    setExpandedStrategies(prev => ({
      ...prev,
      [strategy.ID]: true
    }));
  };

  const cancelEditStrategy = () => {
    setEditingStrategyId(null);
    setEditStrategy(null);
  };

  const updateEditField = (field, value) => {
    setEditStrategy(prev => ({
      ...prev,
      [field]: value
    }));
  };

  const addEditSelector = () => {
    setEditStrategy(prev => ({
      ...prev,
      selectors: [...prev.selectors, { key: '', value: '' }]
    }));
  };

  const removeEditSelector = (index) => {
    setEditStrategy(prev => {
      const selectors = prev.selectors.filter((_, i) => i !== index);
      if (selectors.length === 0) {
        selectors.push({ key: '', value: '' });
      }
      return { ...prev, selectors };
    });
  };

  const updateEditSelector = (index, field, value) => {
    setEditStrategy(prev => {
      const selectors = [...prev.selectors];
      selectors[index] = { ...selectors[index], [field]: value };
      return { ...prev, selectors };
    });
  };

  const addStrategy = () => {
    const newId = strategyCounter + 1;
    setStrategyCounter(newId);
    setStrategies(prev => [...prev, {
      id: `strategy-${newId}`,
      number: newId,
      strategyNamespace: '',
      priority: 10,
      executionTime: 20000000,
      commandRegex: '',
      k8sNamespace: '',
      selectors: [{ key: '', value: '' }]
    }]);
  };

  const removeStrategy = (strategyId) => {
    setStrategies(prev => prev.filter(s => s.id !== strategyId));
  };

  const clearAllStrategies = () => {
    setStrategies([]);
    setStrategyCounter(0);
    showToast('info', 'Form cleared');
  };

  const updateStrategy = (strategyId, field, value) => {
    setStrategies(prev => prev.map(s => 
      s.id === strategyId ? { ...s, [field]: value } : s
    ));
  };

  const addSelector = (strategyId) => {
    setStrategies(prev => prev.map(s => 
      s.id === strategyId 
        ? { ...s, selectors: [...s.selectors, { key: '', value: '' }] }
        : s
    ));
  };

  const removeSelector = (strategyId, index) => {
    setStrategies(prev => prev.map(s => {
      if (s.id !== strategyId) return s;
      const newSelectors = s.selectors.filter((_, i) => i !== index);
      if (newSelectors.length === 0) {
        newSelectors.push({ key: '', value: '' });
      }
      return { ...s, selectors: newSelectors };
    }));
  };

  const updateSelector = (strategyId, index, field, value) => {
    setStrategies(prev => prev.map(s => {
      if (s.id !== strategyId) return s;
      const newSelectors = [...s.selectors];
      newSelectors[index] = { ...newSelectors[index], [field]: value };
      return { ...s, selectors: newSelectors };
    }));
  };

  const saveAllStrategies = async () => {
    if (strategies.length === 0) {
      showToast('error', 'No strategies to save');
      return;
    }
    
    for (const item of strategies) {
      const strategy = {};
      
      if (item.strategyNamespace.trim()) {
        strategy.strategyNamespace = item.strategyNamespace.trim();
      }
      
      if (item.priority) {
        strategy.priority = parseInt(item.priority);
      }
      
      if (item.executionTime) {
        strategy.executionTime = parseInt(item.executionTime);
      }
      
      if (item.commandRegex.trim()) {
        strategy.commandRegex = item.commandRegex.trim();
      }
      
      if (item.k8sNamespace.trim()) {
        strategy.k8sNamespace = item.k8sNamespace.trim().split(',').map(ns => ns.trim()).filter(ns => ns);
      }
      
      const labelSelectors = item.selectors
        .filter(s => s.key.trim() && s.value.trim())
        .map(s => ({ key: s.key.trim(), value: s.value.trim() }));
      
      if (labelSelectors.length > 0) {
        strategy.labelSelectors = labelSelectors;
      }
      
      try {
        const response = await makeAuthenticatedRequest('/api/v1/strategies', {
          method: 'POST',
          body: JSON.stringify(strategy)
        });
        
        const data = await response.json();
        
        if (data.success) {
          showToast('success', 'Strategy created successfully');
          // Refresh loaded strategies and intents
          getStrategies();
          window.dispatchEvent(new CustomEvent('refreshIntents'));
        } else {
          showToast('error', data.error || data.message || 'Failed to create strategy');
        }
      } catch (error) {
        showToast('error', error.message);
      }
    }
  };

  const handleDeleteStrategy = async (strategyId) => {
    if (!window.confirm('Are you sure you want to delete this strategy? This will also delete all associated intents.')) {
      return;
    }
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/strategies', {
        method: 'DELETE',
        body: JSON.stringify({ strategyId })
      });
      
      const data = await response.json();
      
      if (data.success) {
        showToast('success', 'Strategy deleted successfully');
        getStrategies();
        window.dispatchEvent(new CustomEvent('refreshIntents'));
      } else {
        showToast('error', data.error || data.message || 'Failed to delete strategy');
      }
    } catch (error) {
      showToast('error', 'Error: ' + error.message);
    }
  };

  const handleUpdateStrategy = async () => {
    if (!editStrategy) {
      return;
    }

    const payload = {
      strategyId: editStrategy.id
    };

    if (editStrategy.strategyNamespace.trim()) {
      payload.strategyNamespace = editStrategy.strategyNamespace.trim();
    }

    if (editStrategy.priority !== '') {
      payload.priority = parseInt(editStrategy.priority, 10);
    }

    if (editStrategy.executionTime !== '') {
      payload.executionTime = parseInt(editStrategy.executionTime, 10);
    }

    if (editStrategy.commandRegex.trim()) {
      payload.commandRegex = editStrategy.commandRegex.trim();
    }

    if (editStrategy.k8sNamespace.trim()) {
      payload.k8sNamespace = editStrategy.k8sNamespace.split(',').map(ns => ns.trim()).filter(ns => ns);
    }

    const labelSelectors = editStrategy.selectors
      .filter(s => s.key.trim() && s.value.trim())
      .map(s => ({ key: s.key.trim(), value: s.value.trim() }));

    if (labelSelectors.length > 0) {
      payload.labelSelectors = labelSelectors;
    }

    try {
      const response = await makeAuthenticatedRequest('/api/v1/strategies', {
        method: 'PUT',
        body: JSON.stringify(payload)
      });

      const data = await response.json();
      if (data.success) {
        showToast('success', 'Strategy updated successfully');
        cancelEditStrategy();
        getStrategies();
        window.dispatchEvent(new CustomEvent('refreshIntents'));
      } else {
        showToast('error', data.error || data.message || 'Failed to update strategy');
      }
    } catch (error) {
      showToast('error', 'Error: ' + error.message);
    }
  };

  return (
    <section className="card strategies-card full-width">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><Target size={18} /></span>
          <h2>Scheduling Strategies</h2>
          <div className="help-tooltip">
            <HelpCircle size={14} className="help-icon" />
            <div className="tooltip-content">
              <p><strong>Scheduling Strategies</strong> define how pods should be scheduled based on label selectors, namespaces, and priority.</p>
              <p>When you create a strategy, the system automatically generates <strong>Schedule Intents</strong> for matching pods.</p>
              <p className="tooltip-success">✅ Manage strategies to control pod scheduling behavior across your cluster.</p>
            </div>
          </div>
        </div>
        <div className="card-actions">
          <button 
            className="icon-btn auth-required" 
            onClick={getStrategies}
            title="Load Strategies" 
            disabled={!isAuthenticated}
          >
            <Download size={16} />
          </button>
          <button 
            className="primary-btn auth-required" 
            onClick={addStrategy}
            disabled={!isAuthenticated}
          >
            <span>+</span> New Strategy
          </button>
        </div>
      </div>
      <div className="card-body">
        <div className="strategies-container" id="strategiesContainer">
          {strategies.map(strategy => (
            <div key={strategy.id} className="strategy-item" id={strategy.id}>
              <div className="strategy-header">
                <h4>Strategy #{strategy.number}</h4>
                <button 
                  type="button" 
                  className="remove-strategy-btn"
                  onClick={() => removeStrategy(strategy.id)}
                >
                  ✕ Remove
                </button>
              </div>
              <div className="strategy-form">
                <div>
                  <label>Strategy Namespace</label>
                  <input 
                    type="text" 
                    name="strategyNamespace"
                    placeholder="e.g., default, trading, ml"
                    value={strategy.strategyNamespace}
                    onChange={(e) => updateStrategy(strategy.id, 'strategyNamespace', e.target.value)}
                  />
                </div>
                <div>
                  <label>Priority (0-20)</label>
                  <input 
                    type="number" 
                    name="priority"
                    value={strategy.priority}
                    min="0" 
                    max="20"
                    placeholder="10"
                    onChange={(e) => updateStrategy(strategy.id, 'priority', e.target.value)}
                  />
                </div>
                <div>
                  <label>Execution Time (ns)</label>
                  <input 
                    type="number" 
                    name="executionTime"
                    value={strategy.executionTime}
                    placeholder="20000000"
                    onChange={(e) => updateStrategy(strategy.id, 'executionTime', e.target.value)}
                  />
                </div>
                <div>
                  <label>Command Regex (optional)</label>
                  <input 
                    type="text" 
                    name="commandRegex"
                    placeholder="e.g., nr-gnb|ping"
                    value={strategy.commandRegex}
                    onChange={(e) => updateStrategy(strategy.id, 'commandRegex', e.target.value)}
                  />
                </div>
                <div className="full-width">
                  <label>K8s Namespaces (comma separated)</label>
                  <input 
                    type="text" 
                    name="k8sNamespace"
                    placeholder="default, kube-system"
                    value={strategy.k8sNamespace}
                    onChange={(e) => updateStrategy(strategy.id, 'k8sNamespace', e.target.value)}
                  />
                </div>
                <div className="full-width selectors-container">
                  <label>Label Selectors</label>
                  <div className="selectors-list" id={`selectors-${strategy.id}`}>
                    {strategy.selectors.map((selector, index) => (
                      <div key={index} className="selector-row">
                        <input 
                          type="text" 
                          name="selectorKey"
                          placeholder="Key"
                          value={selector.key}
                          onChange={(e) => updateSelector(strategy.id, index, 'key', e.target.value)}
                        />
                        <input 
                          type="text" 
                          name="selectorValue"
                          placeholder="Value"
                          value={selector.value}
                          onChange={(e) => updateSelector(strategy.id, index, 'value', e.target.value)}
                        />
                        <button 
                          type="button"
                          onClick={() => removeSelector(strategy.id, index)}
                        >
                          ✕
                        </button>
                      </div>
                    ))}
                  </div>
                  <button 
                    type="button" 
                    className="add-selector-btn"
                    onClick={() => addSelector(strategy.id)}
                  >
                    + Add Selector
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
        {strategies.length > 0 && (
          <div className="strategies-actions" id="strategiesActions" style={{ display: 'flex' }}>
            <button className="danger-btn" onClick={clearAllStrategies}>
              <Trash2 size={16} /> Clear Form
            </button>
            <button 
              className="success-btn auth-required" 
              onClick={saveAllStrategies}
              disabled={!isAuthenticated}
            >
              <Save size={16} /> Create Strategy
            </button>
          </div>
        )}
        
        {/* Loaded Strategies Display */}
        <div className="loaded-strategies-section">
          <h3 className="section-title"><FolderOpen size={16} /> Saved Strategies</h3>
          
          {loading && (
            <div className="empty-state">
              <span className="empty-icon"><Loader2 size={24} className="spin" /></span>
              <p>Loading strategies...</p>
            </div>
          )}
          
          {!loading && error && (
            <div className="empty-state error">
              <span className="empty-icon"><XCircle size={24} /></span>
              <p>Error: {error}</p>
            </div>
          )}
          
          {!loading && !error && loadedStrategies.length === 0 && (
            <div className="empty-state">
              <span className="empty-icon"><Inbox size={24} /></span>
              <p>No strategies found. Click the download button to load strategies.</p>
            </div>
          )}
          
          {loadedStrategies.map((strategy) => (
            <div key={strategy.ID} className="strategy-loaded-item">
              <div className="strategy-loaded-header">
                <div className="strategy-loaded-title" onClick={() => toggleExpand(strategy.ID)}>
                  <span className="expand-icon">{expandedStrategies[strategy.ID] ? <ChevronDown size={16} /> : <ChevronRight size={16} />}</span>
                  <span className="strategy-id">Strategy: {strategy.ID.slice(-8)}...</span>
                  {strategy.StrategyNamespace && (
                    <span className="strategy-namespace-badge">{strategy.StrategyNamespace}</span>
                  )}
                </div>
                <div className="strategy-loaded-summary">
                  <span className="strategy-priority">Priority: {strategy.Priority}</span>
                  {strategy.K8sNamespace && strategy.K8sNamespace.length > 0 && (
                    <span className="strategy-k8s-ns">K8s NS: {strategy.K8sNamespace.join(', ')}</span>
                  )}
                  <button 
                    className="secondary-btn-small"
                    onClick={(e) => {
                      e.stopPropagation();
                      beginEditStrategy(strategy);
                    }}
                    title="Edit Strategy"
                    disabled={!isAuthenticated}
                  >
                    <Pencil size={14} />
                  </button>
                  <button 
                    className="danger-btn-small" 
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDeleteStrategy(strategy.ID);
                    }}
                    title="Delete Strategy"
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
              
              {expandedStrategies[strategy.ID] && (
                <div className="strategy-loaded-details">
                  <div className="detail-grid">
                    <div className="detail-item">
                      <span className="detail-label">Strategy ID</span>
                      <span className="detail-value">{strategy.ID}</span>
                    </div>
                    {strategy.StrategyNamespace && (
                      <div className="detail-item">
                        <span className="detail-label">Namespace</span>
                        <span className="detail-value">{strategy.StrategyNamespace}</span>
                      </div>
                    )}
                    <div className="detail-item">
                      <span className="detail-label">Priority</span>
                      <span className="detail-value">{strategy.Priority}</span>
                    </div>
                    <div className="detail-item">
                      <span className="detail-label">Execution Time</span>
                      <span className="detail-value">{strategy.ExecutionTime} ns</span>
                    </div>
                    {strategy.CommandRegex && (
                      <div className="detail-item">
                        <span className="detail-label">Command Regex</span>
                        <span className="detail-value">{strategy.CommandRegex}</span>
                      </div>
                    )}
                    {strategy.K8sNamespace && strategy.K8sNamespace.length > 0 && (
                      <div className="detail-item">
                        <span className="detail-label">K8s Namespaces</span>
                        <span className="detail-value">{strategy.K8sNamespace.join(', ')}</span>
                      </div>
                    )}
                  </div>
                  
                  {strategy.LabelSelectors && strategy.LabelSelectors.length > 0 && (
                    <div className="labels-section">
                      <span className="labels-title">Label Selectors</span>
                      <div className="labels-grid">
                        {strategy.LabelSelectors.map((selector, index) => (
                          <div key={index} className="label-item">
                            <span className="label-key">{selector.key}</span>
                            <span className="label-value">{selector.value}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {editingStrategyId === strategy.ID && editStrategy && (
                    <div className="strategy-edit-form">
                      <h4>Edit Strategy</h4>
                      <div className="strategy-form">
                        <div>
                          <label>Strategy Namespace</label>
                          <input
                            type="text"
                            value={editStrategy.strategyNamespace}
                            onChange={(e) => updateEditField('strategyNamespace', e.target.value)}
                          />
                        </div>
                        <div>
                          <label>Priority (0-20)</label>
                          <input
                            type="number"
                            min="0"
                            max="20"
                            value={editStrategy.priority}
                            onChange={(e) => updateEditField('priority', e.target.value)}
                          />
                        </div>
                        <div>
                          <label>Execution Time (ns)</label>
                          <input
                            type="number"
                            value={editStrategy.executionTime}
                            onChange={(e) => updateEditField('executionTime', e.target.value)}
                          />
                        </div>
                        <div>
                          <label>Command Regex (optional)</label>
                          <input
                            type="text"
                            value={editStrategy.commandRegex}
                            onChange={(e) => updateEditField('commandRegex', e.target.value)}
                          />
                        </div>
                        <div className="full-width">
                          <label>K8s Namespaces (comma separated)</label>
                          <input
                            type="text"
                            value={editStrategy.k8sNamespace}
                            onChange={(e) => updateEditField('k8sNamespace', e.target.value)}
                          />
                        </div>
                        <div className="full-width selectors-container">
                          <label>Label Selectors</label>
                          <div className="selectors-list">
                            {editStrategy.selectors.map((selector, index) => (
                              <div key={index} className="selector-row">
                                <input
                                  type="text"
                                  placeholder="Key"
                                  value={selector.key}
                                  onChange={(e) => updateEditSelector(index, 'key', e.target.value)}
                                />
                                <input
                                  type="text"
                                  placeholder="Value"
                                  value={selector.value}
                                  onChange={(e) => updateEditSelector(index, 'value', e.target.value)}
                                />
                                <button type="button" onClick={() => removeEditSelector(index)}>
                                  ✕
                                </button>
                              </div>
                            ))}
                          </div>
                          <button type="button" className="add-selector-btn" onClick={addEditSelector}>
                            + Add Selector
                          </button>
                        </div>
                      </div>
                      <div className="strategies-actions" style={{ display: 'flex' }}>
                        <button className="danger-btn" onClick={cancelEditStrategy}>
                          Cancel
                        </button>
                        <button className="success-btn auth-required" onClick={handleUpdateStrategy} disabled={!isAuthenticated}>
                          <Save size={16} /> Update Strategy
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
