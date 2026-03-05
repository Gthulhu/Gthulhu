# Gthulhu React SPA

This is the React Single Page Application version of the Gthulhu eBPF Scheduler Control Interface.

## Project Structure

```
react-app/
├── public/
│   └── logo.png              # Logo image
├── src/
│   ├── components/
│   │   ├── cards/
│   │   │   ├── HealthCard.jsx
│   │   │   ├── UserProfileCard.jsx
│   │   │   ├── IntentsCard.jsx
│   │   │   ├── StrategiesCard.jsx
│   │   │   ├── UsersCard.jsx
│   │   │   ├── RolesCard.jsx
│   │   │   └── PodsCard.jsx
│   │   ├── modals/
│   │   │   ├── LoginModal.jsx
│   │   │   ├── ConfigModal.jsx
│   │   ├── Header.jsx
│   │   ├── Footer.jsx
│   │   ├── Dashboard.jsx
│   │   └── ToastContainer.jsx
│   ├── context/
│   │   └── AppContext.jsx     # Global state management
│   ├── styles/
│   │   └── index.css          # Original styles (fully preserved)
│   ├── App.jsx
│   └── main.jsx
├── index.html
├── package.json
├── vite.config.js
└── README.md
```

## Installation & Running

### Development Mode

```bash
# Navigate to the react-app directory
cd /home/ianchen0119/Gthulhu/api/web/static/react-app

# Install dependencies
npm install

# Start the development server
npm run dev
```

The development server will run at http://localhost:3000 and automatically proxy API requests to:
- Manager API: http://localhost:8080
- Decision Maker API: http://localhost:8081

### Production Build

```bash
# Build for production
npm run build

# Preview the build
npm run preview
```

## Compatibility with Original Version

This React version maintains full compatibility with the original HTML/JS version:
- Visual appearance (using the same CSS)
- Functional behavior (same API calls and state management)
- User experience (same interaction patterns)

## API Proxy Configuration

The API proxy for development is configured in `vite.config.js`:

```javascript
server: {
  port: 3000,
  proxy: {
    '/api': 'http://localhost:8080',
    '/health': 'http://localhost:8080',
    '/version': 'http://localhost:8080'
  }
}
```

To connect to a different backend, use the API Config feature in the application.

## Tech Stack

- React 18
- Vite 5
- Native CSS (preserving original styles)
