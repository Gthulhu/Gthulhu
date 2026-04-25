/**
 * app.js
 * Alpine.js application data – wires UI state to mock-api calls.
 */

import api from "./mock-api.js";

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------
const fmt = {
  ns: (ns) => {
    if (!ns) return "0 ms";
    if (ns < 1_000) return ns + " ns";
    if (ns < 1_000_000) return (ns / 1_000).toFixed(2) + " µs";
    if (ns < 1_000_000_000) return (ns / 1_000_000).toFixed(2) + " ms";
    return (ns / 1_000_000_000).toFixed(3) + " s";
  },
  num: (n) => (n ?? 0).toLocaleString(),
  intentState: (s) => ({ 0: "Unknown", 1: "Initialized", 2: "Sent" }[s] ?? "Unknown"),
  intentStateColor: (s) => ({ 0: "gray", 1: "yellow", 2: "green" }[s] ?? "gray"),
  userStatus: (s) => ({ 1: "Active", 2: "Inactive", 3: "Pending Password" }[s] ?? "Unknown"),
  userStatusColor: (s) => ({ 1: "green", 2: "red", 3: "yellow" }[s] ?? "gray"),
  priority: (p) => ({ 0: "Normal", 1: "Low", 2: "High", 3: "Critical" }[p] ?? `P${p}`),
  priorityColor: (p) => ({ 0: "gray", 1: "blue", 2: "orange", 3: "red" }[p] ?? "gray"),
  truncate: (s, n = 24) => (s && s.length > n ? s.slice(0, n) + "…" : s),
};

window.fmt = fmt; // expose to Alpine templates

// ---------------------------------------------------------------------------
// Toast helper (used via window)
// ---------------------------------------------------------------------------
function createToastStore() {
  return {
    toasts: [],
    add(message, type = "info") {
      const id = Date.now();
      this.toasts.push({ id, message, type });
      setTimeout(() => this.remove(id), 4000);
    },
    remove(id) {
      this.toasts = this.toasts.filter((t) => t.id !== id);
    },
  };
}

