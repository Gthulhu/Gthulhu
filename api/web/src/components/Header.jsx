import React, { useState } from 'react';
import { useApp } from '../context/AppContext';
import { LogIn, LogOut } from 'lucide-react';

export default function Header() {
  const { isAuthenticated, logout, showToast, getApiUrl, login, setCurrentUser } = useApp();
  const [showLoginModal, setShowLoginModal] = useState(false);

  const handleAuthClick = () => {
    if (isAuthenticated) {
      logout();
    } else {
      // Dispatch custom event to open login modal
      window.dispatchEvent(new CustomEvent('openLoginModal'));
    }
  };

  const handleConfigClick = () => {
    window.dispatchEvent(new CustomEvent('openConfigModal'));
  };

  return (
    <header className="main-header">
      <div className="header-content">
        <div className="logo-section">
          <div className="logo-icon">
            <img src="/logo.png" alt="Gthulhu Logo" className="logo-image" />
          </div>
          <div className="logo-text">
            <h1>GTHULHU</h1>
            <span className="tagline">Next generation scheduling platform</span>
          </div>
        </div>
        <div className="auth-section" id="authSection">
          <div className={`connection-status ${isAuthenticated ? 'connected' : ''}`} id="connectionStatus">
            <span className="status-dot"></span>
            <span className="status-text">{isAuthenticated ? 'Connected' : 'Disconnected'}</span>
          </div>
          <button 
            className={`auth-btn ${isAuthenticated ? 'logout' : ''}`}
            id="authBtn" 
            onClick={handleAuthClick}
          >
            <span className="btn-icon">{isAuthenticated ? <LogOut size={16} /> : <LogIn size={16} />}</span>
            <span>{isAuthenticated ? 'Logout' : 'Login'}</span>
          </button>
        </div>
      </div>
    </header>
  );
}
