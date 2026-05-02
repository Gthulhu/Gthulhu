import React, { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { LogIn, Eye, EyeOff } from 'lucide-react';

export default function LoginPage() {
  const { isAuthenticated, login, getApiUrl, showToast } = useApp();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');

  if (isAuthenticated) {
    return <Navigate to="/nodes" replace />;
  }

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!username.trim() || !password.trim()) {
      const message = 'Please enter username and password';
      setErrorMessage(message);
      showToast('error', message);
      return;
    }
    setLoading(true);
    setErrorMessage('');
    try {
      const response = await fetch(getApiUrl('/api/v1/auth/login'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: username.trim(), password }),
      });
      let data = {};
      let parseFailed = false;
      try {
        data = await response.json();
      } catch (parseError) {
        parseFailed = true;
        console.warn('Failed to parse login response:', parseError);
      }
      if (response.ok && data.success) {
        login({
          accessToken: data.data.accessToken || data.data.token,
          refreshToken: data.data.refreshToken || ''
        });
        showToast('success', 'Logged in successfully');
      } else {
        // Manager API errors use "error"; MSW mock responses use "message".
        const message = parseFailed ? 'Invalid server response' : (data.error || data.message || 'Login failed');
        setErrorMessage(message);
        showToast('error', message);
      }
    } catch (err) {
      const message = 'Connection error: ' + err.message;
      setErrorMessage(message);
      showToast('error', message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-page">
      {errorMessage && (
        <div className="login-failure-popup" role="alert" aria-live="assertive">
          <span>{errorMessage}</span>
          <button type="button" onClick={() => setErrorMessage('')} aria-label="Dismiss login error">
            ×
          </button>
        </div>
      )}
      <div className="login-card">
        <div className="login-header">
          <div className="sidebar-logo-icon" style={{ width: 48, height: 48, fontSize: 20 }}>
            G
          </div>
          <h1 className="login-title">Gthulhu</h1>
          <p className="login-subtitle">Kubernetes-native Scheduler Management</p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Username</label>
            <input
              type="text"
              className="form-input"
              placeholder="Enter your username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoFocus
              autoComplete="username"
            />
          </div>
          <div className="form-group">
            <label className="form-label">Password</label>
            <div style={{ position: 'relative' }}>
              <input
                type={showPassword ? 'text' : 'password'}
                className="form-input"
                placeholder="Enter your password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
              />
              <button
                type="button"
                className="btn-icon"
                style={{
                  position: 'absolute',
                  right: 8,
                  top: '50%',
                  transform: 'translateY(-50%)',
                }}
                onClick={() => setShowPassword((v) => !v)}
                tabIndex={-1}
              >
                {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            style={{ width: '100%', marginTop: 8 }}
            disabled={loading}
          >
            {loading ? (
              <span>Signing in...</span>
            ) : (
              <>
                <LogIn size={16} />
                <span>Sign In</span>
              </>
            )}
          </button>
        </form>
      </div>
    </div>
  );
}
