import React from 'react';
import { AppProvider } from './context/AppContext';
import Header from './components/Header';
import Footer from './components/Footer';
import Dashboard from './components/Dashboard';
import LoginModal from './components/modals/LoginModal';
import ConfigModal from './components/modals/ConfigModal';
import ToastContainer from './components/ToastContainer';

function App() {
  return (
    <AppProvider>
      <div className="app">
        {/* Ambient Background Effects */}
        <div className="ambient-grid"></div>
        <div className="scan-line"></div>
        
        <Header />
        <Dashboard />
        <Footer />
        
        {/* Modals */}
        <LoginModal />
        <ConfigModal />
        
        {/* Toast Container */}
        <ToastContainer />
      </div>
    </AppProvider>
  );
}

export default App;