// ---------------------------------------------------------------------------
// Root app data
// ---------------------------------------------------------------------------
function appData() {
  return {
    // Layout
    page: "login",
    sidebarOpen: true,
    loadingPage: false,

    // Auth
    loginUsername: "admin",
    loginPassword: "admin",
    loginError: "",
    loginLoading: false,
    isLoggedIn: false,
    currentUser: null,

    // Toast
    toasts: [],

    // Page-level data containers
    users: [],
    roles: [],
    permissions: [],
    strategies: [],
    intents: [],
    nodes: [],
    selectedNode: null,
    podPIDMapping: null,
    podPIDLoading: false,
    psms: [],
    runtimeMetrics: [],
    runtimeMetricsChart: null,
    schedulerConfigStatus: [],

    // Forms
    showCreateStrategy: false,
    showCreatePSM: false,
    showCreateUser: false,
    showCreateRole: false,
    showSchedulerConfigForm: false,

    newStrategy: { strategyNamespace: "", labelSelectors: [{ key: "", value: "" }], k8sNamespace: "", commandRegex: "", priority: 0, executionTime: 5000000 },
    newPSM: { labelSelectors: [{ key: "", value: "" }], k8sNamespaces: "", commandRegex: "", collectionIntervalSeconds: 10, enabled: true, metrics: { voluntaryCtxSwitches: true, involuntaryCtxSwitches: true, cpuTimeNs: true, waitTimeNs: false, runCount: true, cpuMigrations: false } },
    newUser: { username: "", password: "" },
    newRole: { name: "", description: "" },
    schedulerConfigForm: { sliceNsDefault: 5000000, sliceNsMin: 500000, fifoScheduling: false, nodeIds: "" },

    // ---------------------------------------------------------------------------
    // Init
    // ---------------------------------------------------------------------------
    init() {
      if (api.isLoggedIn()) {
        this.isLoggedIn = true;
        this.currentUser = api.getCurrentUser();
        this.navigate("dashboard");
      }
    },

    // ---------------------------------------------------------------------------
    // Toast
    // ---------------------------------------------------------------------------
    addToast(message, type = "info") {
      const id = Date.now();
      this.toasts.push({ id, message, type });
      setTimeout(() => { this.toasts = this.toasts.filter((t) => t.id !== id); }, 4000);
    },

    // ---------------------------------------------------------------------------
    // Auth
    // ---------------------------------------------------------------------------
    async login() {
      this.loginError = "";
      this.loginLoading = true;
      try {
        const res = await api.login(this.loginUsername, this.loginPassword);
        if (!res.success) {
          this.loginError = res.message;
          return;
        }
        this.isLoggedIn = true;
        this.currentUser = api.getCurrentUser();
        this.navigate("dashboard");
        this.addToast(`Welcome back, ${this.currentUser.username}!`, "success");
      } finally {
        this.loginLoading = false;
      }
    },

    async logout() {
      await api.logout();
      this.isLoggedIn = false;
      this.currentUser = null;
      this.page = "login";
    },

    // ---------------------------------------------------------------------------
    // Navigation
    // ---------------------------------------------------------------------------
    async navigate(p) {
      this.page = p;
      this.loadingPage = true;
      try {
        if (p === "dashboard") await this.loadDashboard();
        else if (p === "nodes") await this.loadNodes();
        else if (p === "strategies") await this.loadStrategies();
        else if (p === "intents") await this.loadIntents();
        else if (p === "psm") await this.loadPSM();
        else if (p === "metrics") await this.loadRuntimeMetrics();
        else if (p === "users") await this.loadUsers();
        else if (p === "roles") await this.loadRoles();
        else if (p === "scheduler") await this.loadSchedulerConfig();
      } finally {
        this.loadingPage = false;
      }
    },

    // ---------------------------------------------------------------------------
    // Dashboard
    // ---------------------------------------------------------------------------
    dashboardStats: { nodes: 0, onlineNodes: 0, strategies: 0, intents: 0, psms: 0, monitoredPods: 0 },

    async loadDashboard() {
      const [nodesRes, strategiesRes, intentsRes, psmsRes, metricsRes] = await Promise.all([
        api.listNodes(), api.listStrategies(), api.listIntents(), api.listPSM(), api.listRuntimeMetrics(),
      ]);
      this.nodes = nodesRes.data?.nodes ?? [];
      this.strategies = strategiesRes.data?.strategies ?? [];
      this.intents = intentsRes.data?.intents ?? [];
      this.psms = psmsRes.data?.items ?? [];
      this.runtimeMetrics = metricsRes.data?.items ?? [];
      this.dashboardStats = {
        nodes: this.nodes.length,
        onlineNodes: this.nodes.filter((n) => n.status === "Online").length,
        strategies: this.strategies.length,
        intents: this.intents.length,
        psms: this.psms.length,
        monitoredPods: this.runtimeMetrics.length,
      };
    },

    // ---------------------------------------------------------------------------
    // Nodes
    // ---------------------------------------------------------------------------
    async loadNodes() {
      const res = await api.listNodes();
      this.nodes = res.data?.nodes ?? [];
      this.selectedNode = null;
      this.podPIDMapping = null;
    },

    async selectNode(nodeId) {
      this.selectedNode = nodeId;
      this.podPIDLoading = true;
      this.podPIDMapping = null;
      try {
        const res = await api.getNodePodPIDMapping(nodeId);
        if (res.success) this.podPIDMapping = res.data;
        else this.addToast(res.message, "error");
      } finally {
        this.podPIDLoading = false;
      }
    },

    // ---------------------------------------------------------------------------
    // Strategies
    // ---------------------------------------------------------------------------
    async loadStrategies() {
      const res = await api.listStrategies();
      this.strategies = res.data?.strategies ?? [];
    },

    async createStrategy() {
      const payload = {
        strategyNamespace: this.newStrategy.strategyNamespace,
        labelSelectors: this.newStrategy.labelSelectors.filter((ls) => ls.key),
        k8sNamespace: this.newStrategy.k8sNamespace ? this.newStrategy.k8sNamespace.split(",").map((s) => s.trim()) : [],
        commandRegex: this.newStrategy.commandRegex,
        priority: Number(this.newStrategy.priority),
        executionTime: Number(this.newStrategy.executionTime),
      };
      const res = await api.createStrategy(payload);
      if (res.success) {
        this.addToast("Strategy created successfully", "success");
        this.showCreateStrategy = false;
        this.newStrategy = { strategyNamespace: "", labelSelectors: [{ key: "", value: "" }], k8sNamespace: "", commandRegex: "", priority: 0, executionTime: 5000000 };
        await this.loadStrategies();
      } else {
        this.addToast(res.message, "error");
      }
    },

    async deleteStrategy(id) {
      if (!confirm("Delete this strategy and all its intents?")) return;
      const res = await api.deleteStrategy(id);
      if (res.success) {
        this.addToast("Strategy deleted", "success");
        await this.loadStrategies();
      } else {
        this.addToast(res.message, "error");
      }
    },

    addLabelSelector(target) {
      this[target].labelSelectors.push({ key: "", value: "" });
    },

    removeLabelSelector(target, idx) {
      this[target].labelSelectors.splice(idx, 1);
    },

    // ---------------------------------------------------------------------------
    // Intents
    // ---------------------------------------------------------------------------
    async loadIntents() {
      const res = await api.listIntents();
      this.intents = res.data?.intents ?? [];
    },

    async deleteIntent(id) {
      if (!confirm("Delete this intent?")) return;
      const res = await api.deleteIntents([id]);
      if (res.success) {
        this.addToast("Intent deleted", "success");
        await this.loadIntents();
      } else {
        this.addToast(res.message, "error");
      }
    },

    // ---------------------------------------------------------------------------
    // PSM
    // ---------------------------------------------------------------------------
    async loadPSM() {
      const res = await api.listPSM();
      this.psms = res.data?.items ?? [];
    },

    async createPSM() {
      const payload = {
        labelSelectors: this.newPSM.labelSelectors.filter((ls) => ls.key),
        k8sNamespaces: this.newPSM.k8sNamespaces ? this.newPSM.k8sNamespaces.split(",").map((s) => s.trim()) : [],
        commandRegex: this.newPSM.commandRegex,
        collectionIntervalSeconds: Number(this.newPSM.collectionIntervalSeconds),
        enabled: this.newPSM.enabled,
        metrics: { ...this.newPSM.metrics },
      };
      const res = await api.createPSM(payload);
      if (res.success) {
        this.addToast("PSM config created", "success");
        this.showCreatePSM = false;
        await this.loadPSM();
      } else {
        this.addToast(res.message, "error");
      }
    },

    async deletePSM(id) {
      if (!confirm("Delete this PSM configuration?")) return;
      const res = await api.deletePSM(id);
      if (res.success) {
        this.addToast("PSM config deleted", "success");
        await this.loadPSM();
      } else {
        this.addToast(res.message, "error");
      }
    },

    // ---------------------------------------------------------------------------
    // Runtime Metrics
    // ---------------------------------------------------------------------------
    metricsRefreshInterval: null,

    async loadRuntimeMetrics() {
      const res = await api.listRuntimeMetrics();
      this.runtimeMetrics = res.data?.items ?? [];
      this.$nextTick?.(() => this.renderMetricsChart());
    },

    async refreshMetrics() {
      this.loadingPage = true;
      await this.loadRuntimeMetrics();
      this.loadingPage = false;
      this.addToast("Metrics refreshed", "info");
    },

    renderMetricsChart() {
      const canvas = document.getElementById("metricsChart");
      if (!canvas) return;
      if (this.runtimeMetricsChart) {
        this.runtimeMetricsChart.destroy();
        this.runtimeMetricsChart = null;
      }
      const labels = this.runtimeMetrics.map((m) => fmt.truncate(`${m.namespace}/${m.podName}`, 28));
      const cpuData = this.runtimeMetrics.map((m) => +(m.cpuTimeNs / 1e9).toFixed(3));
      const waitData = this.runtimeMetrics.map((m) => +(m.waitTimeNs / 1e9).toFixed(3));
      // eslint-disable-next-line no-undef
      this.runtimeMetricsChart = new Chart(canvas, {
        type: "bar",
        data: {
          labels,
          datasets: [
            { label: "CPU Time (s)", data: cpuData, backgroundColor: "rgba(99,102,241,0.8)", borderRadius: 4 },
            { label: "Wait Time (s)", data: waitData, backgroundColor: "rgba(251,146,60,0.8)", borderRadius: 4 },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: { legend: { position: "top" } },
          scales: { y: { beginAtZero: true, title: { display: true, text: "Seconds" } } },
        },
      });
    },

    // ---------------------------------------------------------------------------
    // Users
    // ---------------------------------------------------------------------------
    async loadUsers() {
      const res = await api.listUsers();
      this.users = res.data?.users ?? [];
    },

    async createUser() {
      const res = await api.createUser(this.newUser.username, this.newUser.password);
      if (res.success) {
        this.addToast("User created", "success");
        this.showCreateUser = false;
        this.newUser = { username: "", password: "" };
        await this.loadUsers();
      } else {
        this.addToast(res.message, "error");
      }
    },

    // ---------------------------------------------------------------------------
    // Roles
    // ---------------------------------------------------------------------------
    async loadRoles() {
      const [rolesRes, permsRes] = await Promise.all([api.listRoles(), api.listPermissions()]);
      this.roles = rolesRes.data?.roles ?? [];
      this.permissions = permsRes.data?.permissions ?? [];
    },

    async createRole() {
      const res = await api.createRole(this.newRole.name, this.newRole.description, []);
      if (res.success) {
        this.addToast("Role created", "success");
        this.showCreateRole = false;
        this.newRole = { name: "", description: "" };
        await this.loadRoles();
      } else {
        this.addToast(res.message, "error");
      }
    },

    async deleteRole(id) {
      if (!confirm("Delete this role?")) return;
      const res = await api.deleteRole(id);
      if (res.success) {
        this.addToast("Role deleted", "success");
        await this.loadRoles();
      } else {
        this.addToast(res.message, "error");
      }
    },

    // ---------------------------------------------------------------------------
    // Scheduler Config
    // ---------------------------------------------------------------------------
    async loadSchedulerConfig() {
      const res = await api.getSchedulerConfigStatus([]);
      this.schedulerConfigStatus = res.data?.results ?? [];
    },

    async applySchedulerConfig() {
      const nodeIds = this.schedulerConfigForm.nodeIds
        ? this.schedulerConfigForm.nodeIds.split(",").map((s) => s.trim()).filter(Boolean)
        : [];
      const config = {
        sliceNsDefault: Number(this.schedulerConfigForm.sliceNsDefault),
        sliceNsMin: Number(this.schedulerConfigForm.sliceNsMin),
        fifoScheduling: this.schedulerConfigForm.fifoScheduling,
      };
      const res = await api.applyRuntimeConfig(nodeIds, config);
      if (res.success) {
        this.addToast("Runtime config applied successfully", "success");
        this.showSchedulerConfigForm = false;
        await this.loadSchedulerConfig();
      } else {
        this.addToast(res.message, "error");
      }
    },
  };
}

// Register with Alpine when it loads
document.addEventListener("alpine:init", () => {
  // eslint-disable-next-line no-undef
  Alpine.data("app", appData);
});
