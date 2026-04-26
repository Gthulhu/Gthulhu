import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './styles/index.css';

async function enableMocking() {
  if (import.meta.env.VITE_USE_MOCK !== 'true') {
    return;
  }
  const { worker } = await import('./mocks/browser.js');
  // When deployed under a subpath (e.g. GitHub Pages at /Gthulhu/),
  // the service worker file lives at <BASE_URL>mockServiceWorker.js.
  // MSW's default registration uses '/mockServiceWorker.js' which 404s
  // in that case, so we explicitly point it at the built asset.
  return worker.start({
    onUnhandledRequest: 'bypass',
    serviceWorker: {
      url: `${import.meta.env.BASE_URL}mockServiceWorker.js`,
    },
  });
}

enableMocking().then(() => {
  ReactDOM.createRoot(document.getElementById('root')).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>
  );
});
