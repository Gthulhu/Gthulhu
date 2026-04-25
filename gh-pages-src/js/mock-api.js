/**
 * mock-api.js
 * Simulates the Gthulhu Manager REST API with realistic in-memory state.
 * All endpoints follow the same shape as the real API:
 *   { success: true, data: <T> }  or  { success: false, message: "..." }
 */

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
const delay = (ms = 300 + Math.random() * 200) =>
  new Promise((r) => setTimeout(r, ms));

let _tokenCounter = 0;
const makeToken = () =>
  "eyJhbGciOiJSUzI1NiJ9.demo." + btoa(JSON.stringify({ uid: "admin-uid", v: ++_tokenCounter, exp: Date.now() + 3_600_000 }));

const ok = (data) => ({ success: true, data });
const err = (message, status = 400) => ({ success: false, message, status });

// ---------------------------------------------------------------------------
// In-memory state
// ---------------------------------------------------------------------------
const state = {
  currentUser: null,
  token: null,
  refreshToken: null,

  users: [
    { id: "user-001", username: "admin", roles: ["admin"], status: 1 },
    { id: "user-002", username: "alice", roles: ["operator"], status: 1 },
    { id: "user-003", username: "bob", roles: ["viewer"], status: 2 },
  ],

  roles: [
    {
      id: "role-001",
      name: "admin",
      description: "Full system access",
      rolePolicy: [
        { permissionKey: "user.create", self: false, k8sNamespace: "", policyNamespace: "" },
        { permissionKey: "schedule_strategy.create", self: false, k8sNamespace: "*", policyNamespace: "*" },
        { permissionKey: "pod_scheduling_metrics.create", self: false, k8sNamespace: "*", policyNamespace: "*" },
        { permissionKey: "scheduler_config.update", self: false, k8sNamespace: "*", policyNamespace: "*" },
      ],
    },
    {
      id: "role-002",
      name: "operator",
      description: "Manage strategies and monitor metrics",
      rolePolicy: [
        { permissionKey: "schedule_strategy.create", self: true, k8sNamespace: "default", policyNamespace: "default" },
        { permissionKey: "pod_scheduling_metrics.read", self: false, k8sNamespace: "*", policyNamespace: "*" },
      ],
    },
    {
      id: "role-003",
      name: "viewer",
      description: "Read-only access",
      rolePolicy: [
        { permissionKey: "schedule_strategy.read", self: false, k8sNamespace: "*", policyNamespace: "*" },
        { permissionKey: "pod_scheduling_metrics.read", self: false, k8sNamespace: "*", policyNamespace: "*" },
      ],
    },
  ],

  permissions: [
    { key: "user.create", description: "Create users" },
    { key: "user.read", description: "Read users" },
    { key: "user.permission.update", description: "Update user permissions" },
    { key: "user.password.reset", description: "Reset user password" },
    { key: "user.password.change", description: "Change own password" },
    { key: "role.create", description: "Create roles" },
    { key: "role.read", description: "Read roles" },
    { key: "role.update", description: "Update roles" },
    { key: "role.delete", description: "Delete roles" },
    { key: "permission.read", description: "Read permissions" },
    { key: "schedule_strategy.create", description: "Create scheduling strategies" },
    { key: "schedule_strategy.read", description: "Read scheduling strategies" },
    { key: "schedule_strategy.update", description: "Update scheduling strategies" },
    { key: "schedule_strategy.delete", description: "Delete scheduling strategies" },
    { key: "schedule_intent.read", description: "Read scheduling intents" },
    { key: "schedule_intent.delete", description: "Delete scheduling intents" },
    { key: "pod_pid_mapping.read", description: "Read pod-PID mappings" },
    { key: "pod_scheduling_metrics.create", description: "Create pod scheduling metrics" },
    { key: "pod_scheduling_metrics.read", description: "Read pod scheduling metrics" },
    { key: "pod_scheduling_metrics.update", description: "Update pod scheduling metrics" },
    { key: "pod_scheduling_metrics.delete", description: "Delete pod scheduling metrics" },
    { key: "scheduler_config.read", description: "Read scheduler runtime config" },
    { key: "scheduler_config.update", description: "Update scheduler runtime config" },
  ],

  strategies: [
    {
      id: "strat-001",
      strategyNamespace: "trading",
      labelSelectors: [{ key: "app", value: "order-processor" }],
      k8sNamespace: ["trading"],
      commandRegex: ".*order-processor.*",
      priority: 2,
      executionTime: 5000000,
    },
    {
      id: "strat-002",
      strategyNamespace: "analytics",
      labelSelectors: [{ key: "app", value: "spark-worker" }, { key: "tier", value: "compute" }],
      k8sNamespace: ["analytics", "data"],
      commandRegex: ".*spark.*",
      priority: 1,
      executionTime: 10000000,
    },
    {
      id: "strat-003",
      strategyNamespace: "ml",
      labelSelectors: [{ key: "workload", value: "training" }],
      k8sNamespace: ["ml"],
      commandRegex: ".*python.*train.*",
      priority: 3,
      executionTime: 20000000,
    },
  ],

  intents: [
    { id: "intent-001", strategyId: "strat-001", podId: "order-processor-7f9b4-xk2ld", podName: "order-processor-7f9b4-xk2ld", nodeId: "node-1", k8sNamespace: "trading", commandRegex: ".*order-processor.*", priority: 2, executionTime: 5000000, podLabels: { app: "order-processor", version: "v2.1.0" }, state: 2 },
    { id: "intent-002", strategyId: "strat-001", podId: "order-processor-7f9b4-m8nqp", podName: "order-processor-7f9b4-m8nqp", nodeId: "node-2", k8sNamespace: "trading", commandRegex: ".*order-processor.*", priority: 2, executionTime: 5000000, podLabels: { app: "order-processor", version: "v2.1.0" }, state: 2 },
    { id: "intent-003", strategyId: "strat-002", podId: "spark-worker-0", podName: "spark-worker-0", nodeId: "node-1", k8sNamespace: "analytics", commandRegex: ".*spark.*", priority: 1, executionTime: 10000000, podLabels: { app: "spark-worker", tier: "compute" }, state: 1 },
    { id: "intent-004", strategyId: "strat-002", podId: "spark-worker-1", podName: "spark-worker-1", nodeId: "node-2", k8sNamespace: "analytics", commandRegex: ".*spark.*", priority: 1, executionTime: 10000000, podLabels: { app: "spark-worker", tier: "compute" }, state: 2 },
    { id: "intent-005", strategyId: "strat-003", podId: "train-job-abc12", podName: "train-job-abc12", nodeId: "node-1", k8sNamespace: "ml", commandRegex: ".*python.*train.*", priority: 3, executionTime: 20000000, podLabels: { workload: "training", framework: "pytorch" }, state: 1 },
  ],

  nodes: [
    { name: "node-1", status: "Online" },
    { name: "node-2", status: "Online" },
    { name: "node-3", status: "Offline" },
  ],

  podPIDMappings: {
    "node-1": {
      nodeName: "node-1",
      nodeId: "node-1",
      timestamp: new Date().toISOString(),
      pods: [
        { podUid: "pod-uid-001", podId: "order-processor-7f9b4-xk2ld", processes: [{ pid: 12341, command: "order-processor --port=8080", ppid: 1, containerId: "ctr-001a" }, { pid: 12342, command: "order-processor-metrics", ppid: 12341, containerId: "ctr-001a" }] },
        { podUid: "pod-uid-003", podId: "spark-worker-0", processes: [{ pid: 23451, command: "java -cp spark-worker.jar", ppid: 1, containerId: "ctr-003a" }, { pid: 23452, command: "java -cp spark-executor.jar", ppid: 23451, containerId: "ctr-003a" }] },
        { podUid: "pod-uid-005", podId: "train-job-abc12", processes: [{ pid: 34561, command: "python train.py --epochs=100", ppid: 1, containerId: "ctr-005a" }] },
        { podUid: "pod-uid-007", podId: "nginx-proxy-84b6f-p9ld2", processes: [{ pid: 45671, command: "nginx: master process", ppid: 1, containerId: "ctr-007a" }, { pid: 45672, command: "nginx: worker process", ppid: 45671, containerId: "ctr-007a" }] },
      ],
    },
    "node-2": {
      nodeName: "node-2",
      nodeId: "node-2",
      timestamp: new Date().toISOString(),
      pods: [
        { podUid: "pod-uid-002", podId: "order-processor-7f9b4-m8nqp", processes: [{ pid: 12391, command: "order-processor --port=8080", ppid: 1, containerId: "ctr-002a" }] },
        { podUid: "pod-uid-004", podId: "spark-worker-1", processes: [{ pid: 23501, command: "java -cp spark-worker.jar", ppid: 1, containerId: "ctr-004a" }, { pid: 23502, command: "java -cp spark-executor.jar", ppid: 23501, containerId: "ctr-004a" }] },
        { podUid: "pod-uid-008", podId: "postgres-main-0", processes: [{ pid: 56781, command: "postgres: checkpointer", ppid: 1, containerId: "ctr-008a" }, { pid: 56782, command: "postgres: background writer", ppid: 1, containerId: "ctr-008a" }] },
      ],
    },
  },

  psms: [
    {
      id: "psm-001",
      labelSelectors: [{ key: "app", value: "order-processor" }],
      k8sNamespaces: ["trading"],
      commandRegex: "",
      collectionIntervalSeconds: 5,
      enabled: true,
      metrics: { voluntaryCtxSwitches: true, involuntaryCtxSwitches: true, cpuTimeNs: true, waitTimeNs: true, runCount: true, cpuMigrations: false },
      scaling: null,
      createdTime: Date.now() - 86400000,
      updatedTime: Date.now() - 3600000,
    },
    {
      id: "psm-002",
      labelSelectors: [{ key: "app", value: "spark-worker" }, { key: "tier", value: "compute" }],
      k8sNamespaces: ["analytics"],
      commandRegex: ".*java.*spark.*",
      collectionIntervalSeconds: 10,
      enabled: true,
      metrics: { voluntaryCtxSwitches: true, involuntaryCtxSwitches: true, cpuTimeNs: true, waitTimeNs: true, runCount: true, cpuMigrations: true },
      scaling: { enabled: true, metricName: "spark_cpu_pressure", targetValue: "75", scaleTargetRef: { apiVersion: "apps/v1", kind: "StatefulSet", name: "spark-worker" }, minReplicaCount: 2, maxReplicaCount: 10, cooldownPeriod: 300 },
      createdTime: Date.now() - 172800000,
      updatedTime: Date.now() - 7200000,
    },
  ],

  runtimeMetrics: [
    { namespace: "trading", podName: "order-processor-7f9b4-xk2ld", nodeId: "node-1", voluntaryCtxSwitches: 1842, involuntaryCtxSwitches: 47, cpuTimeNs: 2834920000, waitTimeNs: 482930000, runCount: 9842, cpuMigrations: 12, smtMigrations: 3, l3Migrations: 8, numaMigrations: 0 },
    { namespace: "trading", podName: "order-processor-7f9b4-m8nqp", nodeId: "node-2", voluntaryCtxSwitches: 1634, involuntaryCtxSwitches: 63, cpuTimeNs: 2194830000, waitTimeNs: 631420000, runCount: 8391, cpuMigrations: 19, smtMigrations: 5, l3Migrations: 11, numaMigrations: 1 },
    { namespace: "analytics", podName: "spark-worker-0", nodeId: "node-1", voluntaryCtxSwitches: 5341, involuntaryCtxSwitches: 284, cpuTimeNs: 18293400000, waitTimeNs: 2938400000, runCount: 43921, cpuMigrations: 89, smtMigrations: 23, l3Migrations: 45, numaMigrations: 3 },
    { namespace: "analytics", podName: "spark-worker-1", nodeId: "node-2", voluntaryCtxSwitches: 4912, involuntaryCtxSwitches: 312, cpuTimeNs: 16482900000, waitTimeNs: 3128400000, runCount: 39812, cpuMigrations: 94, smtMigrations: 31, l3Migrations: 52, numaMigrations: 4 },
    { namespace: "ml", podName: "train-job-abc12", nodeId: "node-1", voluntaryCtxSwitches: 892, involuntaryCtxSwitches: 412, cpuTimeNs: 48293000000, waitTimeNs: 1293400000, runCount: 12834, cpuMigrations: 7, smtMigrations: 2, l3Migrations: 3, numaMigrations: 0 },
  ],

  schedulerConfig: {
    results: [
      { nodeId: "node-1", status: "Applied", config: { sliceNsDefault: 5000000, sliceNsMin: 500000, fifoScheduling: false } },
      { nodeId: "node-2", status: "Applied", config: { sliceNsDefault: 5000000, sliceNsMin: 500000, fifoScheduling: false } },
    ],
  },
};

