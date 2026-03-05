import React, { useState, useEffect, useRef } from 'react';
import { useApp } from '../../context/AppContext';
import { Lock, Mail, Key, Shield } from 'lucide-react';

export default function LoginModal() {
  const { isAuthenticated, login, showToast, getApiUrl, setCurrentUser, makeAuthenticatedRequest } = useApp();
  const [isOpen, setIsOpen] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const emailRef = useRef(null);

  // Auto-show login modal when not authenticated
  useEffect(() => {
    if (!isAuthenticated) {
      setIsOpen(true);
      setTimeout(() => emailRef.current?.focus(), 100);
    } else {
      setIsOpen(false);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    const handleOpen = () => {
      setIsOpen(true);
      setTimeout(() => emailRef.current?.focus(), 100);
    };
    
    window.addEventListener('openLoginModal', handleOpen);
    return () => window.removeEventListener('openLoginModal', handleOpen);
  }, []);

  // Disable ESC key close when not authenticated (force login)
  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape' && isOpen && isAuthenticated) {
        handleClose();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, isAuthenticated]);

  const handleClose = () => {
    // Only allow close if authenticated
    if (!isAuthenticated) return;
    
    setIsOpen(false);
    setEmail('');
    setPassword('');
    setError('');
    setLoading(false);
  };

  const handleOverlayClick = (e) => {
    // Only allow overlay click close if authenticated
    if (e.target === e.currentTarget && isAuthenticated) {
      handleClose();
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    
    if (!email || !password) {
      setError('Please enter both email and password');
      return;
    }
    
    setLoading(true);
    setError('');
    
    try {
      const response = await fetch(getApiUrl('/api/v1/auth/login'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username: email, password })
      });
      
      const data = await response.json();
      
      if (response.ok && data.success && data.data && data.data.token) {
        login(data.data.token);
        handleClose();
        showToast('success', 'Authentication successful!');
        
        // Trigger profile refresh
        window.dispatchEvent(new CustomEvent('refreshUserProfile'));
        window.dispatchEvent(new CustomEvent('checkHealth'));
      } else {
        throw new Error(data.error || data.message || 'Authentication failed');
      }
    } catch (error) {
      console.error('Login error:', error);
      setError(error.message || 'Failed to authenticate. Please check your credentials.');
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className={`modal-overlay ${isOpen ? 'active' : ''} ${!isAuthenticated ? 'force-login' : ''}`} onClick={handleOverlayClick}>
      <div className="modal-container">
        <div className="modal-header">
          <h2>
            <span className="modal-icon"><Lock size={18} /></span>
            Login
          </h2>
          {isAuthenticated && (
            <button className="modal-close" onClick={handleClose}>×</button>
          )}
        </div>
        <div className="modal-body">
          <form onSubmit={handleSubmit}>
            <div className="input-group">
              <label htmlFor="email">
                <span className="label-icon"><Mail size={14} /></span>
                Email Address
              </label>
              <input 
                type="email" 
                id="email" 
                name="email" 
                placeholder="admin@gthulhu.io" 
                required 
                autoComplete="email"
                ref={emailRef}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
              <div className="input-glow"></div>
            </div>
            <div className="input-group">
              <label htmlFor="password">
                <span className="label-icon"><Key size={14} /></span>
                Password
              </label>
              <input 
                type="password" 
                id="password" 
                name="password" 
                placeholder="••••••••" 
                required 
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <div className="input-glow"></div>
            </div>
            <div className="form-actions">
              <button 
                type="submit" 
                className={`submit-btn ${loading ? 'loading' : ''}`}
                disabled={loading}
              >
                <span className="btn-text">Login</span>
                <span className="btn-loader"></span>
              </button>
            </div>
            {error && <div className="error-message show">{error}</div>}
          </form>
        </div>
      </div>
    </div>
  );
}
