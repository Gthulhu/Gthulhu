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
  const [refreshToken, setRefreshToken] = useState(() => localStorage.getItem('refreshToken'));
  const [isAuthenticated, setIsAuthenticated] = useState(() => !!localStorage.getItem('jwtToken'));
  const [apiBaseUrl, setApiBaseUrl] = useState(() => localStorage.getItem('apiBaseUrl') || '');
  const [healthHistory, setHealthHistory] = useState([]);
  const [strategyCounter, setStrategyCounter] = useState(0);
  const [currentUser, setCurrentUser] = useState(null);
  const [toasts, setToasts] = useState([]);
  const refreshRequestRef = React.useRef(null);

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
  const login = useCallback((tokenOrPayload) => {
    const token = typeof tokenOrPayload === 'string' ? tokenOrPayload : tokenOrPayload?.accessToken;
    const refresh = typeof tokenOrPayload === 'string' ? '' : (tokenOrPayload?.refreshToken || '');
    if (!token) {
      return;
    }
    setJwtToken(token);
    setRefreshToken(refresh || null);
    setIsAuthenticated(true);
    localStorage.setItem('jwtToken', token);
    if (refresh) {
      localStorage.setItem('refreshToken', refresh);
    } else {
      localStorage.removeItem('refreshToken');
    }
  }, []);

  const logout = useCallback(async () => {
    const currentRefreshToken = localStorage.getItem('refreshToken');
    const base = apiBaseUrl ? apiBaseUrl.replace(/\/$/, '') : '';
    if (currentRefreshToken) {
      try {
        await fetch(base + '/api/v1/auth/logout', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refreshToken: currentRefreshToken })
        });
      } catch {
        // best effort revoke
      }
    }
    setJwtToken(null);
    setRefreshToken(null);
    setIsAuthenticated(false);
    setCurrentUser(null);
    localStorage.removeItem('jwtToken');
    localStorage.removeItem('refreshToken');
    showToast('info', 'You have been logged out');
  }, [apiBaseUrl, showToast]);

  const clearLocalAuth = useCallback(() => {
    setJwtToken(null);
    setRefreshToken(null);
    setIsAuthenticated(false);
    setCurrentUser(null);
    localStorage.removeItem('jwtToken');
    localStorage.removeItem('refreshToken');
  }, []);

  const refreshAccessToken = useCallback(async () => {
    if (refreshRequestRef.current) {
      return refreshRequestRef.current;
    }

    const currentRefreshToken = localStorage.getItem('refreshToken') || refreshToken;
    if (!currentRefreshToken) {
      return null;
    }

    refreshRequestRef.current = (async () => {
      const response = await fetch(getApiUrl('/api/v1/auth/refresh'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refreshToken: currentRefreshToken })
      });

      if (!response.ok) {
        return null;
      }
      const data = await response.json();
      if (!data?.success || !data?.data?.accessToken) {
        return null;
      }
      login({ accessToken: data.data.accessToken, refreshToken: data.data.refreshToken || '' });
      return data.data.accessToken;
    })();

    try {
      return await refreshRequestRef.current;
    } finally {
      refreshRequestRef.current = null;
    }
  }, [refreshToken, getApiUrl, login]);

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
        const newAccessToken = await refreshAccessToken();
        if (newAccessToken) {
          const retryResponse = await fetch(getApiUrl(endpoint), {
            ...options,
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer ' + newAccessToken,
              ...options.headers
            }
          });
          if (retryResponse.status !== 401) {
            if (retryResponse.status === 403) {
              const retryErrorData = await retryResponse.json().catch(() => ({ error: 'Permission denied' }));
              const retryErrorMsg = retryErrorData.error || 'You do not have permission to perform this action';
              showToast('error', retryErrorMsg);
              throw new Error(retryErrorMsg);
            }
            return retryResponse;
          }
        }

        console.warn('[Auth] 401 Unauthorized - Token expired or revoked, logging out...');
        clearLocalAuth();
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
  }, [isAuthenticated, jwtToken, getApiUrl, refreshAccessToken, clearLocalAuth, showToast]);

  // Runtime config API
  const getRuntimeConfigStatus = useCallback(async (nodeIds = []) => {
    const qs = nodeIds.length ? '?nodeIds=' + nodeIds.join(',') : '';
    const res = await makeAuthenticatedRequest('/api/v1/scheduler/runtime-config/status' + qs);
    const data = await res.json();
    if (!data.success) throw new Error(data.error || 'Failed to get runtime config status');
    return data.data?.results || [];
  }, [makeAuthenticatedRequest]);

  const applyRuntimeConfig = useCallback(async (nodeIds, config) => {
    const res = await makeAuthenticatedRequest('/api/v1/scheduler/runtime-config/apply', {
      method: 'POST',
      body: JSON.stringify({ nodeIds, config }),
    });
    const data = await res.json();
    if (!data.success) throw new Error(data.error || 'Failed to apply runtime config');
    return data.data?.results || [];
  }, [makeAuthenticatedRequest]);

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
    saveApiConfig,
    getRuntimeConfigStatus,
    applyRuntimeConfig
  };

  return (
    <AppContext.Provider value={value}>
      {children}
    </AppContext.Provider>
  );
}
