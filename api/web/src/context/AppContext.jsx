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

    const response = await fetch(getApiUrl(endpoint), {
      ...options,
      headers
    });

    if (response.status === 401) {
      logout();
      showToast('error', 'Session expired. Please login again.');
      throw new Error('Session expired');
    }

    return response;
  }, [isAuthenticated, jwtToken, getApiUrl, logout, showToast]);

  // Save API config
  const saveApiConfig = useCallback((url) => {
    setApiBaseUrl(url);
    localStorage.setItem('apiBaseUrl', url);
    showToast('success', 'Configuration saved successfully');
  }, [showToast]);

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
