import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';

const AppContext = createContext(null);

export function useApp() {
  const context = useContext(AppContext);
  if (!context) {
    throw new Error('useApp must be used within AppProvider');
  }
  return context;
}

export function AppProvider({ children }) {
  const [jwtToken, setJwtToken] = useState(() => localStorage.getItem('jwtToken'));
  const [isAuthenticated, setIsAuthenticated] = useState(() => !!localStorage.getItem('jwtToken'));
  const [apiBaseUrl, setApiBaseUrl] = useState(() => localStorage.getItem('apiBaseUrl') || '');
  const [healthHistory, setHealthHistory] = useState([]);
  const [strategyCounter, setStrategyCounter] = useState(0);
  const [currentUser, setCurrentUser] = useState(null);
  const [toasts, setToasts] = useState([]);

  // API URL helper
  const getApiUrl = useCallback((endpoint) => {
    if (!apiBaseUrl || apiBaseUrl === '') {
      return endpoint;
    }
    const base = apiBaseUrl.replace(/\/$/, '');
    return base + endpoint;
  }, [apiBaseUrl]);

  // Toast notifications
  const showToast = useCallback((type, message) => {
    const id = Date.now();
    const toast = { id, type, message, timestamp: new Date().toISOString() };
    setToasts(prev => [...prev, toast].slice(-50));
  }, []);

  const removeToast = useCallback((id) => {
    setToasts(prev => prev.filter(t => t.id !== id));
  }, []);

  const clearToasts = useCallback(() => {
    setToasts([]);
  }, []);

  // Authentication
  const login = useCallback((token) => {
    setJwtToken(token);
    setIsAuthenticated(true);
    localStorage.setItem('jwtToken', token);
  }, []);

  const logout = useCallback(() => {
    setJwtToken(null);
    setIsAuthenticated(false);
    setCurrentUser(null);
    localStorage.removeItem('jwtToken');
    showToast('info', 'You have been logged out');
  }, [showToast]);

  // Authenticated request helper
  const makeAuthenticatedRequest = useCallback(async (endpoint, options = {}) => {
    if (!isAuthenticated) {
      throw new Error('Authentication required');
    }

    const headers = {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer ' + jwtToken,
      ...options.headers
    };

    try {
      const response = await fetch(getApiUrl(endpoint), {
        ...options,
        headers
      });

      // Handle authentication and authorization errors
      if (response.status === 401) {
        // 401 = Token expired or invalid -> logout
        console.warn('[Auth] 401 Unauthorized - Token expired or invalid, logging out...');
        logout();
        showToast('error', 'Session expired. Please login again.');
        throw new Error('Session expired');
      } else if (response.status === 403) {
        // 403 = Forbidden - Valid token but insufficient permissions -> don't logout
        console.warn('[Auth] 403 Forbidden - Insufficient permissions');
        const errorData = await response.json().catch(() => ({ error: 'Permission denied' }));
        const errorMsg = errorData.error || 'You do not have permission to perform this action';
        showToast('error', errorMsg);
        throw new Error(errorMsg);
      }

      return response;
    } catch (error) {
      // If it's our custom error, just rethrow it
      if (error.message === 'Session expired' || error.message.includes('permission')) {
        throw error;
      }
      
      // For other network errors, log and rethrow
      console.error('[Auth] Request failed:', error);
      throw error;
    }
  }, [isAuthenticated, jwtToken, getApiUrl, logout, showToast]);

  // Save API config
  const saveApiConfig = useCallback((url) => {
    setApiBaseUrl(url);
    localStorage.setItem('apiBaseUrl', url);
    showToast('success', 'Configuration saved successfully');
  }, [showToast]);

  // Fetch current user profile
  const fetchCurrentUser = useCallback(async () => {
    if (!jwtToken) return;
    try {
      const headers = {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + jwtToken,
      };
      const base = apiBaseUrl ? apiBaseUrl.replace(/\/$/, '') : '';
      const response = await fetch(base + '/api/v1/users/self', { headers });
      if (response.ok) {
        const data = await response.json();
        if (data.success && data.data) {
          setCurrentUser(data.data);
        }
      }
    } catch {
      /* silent – user info is non-critical */
    }
  }, [jwtToken, apiBaseUrl]);

  // Auto-fetch user profile when authenticated
  useEffect(() => {
    if (isAuthenticated && jwtToken) {
      fetchCurrentUser();
    }
  }, [isAuthenticated, jwtToken, fetchCurrentUser]);

  // Handle token from URL (OAuth flows)
  useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search);
    const token = urlParams.get('token');
    if (token) {
      login(token);
      window.history.replaceState({}, document.title, window.location.pathname);
    }
  }, [login]);

  const value = {
    jwtToken,
    isAuthenticated,
    apiBaseUrl,
    healthHistory,
    setHealthHistory,
    strategyCounter,
    setStrategyCounter,
    currentUser,
    setCurrentUser,
    toasts,
    showToast,
    removeToast,
    clearToasts,
    login,
    logout,
    getApiUrl,
    makeAuthenticatedRequest,
    saveApiConfig
  };

  return (
    <AppContext.Provider value={value}>
      {children}
    </AppContext.Provider>
  );
}