// Passwords (demo only)
const PASSWORDS = { admin: "admin", alice: "alice123", bob: "bob123" };

// ---------------------------------------------------------------------------
// API methods
// ---------------------------------------------------------------------------

export const api = {
  // --- Auth ---
  async login(username, password) {
    await delay();
    const user = state.users.find((u) => u.username === username);
    if (!user || PASSWORDS[username] !== password)
      return err("Invalid username or password", 422);
    const token = makeToken();
    const refresh = "refresh." + makeToken();
    state.token = token;
    state.refreshToken = refresh;
    state.currentUser = user;
    return ok({ token, accessToken: token, refreshToken: refresh });
  },

  async logout() {
    await delay(150);
    state.token = null;
    state.refreshToken = null;
    state.currentUser = null;
    return ok({});
  },

  async validateToken() {
    await delay(150);
    if (!state.currentUser) return err("Unauthorized", 401);
    return ok({ valid: true, uid: state.currentUser.id, needChangePassword: false });
  },

  // --- Users ---
  async listUsers() {
    await delay();
    return ok({ users: state.users.map((u) => ({ id: u.id, username: u.username, roles: u.roles, status: u.status })) });
  },

  async getSelfUser() {
    await delay(150);
    if (!state.currentUser) return err("Unauthorized", 401);
    const u = state.currentUser;
    return ok({ id: u.id, username: u.username, roles: u.roles, status: u.status });
  },

  async createUser(username, password) {
    await delay();
    if (!username || !password) return err("Username and password are required");
    if (state.users.find((u) => u.username === username)) return err("Username already exists");
    const newUser = { id: "user-" + Date.now(), username, roles: [], status: 1 };
    state.users.push(newUser);
    PASSWORDS[username] = password;
    return ok({});
  },

  // --- Roles ---
  async listRoles() {
    await delay();
    return ok({ roles: state.roles });
  },

  async createRole(name, description, policies) {
    await delay();
    if (!name) return err("Role name is required");
    const newRole = { id: "role-" + Date.now(), name, description: description || "", rolePolicy: policies || [] };
    state.roles.push(newRole);
    return ok({});
  },

  async deleteRole(id) {
    await delay();
    const idx = state.roles.findIndex((r) => r.id === id);
    if (idx === -1) return err("Role not found", 404);
    state.roles.splice(idx, 1);
    return ok({});
  },

  async listPermissions() {
    await delay();
    return ok({ permissions: state.permissions });
  },

  // --- Strategies ---
  async listStrategies() {
    await delay();
    return ok({ strategies: [...state.strategies] });
  },

  async createStrategy(payload) {
    await delay();
    if (!payload.strategyNamespace) return err("Strategy namespace is required");
    const newStrategy = { id: "strat-" + Date.now(), ...payload };
    state.strategies.push(newStrategy);
    // Auto-generate mock intents
    const mockPodId = "pod-" + Math.random().toString(36).slice(2, 10);
    state.intents.push({
      id: "intent-" + Date.now(),
      strategyId: newStrategy.id,
      podId: mockPodId,
      podName: mockPodId,
      nodeId: "node-1",
      k8sNamespace: (payload.k8sNamespace || ["default"])[0],
      commandRegex: payload.commandRegex || "",
      priority: payload.priority || 0,
      executionTime: payload.executionTime || 5000000,
      podLabels: (payload.labelSelectors || []).reduce((a, ls) => { a[ls.key] = ls.value; return a; }, {}),
      state: 1,
    });
    return ok({});
  },

  async updateStrategy(strategyId, payload) {
    await delay();
    const idx = state.strategies.findIndex((s) => s.id === strategyId);
    if (idx === -1) return err("Strategy not found", 404);
    state.strategies[idx] = { ...state.strategies[idx], ...payload };
    return ok({});
  },

  async deleteStrategy(strategyId) {
    await delay();
    const idx = state.strategies.findIndex((s) => s.id === strategyId);
    if (idx === -1) return err("Strategy not found", 404);
    state.strategies.splice(idx, 1);
    state.intents = state.intents.filter((i) => i.strategyId !== strategyId);
    return ok({});
  },

  // --- Intents ---
  async listIntents() {
    await delay();
    return ok({ intents: [...state.intents] });
  },

  async deleteIntents(intentIds) {
    await delay();
    state.intents = state.intents.filter((i) => !intentIds.includes(i.id));
    return ok({});
  },

  // --- Nodes ---
  async listNodes() {
    await delay();
    return ok({ nodes: state.nodes });
  },

  async getNodePodPIDMapping(nodeId) {
    await delay();
    const mapping = state.podPIDMappings[nodeId];
    if (!mapping) return err("Node not found or offline", 404);
    return ok(mapping);
  },

  // --- PSM ---
  async listPSM() {
    await delay();
    return ok({ items: [...state.psms] });
  },

  async createPSM(payload) {
    await delay();
    if (!payload.labelSelectors || payload.labelSelectors.length === 0)
      return err("At least one label selector is required");
    const newPSM = {
      id: "psm-" + Date.now(),
      ...payload,
      enabled: payload.enabled !== undefined ? payload.enabled : true,
      collectionIntervalSeconds: payload.collectionIntervalSeconds || 10,
      createdTime: Date.now(),
      updatedTime: Date.now(),
    };
    state.psms.push(newPSM);
    return ok({});
  },

  async updatePSM(id, payload) {
    await delay();
    const idx = state.psms.findIndex((p) => p.id === id);
    if (idx === -1) return err("PSM not found", 404);
    state.psms[idx] = { ...state.psms[idx], ...payload, updatedTime: Date.now() };
    return ok({});
  },

  async deletePSM(id) {
    await delay();
    const idx = state.psms.findIndex((p) => p.id === id);
    if (idx === -1) return err("PSM not found", 404);
    state.psms.splice(idx, 1);
    return ok({});
  },

  // --- Runtime Metrics ---
  async listRuntimeMetrics() {
    await delay();
    // Add slight variance each call to simulate live data
    const items = state.runtimeMetrics.map((m) => ({
      ...m,
      voluntaryCtxSwitches: m.voluntaryCtxSwitches + Math.floor(Math.random() * 50),
      involuntaryCtxSwitches: m.involuntaryCtxSwitches + Math.floor(Math.random() * 10),
      cpuTimeNs: m.cpuTimeNs + Math.floor(Math.random() * 1_000_000),
      waitTimeNs: m.waitTimeNs + Math.floor(Math.random() * 100_000),
      runCount: m.runCount + Math.floor(Math.random() * 20),
    }));
    return ok({ items, warnings: [] });
  },

  // --- Scheduler Config ---
  async getSchedulerConfigStatus(nodeIds) {
    await delay();
    const results = nodeIds && nodeIds.length > 0
      ? state.schedulerConfig.results.filter((r) => nodeIds.includes(r.nodeId))
      : state.schedulerConfig.results;
    return ok({ results });
  },

  async applyRuntimeConfig(nodeIds, config) {
    await delay(600);
    const targetNodes = nodeIds && nodeIds.length > 0
      ? nodeIds
      : state.nodes.filter((n) => n.status === "Online").map((n) => n.name);
    const results = targetNodes.map((nodeId) => {
      const existing = state.schedulerConfig.results.find((r) => r.nodeId === nodeId);
      if (existing) {
        existing.config = { ...existing.config, ...config };
        existing.status = "Applied";
      } else {
        state.schedulerConfig.results.push({ nodeId, status: "Applied", config });
      }
      return { nodeId, status: "Applied", config };
    });
    return ok({ results });
  },

  // --- Helpers exposed to UI ---
  isLoggedIn: () => !!state.currentUser,
  getCurrentUser: () => state.currentUser,
  getToken: () => state.token,
};

export default api;
