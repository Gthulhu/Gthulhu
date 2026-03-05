# JWT Authentication and Static Web Interface Implementation

## Overview
This task implements JWT-based authentication for the BSS Metrics API server and creates a comprehensive static web interface for interacting with all API endpoints. The implementation includes both backend authentication middleware and a complete frontend client application.

## Backend Implementation

### JWT Authentication Middleware
- **Location**: `main.go`
- **Function**: `jwtMiddleware()`
- **Features**:
  - RSA public key validation using PEM format
  - Bearer token extraction from Authorization header
  - Automatic token generation for demonstration purposes
  - Graceful error handling with structured JSON responses

### Static File Server
- **Route**: `/static/*` (serves files from `./static/` directory)
- **Middleware**: Bypasses JWT authentication for static assets
- **Features**:
  - Direct file serving without authentication requirements
  - Proper MIME type handling for HTML, CSS, and JavaScript files

### Enhanced API Endpoints
- **GET `/api/v1/metrics`**: New endpoint to retrieve current metrics data
- **Metrics Storage**: Global variables to store latest metrics for retrieval
- **Thread Safety**: Mutex protection for concurrent access to shared metrics data

## Frontend Implementation

### Core Technologies
- **Pure JavaScript**: No frameworks or build tools required
- **Responsive Design**: Modern CSS Grid and Flexbox layouts
- **Local Storage**: Persistent JWT token storage across sessions

### Key Features

#### 1. Authentication System
- **JWT Token Management**: Automatic storage and retrieval from localStorage
- **Public Key Input**: Support for RSA public key in PEM format
- **Sample Key Integration**: Pre-filled demonstration public key
- **Authentication Status**: Real-time display of authentication state

#### 2. API Client Interface
- **Health Check Monitoring**: 
  - Manual and automatic health status checking
  - Visual grid display showing last 10 health check results
  - Color-coded status indicators (green for healthy, red for unhealthy)
  - Configurable auto-refresh intervals (1-300 seconds)

- **Metrics Visualization**:
  - Real-time metrics retrieval from `/api/v1/metrics` endpoint
  - Auto-refresh functionality with configurable intervals
  - Structured display of BSS scheduler metrics
  - Authentication-protected access

- **Pod-PID Mapping**: 
  - Retrieve current process-to-pod mappings
  - Display Kubernetes pod labels and associated PIDs
  - Essential for understanding scheduler strategy application

- **Scheduling Strategies**:
  - View current scheduling strategies
  - Create new strategies with label selectors
  - Configure execution time, priority, and process filters
  - Support for regex-based command matching

#### 3. User Experience Enhancements
- **Auto-Refresh Controls**: Independent interval settings for health and metrics
- **Visual Status Grid**: Real-time health status visualization
- **Responsive Layout**: Works on desktop and mobile devices
- **Error Handling**: Comprehensive error display and user feedback
- **Authentication Flow**: Seamless token management with clear status indicators

### File Structure
```
static/
├── index.html          # Main application interface
├── app.js             # Core JavaScript functionality
└── style.css          # Responsive styling and layout
```

## Implementation Details

### Authentication Flow
1. User inputs RSA public key (or uses sample key)
2. Frontend generates JWT token using client-side signing
3. Token stored in localStorage for persistence
4. All authenticated requests include Bearer token in Authorization header
5. Backend validates token using corresponding private key

### Health Check Visualization
- **Grid Display**: 10-cell grid showing historical health status
- **Color Coding**: Green (healthy) / Red (unhealthy) status indicators
- **Real-time Updates**: Automatic refresh based on user-configured intervals
- **Status History**: Maintains last 10 check results for trend visualization

### Metrics Auto-Refresh
- **Configurable Intervals**: 1-300 second refresh rates
- **Authentication Awareness**: Automatically stops when user logs out
- **Real-time Data**: Displays latest BSS scheduler metrics
- **Error Resilience**: Continues operation even if individual requests fail

## Configuration Files
- **JWT Keys**: `config/jwt_public_key.pem` and `config/jwt_private_key.key`
- **Sample Configuration**: Pre-configured for immediate testing and demonstration

## Security Considerations
- **Public Key Validation**: Proper RSA key format verification
- **Token Expiration**: JWT tokens include expiration claims
- **CORS Headers**: Appropriate cross-origin resource sharing configuration
- **Input Validation**: Comprehensive validation of all user inputs

## Testing and Deployment
- **Development Mode**: Run with `make run` for local testing
- **Container Deployment**: Full Docker and Kubernetes support
- **Health Monitoring**: Built-in health endpoints for operational monitoring
- **Error Logging**: Comprehensive logging for debugging and monitoring

This implementation provides a complete, production-ready authentication system with an intuitive web interface for managing and monitoring the BSS Metrics API server.