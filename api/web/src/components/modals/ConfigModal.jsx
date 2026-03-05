import React, { useState, useEffect } from 'react';
import { useApp } from '../../context/AppContext';
import { Settings, Globe } from 'lucide-react';

export default function ConfigModal() {
  const { apiBaseUrl, saveApiConfig } = useApp();
  const [isOpen, setIsOpen] = useState(false);
  const [url, setUrl] = useState('');

  useEffect(() => {
    const handleOpen = () => {
      setUrl(apiBaseUrl);
      setIsOpen(true);
    };
    
    window.addEventListener('openConfigModal', handleOpen);
    return () => window.removeEventListener('openConfigModal', handleOpen);
  }, [apiBaseUrl]);

  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape' && isOpen) {
        handleClose();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen]);

  const handleClose = () => {
    setIsOpen(false);
  };

  const handleOverlayClick = (e) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  };

  const handleSave = () => {
    saveApiConfig(url.trim());
    handleClose();
  };

  if (!isOpen) return null;

  return (
    <div className={`modal-overlay ${isOpen ? 'active' : ''}`} onClick={handleOverlayClick}>
      <div className="modal-container modal-sm">
        <div className="modal-header">
          <h2>
            <span className="modal-icon"><Settings size={18} /></span>
            API Configuration
          </h2>
          <button className="modal-close" onClick={handleClose}>×</button>
        </div>
        <div className="modal-body">
          <div className="input-group">
            <label htmlFor="apiBaseUrl">
              <span className="label-icon"><Globe size={14} /></span>
              API Base URL
            </label>
            <input 
              type="url" 
              id="apiBaseUrl" 
              placeholder="http://localhost:8080"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
            />
            <div className="input-glow"></div>
            <small className="input-hint">Leave empty for same-origin requests</small>
          </div>
          <div className="form-actions">
            <button type="button" className="submit-btn" onClick={handleSave}>
              <span className="btn-text">Save Configuration</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
