/**
 * handlers.js
 * MSW v2 HTTP handlers that intercept every fetch() call the React app makes.
 * Each handler returns the exact JSON shape the corresponding React page expects.
 *
 * WARNING: For development / GitHub Pages demo only. Not for production use.
 */

import { http, HttpResponse } from 'msw';
import { state, PASSWORDS, makeToken } from './state.js';

// ---------------------------------------------------------------------------
// ID generator — guaranteed-unique within a browser session
// ---------------------------------------------------------------------------
let _idCounter = 0;
const makeId = (prefix) => `${prefix}-${++_idCounter}-${crypto.randomUUID().slice(0, 8)}`;

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------
const successResponse = (data) => HttpResponse.json({ success: true, data });
const errorResponse = (message, status = 400) =>
  HttpResponse.json({ success: false, message }, { status });

// ---------------------------------------------------------------------------
// Shape converters — camelCase internal state → wire format expected by pages
// ---------------------------------------------------------------------------

/** StrategiesPage.jsx reads PascalCase: s.ID, s.StrategyNamespace, … */
function toStrategy(s) {
  return {
    ID: s.id,
    StrategyNamespace: s.strategyNamespace,
    Priority: s.priority,
    ExecutionTime: s.executionTime,
    CommandRegex: s.commandRegex,
    K8sNamespace: s.k8sNamespace,
    LabelSelectors: s.labelSelectors,
  };
}

/** StrategiesPage.jsx reads PascalCase: intent.ID, intent.NodeID, intent.State, … */
function toIntent(i) {
  return {
    ID: i.id,
    StrategyID: i.strategyId,
    PodID: i.podId,
    PodName: i.podName,
    NodeID: i.nodeId,
    K8sNamespace: i.k8sNamespace,
    CommandRegex: i.commandRegex,
    Priority: i.priority,
    ExecutionTime: i.executionTime,
    State: i.state,
    PodLabels: i.podLabels,
  };
}

// ---------------------------------------------------------------------------
// Token → user lookup helper
// ---------------------------------------------------------------------------
function getCurrentUser(request) {
  const auth = request.headers.get('Authorization') || '';
  const token = auth.replace('Bearer ', '');
  return state.tokens[token] || null;
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------
export const handlers = [

  // ── Health ──────────────────────────────────────────────────────────────
  http.get('/health', () =>
    HttpResponse.json({ status: 'healthy', version: '1.0.0-mock', uptime: 99999 })
  ),

  // ── Auth ─────────────────────────────────────────────────────────────────
  http.post('/api/v1/auth/login', async ({ request }) => {
    const body = await request.json();
    const { username, password } = body || {};
    const user = state.users.find((u) => u.username === username);
    if (!user || PASSWORDS[username] !== password) {
      return errorResponse('Invalid username or password', 422);
    }
    const accessToken = makeToken();
    const refreshToken = 'refresh.' + makeToken();
    state.tokens[accessToken] = user;
    state.tokens[refreshToken] = user;
    return successResponse({ accessToken, refreshToken });
  }),

  http.post('/api/v1/auth/logout', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { refreshToken } = body || {};
    if (refreshToken) delete state.tokens[refreshToken];
    const auth = request.headers.get('Authorization') || '';
    const token = auth.replace('Bearer ', '');
    if (token) delete state.tokens[token];
    return successResponse({});
  }),

  http.post('/api/v1/auth/refresh', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { refreshToken } = body || {};
    const user = refreshToken ? state.tokens[refreshToken] : null;
    if (!user) return errorResponse('Invalid refresh token', 401);
    const newAccessToken = makeToken();
    state.tokens[newAccessToken] = user;
    return successResponse({ accessToken: newAccessToken, refreshToken });
  }),

  // ── Users ────────────────────────────────────────────────────────────────
  http.get('/api/v1/users', () =>
    successResponse({ users: state.users.map((u) => ({ id: u.id, username: u.username, roles: u.roles, status: u.status })) })
  ),

  http.post('/api/v1/users', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { username, password } = body || {};
    if (!username || !password) return errorResponse('Username and password are required');
    if (state.users.find((u) => u.username === username)) return errorResponse('Username already exists');
    const newUser = { id: makeId('user'), username, roles: [], status: 1 };
    state.users.push(newUser);
    PASSWORDS[username] = password;
    return successResponse({});
  }),

  http.get('/api/v1/users/self', ({ request }) => {
    const user = getCurrentUser(request);
    if (!user) return errorResponse('Unauthorized', 401);
    return successResponse({ id: user.id, username: user.username, roles: user.roles, status: user.status });
  }),

  http.put('/api/v1/users/permissions', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { userID, roles, status } = body || {};
    const idx = state.users.findIndex((u) => u.id === userID);
    if (idx === -1) return errorResponse('User not found', 404);
    if (roles !== undefined) state.users[idx].roles = roles;
    if (status !== undefined) state.users[idx].status = status;
    return successResponse({});
  }),

  http.put('/api/v1/users/password/reset', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { userID, newPassword } = body || {};
    const user = state.users.find((u) => u.id === userID);
    if (!user) return errorResponse('User not found', 404);
    if (!newPassword) return errorResponse('New password is required');
    PASSWORDS[user.username] = newPassword;
    return successResponse({});
  }),

  http.put('/api/v1/users/self/password', async ({ request }) => {
    const user = getCurrentUser(request);
    if (!user) return errorResponse('Unauthorized', 401);
    const body = await request.json().catch(() => ({}));
    const { oldPassword, newPassword } = body || {};
    if (PASSWORDS[user.username] !== oldPassword) return errorResponse('Current password is incorrect');
    if (!newPassword) return errorResponse('New password is required');
    PASSWORDS[user.username] = newPassword;
    return successResponse({});
  }),

  // ── Roles ────────────────────────────────────────────────────────────────
  http.get('/api/v1/roles', () =>
    successResponse({ roles: state.roles })
  ),

  http.post('/api/v1/roles', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { name, description, rolePolicies } = body || {};
    if (!name) return errorResponse('Role name is required');
    const newRole = { id: makeId('role'), name, description: description || '', rolePolicy: rolePolicies || [] };
    state.roles.push(newRole);
    return successResponse({});
  }),

  http.put('/api/v1/roles', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { id, name, description, rolePolicy } = body || {};
    const idx = state.roles.findIndex((r) => r.id === id);
    if (idx === -1) return errorResponse('Role not found', 404);
    if (name !== undefined) state.roles[idx].name = name;
    if (description !== undefined) state.roles[idx].description = description;
    if (rolePolicy !== undefined) state.roles[idx].rolePolicy = rolePolicy;
    return successResponse({});
  }),

  http.delete('/api/v1/roles', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { id } = body || {};
    const idx = state.roles.findIndex((r) => r.id === id);
    if (idx === -1) return errorResponse('Role not found', 404);
    state.roles.splice(idx, 1);
    return successResponse({});
  }),

  // ── Permissions ──────────────────────────────────────────────────────────
  http.get('/api/v1/permissions', () =>
    successResponse({ permissions: state.permissions })
  ),

  // ── Strategies ───────────────────────────────────────────────────────────
  http.get('/api/v1/strategies/self', () =>
    successResponse({ strategies: state.strategies.map(toStrategy) })
  ),

  http.post('/api/v1/strategies', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    if (!body.strategyNamespace) return errorResponse('Strategy namespace is required');
    const newStrategy = {
      id: makeId('strat'),
      strategyNamespace: body.strategyNamespace,
      labelSelectors: body.labelSelectors || [],
      k8sNamespace: body.k8sNamespace || [],
      commandRegex: body.commandRegex || '',
      priority: body.priority || 0,
      executionTime: body.executionTime || 5000000,
    };
    state.strategies.push(newStrategy);
    // Auto-generate a mock intent for the new strategy
    const mockPodId = 'pod-' + crypto.randomUUID().slice(0,8);
    state.intents.push({
      id: makeId('intent'),
      strategyId: newStrategy.id,
      podId: mockPodId,
      podName: mockPodId,
      nodeId: 'node-1',
      k8sNamespace: (newStrategy.k8sNamespace || ['default'])[0],
      commandRegex: newStrategy.commandRegex,
      priority: newStrategy.priority,
      executionTime: newStrategy.executionTime,
      podLabels: (newStrategy.labelSelectors || []).reduce((a, ls) => { a[ls.key] = ls.value; return a; }, {}),
      state: 1,
    });
    return successResponse({});
  }),

  http.put('/api/v1/strategies', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { strategyId, ...updates } = body || {};
    const idx = state.strategies.findIndex((s) => s.id === strategyId);
    if (idx === -1) return errorResponse('Strategy not found', 404);
    state.strategies[idx] = { ...state.strategies[idx], ...updates };
    return successResponse({});
  }),

  http.delete('/api/v1/strategies', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { strategyId } = body || {};
    const idx = state.strategies.findIndex((s) => s.id === strategyId);
    if (idx === -1) return errorResponse('Strategy not found', 404);
    state.strategies.splice(idx, 1);
    state.intents = state.intents.filter((i) => i.strategyId !== strategyId);
    return successResponse({});
  }),

  // ── Intents ──────────────────────────────────────────────────────────────
  http.get('/api/v1/intents/self', () =>
    successResponse({ intents: state.intents.map(toIntent) })
  ),

  http.delete('/api/v1/intents', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const intentIds = Array.isArray(body) ? body : (body.intentIds || []);
    state.intents = state.intents.filter((i) => !intentIds.includes(i.id));
    return successResponse({});
  }),

  // ── Nodes ────────────────────────────────────────────────────────────────
  http.get('/api/v1/nodes', () =>
    successResponse({ nodes: state.nodes })
  ),

  http.get('/api/v1/nodes/:nodeId/pods/pids', ({ params }) => {
    const mapping = state.podPIDMappings[params.nodeId];
    if (!mapping) return errorResponse('Node not found or offline', 404);
    return successResponse(mapping);
  }),

  // ── Scheduler Runtime Config ─────────────────────────────────────────────
  http.get('/api/v1/scheduler/runtime-config/status', ({ request }) => {
    const url = new URL(request.url);
    const nodeIdsParam = url.searchParams.get('nodeIds');
    const nodeIds = nodeIdsParam ? nodeIdsParam.split(',').map((s) => s.trim()).filter(Boolean) : [];
    const results = nodeIds.length
      ? state.schedulerConfig.results.filter((r) => nodeIds.includes(r.nodeId))
      : state.schedulerConfig.results;
    return successResponse({ results });
  }),

  http.post('/api/v1/scheduler/runtime-config/apply', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { nodeIds = [], config = {} } = body || {};
    const targetNodeIds = nodeIds.length
      ? nodeIds
      : state.nodes.filter((n) => n.status === 'Ready').map((n) => n.name);

    const results = targetNodeIds.map((nodeId) => {
      const existing = state.schedulerConfig.results.find((r) => r.nodeId === nodeId);
      const merged = { ...config };
      if (existing) {
        existing.config = { ...existing.config, ...merged };
        existing.configVersion = merged.configVersion || new Date().toISOString();
        existing.appliedAt = new Date().toISOString();
        existing.success = true;
        existing.lastError = null;
        return existing;
      }
      const newResult = {
        nodeId,
        host: nodeId + '.cluster.local',
        success: true,
        lastError: null,
        configVersion: merged.configVersion || new Date().toISOString(),
        appliedAt: new Date().toISOString(),
        restartCount: 0,
        config: { mode: 'gthulhu', schedulerEnabled: true, monitoringEnabled: true, sliceNsDefault: 20000000, sliceNsMin: 1000000, kernelMode: true, maxTimeWatchdog: true, earlyProcessing: false, builtinIdle: false, ...merged },
      };
      state.schedulerConfig.results.push(newResult);
      return newResult;
    });
    return successResponse({ results });
  }),

  // ── Pod Scheduling Metrics ────────────────────────────────────────────────
  http.get('/api/v1/pod-scheduling-metrics', () =>
    successResponse({ items: [...state.psms] })
  ),

  http.post('/api/v1/pod-scheduling-metrics', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    if (!body.labelSelectors || body.labelSelectors.length === 0) {
      return errorResponse('At least one label selector is required');
    }
    const newPSM = {
      id: makeId('psm'),
      labelSelectors: body.labelSelectors,
      k8sNamespaces: body.k8sNamespaces || [],
      commandRegex: body.commandRegex || '',
      collectionIntervalSeconds: body.collectionIntervalSeconds || 10,
      enabled: body.enabled !== undefined ? body.enabled : true,
      metrics: body.metrics || {},
      scaling: body.scaling || null,
      createdTime: Date.now(),
      updatedTime: Date.now(),
    };
    state.psms.push(newPSM);
    return successResponse({});
  }),

  http.put('/api/v1/pod-scheduling-metrics', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { id, ...updates } = body || {};
    const idx = state.psms.findIndex((p) => p.id === id);
    if (idx === -1) return errorResponse('PSM not found', 404);
    state.psms[idx] = { ...state.psms[idx], ...updates, updatedTime: Date.now() };
    return successResponse({});
  }),

  http.delete('/api/v1/pod-scheduling-metrics', async ({ request }) => {
    const body = await request.json().catch(() => ({}));
    const { id } = body || {};
    const idx = state.psms.findIndex((p) => p.id === id);
    if (idx === -1) return errorResponse('PSM not found', 404);
    state.psms.splice(idx, 1);
    return successResponse({});
  }),

  http.get('/api/v1/pod-scheduling-metrics/runtime', () => {
    const items = state.runtimeMetrics.map((m) => ({
      ...m,
      voluntaryCtxSwitches: m.voluntaryCtxSwitches + Math.floor(Math.random() * 50),
      involuntaryCtxSwitches: m.involuntaryCtxSwitches + Math.floor(Math.random() * 10),
      cpuTimeNs: m.cpuTimeNs + Math.floor(Math.random() * 1_000_000),
      waitTimeNs: m.waitTimeNs + Math.floor(Math.random() * 100_000),
      runCount: m.runCount + Math.floor(Math.random() * 20),
    }));
    return successResponse({ items, warnings: [] });
  }),

  // ── Classify ──────────────────────────────────────────────────────────────
  http.get('/api/v1/classify/:namespace/:pod', ({ params }) => {
    const item = state.classifyItems.find(
      (c) => c.namespace === params.namespace && c.pod === params.pod
    );
    if (!item) return errorResponse('Not found', 404);
    return successResponse(item);
  }),

  http.get('/api/v1/classify', ({ request }) => {
    const url = new URL(request.url);
    const ns = url.searchParams.get('namespace') || '';
    const phase = url.searchParams.get('phase') || '';
    const type = url.searchParams.get('type') || '';
    let items = state.classifyItems;
    if (ns) items = items.filter((c) => c.namespace.includes(ns));
    if (phase) items = items.filter((c) => c.phase === phase);
    if (type) items = items.filter((c) => (c.classification?.current_type || []).includes(type));
    return successResponse({ items });
  }),
];
