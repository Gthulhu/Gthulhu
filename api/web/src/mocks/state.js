/**
 * state.js
 * Shared in-memory mock state for MSW handlers.
 * Data shapes mirror the real Go API responses.
 *
 * WARNING: This file is for development / GitHub Pages demo purposes only.
 * It contains hard-coded demo credentials and MUST NOT be used in production.
 * Mock mode is gated by the VITE_USE_MOCK=true environment variable; it is
 * never active in a normal production build.
 */

export const state = {
  // token → user mapping for auth
  tokens: {},

  users: [
    { id: 'user-001', username: 'admin', roles: ['admin'], status: 1 },
    { id: 'user-002', username: 'alice', roles: ['operator'], status: 1 },
    { id: 'user-003', username: 'bob', roles: ['viewer'], status: 2 },
  ],

  roles: [
    {
      id: 'role-001',
      name: 'admin',
      description: 'Full system access',
      rolePolicy: [
        { permissionKey: 'user.create', self: false, k8sNamespace: '', policyNamespace: '' },
        { permissionKey: 'schedule_strategy.create', self: false, k8sNamespace: '*', policyNamespace: '*' },
        { permissionKey: 'pod_scheduling_metrics.create', self: false, k8sNamespace: '*', policyNamespace: '*' },
        { permissionKey: 'scheduler_config.update', self: false, k8sNamespace: '*', policyNamespace: '*' },
      ],
    },
    {
      id: 'role-002',
      name: 'operator',
      description: 'Manage strategies and monitor metrics',
      rolePolicy: [
        { permissionKey: 'schedule_strategy.create', self: true, k8sNamespace: 'default', policyNamespace: 'default' },
        { permissionKey: 'pod_scheduling_metrics.read', self: false, k8sNamespace: '*', policyNamespace: '*' },
      ],
    },
    {
      id: 'role-003',
      name: 'viewer',
      description: 'Read-only access',
      rolePolicy: [
        { permissionKey: 'schedule_strategy.read', self: false, k8sNamespace: '*', policyNamespace: '*' },
        { permissionKey: 'pod_scheduling_metrics.read', self: false, k8sNamespace: '*', policyNamespace: '*' },
      ],
    },
  ],

  permissions: [
    { key: 'user.create', description: 'Create users' },
    { key: 'user.read', description: 'Read users' },
    { key: 'user.permission.update', description: 'Update user permissions' },
    { key: 'user.password.reset', description: 'Reset user password' },
    { key: 'user.password.change', description: 'Change own password' },
    { key: 'role.create', description: 'Create roles' },
    { key: 'role.read', description: 'Read roles' },
    { key: 'role.update', description: 'Update roles' },
    { key: 'role.delete', description: 'Delete roles' },
    { key: 'permission.read', description: 'Read permissions' },
    { key: 'schedule_strategy.create', description: 'Create scheduling strategies' },
    { key: 'schedule_strategy.read', description: 'Read scheduling strategies' },
    { key: 'schedule_strategy.update', description: 'Update scheduling strategies' },
    { key: 'schedule_strategy.delete', description: 'Delete scheduling strategies' },
    { key: 'schedule_intent.read', description: 'Read scheduling intents' },
    { key: 'schedule_intent.delete', description: 'Delete scheduling intents' },
    { key: 'pod_pid_mapping.read', description: 'Read pod-PID mappings' },
    { key: 'pod_scheduling_metrics.create', description: 'Create pod scheduling metrics' },
    { key: 'pod_scheduling_metrics.read', description: 'Read pod scheduling metrics' },
    { key: 'pod_scheduling_metrics.update', description: 'Update pod scheduling metrics' },
    { key: 'pod_scheduling_metrics.delete', description: 'Delete pod scheduling metrics' },
    { key: 'scheduler_config.read', description: 'Read scheduler runtime config' },
    { key: 'scheduler_config.update', description: 'Update scheduler runtime config' },
  ],

  // Stored with camelCase internally; handlers emit PascalCase for the Go-backend contract
  strategies: [
    {
      id: 'strat-001',
      strategyNamespace: 'trading',
      labelSelectors: [{ key: 'app', value: 'order-processor' }],
      k8sNamespace: ['trading'],
      commandRegex: '.*order-processor.*',
      priority: 2,
      executionTime: 5000000,
    },
    {
      id: 'strat-002',
      strategyNamespace: 'analytics',
      labelSelectors: [{ key: 'app', value: 'spark-worker' }, { key: 'tier', value: 'compute' }],
      k8sNamespace: ['analytics', 'data'],
      commandRegex: '.*spark.*',
      priority: 1,
      executionTime: 10000000,
    },
    {
      id: 'strat-003',
      strategyNamespace: 'ml',
      labelSelectors: [{ key: 'workload', value: 'training' }],
      k8sNamespace: ['ml'],
      commandRegex: '.*python.*train.*',
      priority: 3,
      executionTime: 20000000,
    },
  ],

  intents: [
    { id: 'intent-001', strategyId: 'strat-001', podId: 'order-processor-7f9b4-xk2ld', podName: 'order-processor-7f9b4-xk2ld', nodeId: 'node-1', k8sNamespace: 'trading', commandRegex: '.*order-processor.*', priority: 2, executionTime: 5000000, podLabels: { app: 'order-processor', version: 'v2.1.0' }, state: 2 },
    { id: 'intent-002', strategyId: 'strat-001', podId: 'order-processor-7f9b4-m8nqp', podName: 'order-processor-7f9b4-m8nqp', nodeId: 'node-2', k8sNamespace: 'trading', commandRegex: '.*order-processor.*', priority: 2, executionTime: 5000000, podLabels: { app: 'order-processor', version: 'v2.1.0' }, state: 2 },
    { id: 'intent-003', strategyId: 'strat-002', podId: 'spark-worker-0', podName: 'spark-worker-0', nodeId: 'node-1', k8sNamespace: 'analytics', commandRegex: '.*spark.*', priority: 1, executionTime: 10000000, podLabels: { app: 'spark-worker', tier: 'compute' }, state: 1 },
    { id: 'intent-004', strategyId: 'strat-002', podId: 'spark-worker-1', podName: 'spark-worker-1', nodeId: 'node-2', k8sNamespace: 'analytics', commandRegex: '.*spark.*', priority: 1, executionTime: 10000000, podLabels: { app: 'spark-worker', tier: 'compute' }, state: 2 },
    { id: 'intent-005', strategyId: 'strat-003', podId: 'train-job-abc12', podName: 'train-job-abc12', nodeId: 'node-1', k8sNamespace: 'ml', commandRegex: '.*python.*train.*', priority: 3, executionTime: 20000000, podLabels: { workload: 'training', framework: 'pytorch' }, state: 1 },
  ],

  // status "Ready" / "NotReady" matches what NodesPage.jsx expects
  nodes: [
    { name: 'node-1', status: 'Ready' },
    { name: 'node-2', status: 'Ready' },
    { name: 'node-3', status: 'NotReady' },
  ],

  // snake_case pod/process keys match what NodeDetailPage.jsx expects
  podPIDMappings: {
    'node-1': {
      node_name: 'node-1',
      node_id: 'node-1',
      timestamp: new Date().toISOString(),
      pods: [
        {
          pod_uid: 'pod-uid-001',
          pod_id: 'order-processor-7f9b4-xk2ld',
          processes: [
            { pid: 12341, command: 'order-processor --port=8080', ppid: 1, container_id: 'ctr-001a' },
            { pid: 12342, command: 'order-processor-metrics', ppid: 12341, container_id: 'ctr-001a' },
          ],
        },
        {
          pod_uid: 'pod-uid-003',
          pod_id: 'spark-worker-0',
          processes: [
            { pid: 23451, command: 'java -cp spark-worker.jar', ppid: 1, container_id: 'ctr-003a' },
            { pid: 23452, command: 'java -cp spark-executor.jar', ppid: 23451, container_id: 'ctr-003a' },
          ],
        },
        {
          pod_uid: 'pod-uid-005',
          pod_id: 'train-job-abc12',
          processes: [
            { pid: 34561, command: 'python train.py --epochs=100', ppid: 1, container_id: 'ctr-005a' },
          ],
        },
        {
          pod_uid: 'pod-uid-007',
          pod_id: 'nginx-proxy-84b6f-p9ld2',
          processes: [
            { pid: 45671, command: 'nginx: master process', ppid: 1, container_id: 'ctr-007a' },
            { pid: 45672, command: 'nginx: worker process', ppid: 45671, container_id: 'ctr-007a' },
          ],
        },
      ],
    },
    'node-2': {
      node_name: 'node-2',
      node_id: 'node-2',
      timestamp: new Date().toISOString(),
      pods: [
        {
          pod_uid: 'pod-uid-002',
          pod_id: 'order-processor-7f9b4-m8nqp',
          processes: [
            { pid: 12391, command: 'order-processor --port=8080', ppid: 1, container_id: 'ctr-002a' },
          ],
        },
        {
          pod_uid: 'pod-uid-004',
          pod_id: 'spark-worker-1',
          processes: [
            { pid: 23501, command: 'java -cp spark-worker.jar', ppid: 1, container_id: 'ctr-004a' },
            { pid: 23502, command: 'java -cp spark-executor.jar', ppid: 23501, container_id: 'ctr-004a' },
          ],
        },
        {
          pod_uid: 'pod-uid-008',
          pod_id: 'postgres-main-0',
          processes: [
            { pid: 56781, command: 'postgres: checkpointer', ppid: 1, container_id: 'ctr-008a' },
            { pid: 56782, command: 'postgres: background writer', ppid: 1, container_id: 'ctr-008a' },
          ],
        },
      ],
    },
  },

  psms: [
    {
      id: 'psm-001',
      labelSelectors: [{ key: 'app', value: 'order-processor' }],
      k8sNamespaces: ['trading'],
      commandRegex: '',
      collectionIntervalSeconds: 5,
      enabled: true,
      metrics: { voluntaryCtxSwitches: true, involuntaryCtxSwitches: true, cpuTimeNs: true, waitTimeNs: true, runCount: true, cpuMigrations: false },
      scaling: null,
      createdTime: Date.now() - 86400000,
      updatedTime: Date.now() - 3600000,
    },
    {
      id: 'psm-002',
      labelSelectors: [{ key: 'app', value: 'spark-worker' }, { key: 'tier', value: 'compute' }],
      k8sNamespaces: ['analytics'],
      commandRegex: '.*java.*spark.*',
      collectionIntervalSeconds: 10,
      enabled: true,
      metrics: { voluntaryCtxSwitches: true, involuntaryCtxSwitches: true, cpuTimeNs: true, waitTimeNs: true, runCount: true, cpuMigrations: true },
      scaling: { enabled: true, metricName: 'spark_cpu_pressure', targetValue: '75', scaleTargetRef: { apiVersion: 'apps/v1', kind: 'StatefulSet', name: 'spark-worker' }, minReplicaCount: 2, maxReplicaCount: 10, cooldownPeriod: 300 },
      createdTime: Date.now() - 172800000,
      updatedTime: Date.now() - 7200000,
    },
  ],

  // PodMetricsPage reads: namespace, podName, nodeID, voluntaryCtxSwitches, ...
  runtimeMetrics: [
    { namespace: 'trading', podName: 'order-processor-7f9b4-xk2ld', nodeID: 'node-1', voluntaryCtxSwitches: 1842, involuntaryCtxSwitches: 47, cpuTimeNs: 2834920000, waitTimeNs: 482930000, runCount: 9842, cpuMigrations: 12, smtMigrations: 3, l3Migrations: 8, numaMigrations: 0 },
    { namespace: 'trading', podName: 'order-processor-7f9b4-m8nqp', nodeID: 'node-2', voluntaryCtxSwitches: 1634, involuntaryCtxSwitches: 63, cpuTimeNs: 2194830000, waitTimeNs: 631420000, runCount: 8391, cpuMigrations: 19, smtMigrations: 5, l3Migrations: 11, numaMigrations: 1 },
    { namespace: 'analytics', podName: 'spark-worker-0', nodeID: 'node-1', voluntaryCtxSwitches: 5341, involuntaryCtxSwitches: 284, cpuTimeNs: 18293400000, waitTimeNs: 2938400000, runCount: 43921, cpuMigrations: 89, smtMigrations: 23, l3Migrations: 45, numaMigrations: 3 },
    { namespace: 'analytics', podName: 'spark-worker-1', nodeID: 'node-2', voluntaryCtxSwitches: 4912, involuntaryCtxSwitches: 312, cpuTimeNs: 16482900000, waitTimeNs: 3128400000, runCount: 39812, cpuMigrations: 94, smtMigrations: 31, l3Migrations: 52, numaMigrations: 4 },
    { namespace: 'ml', podName: 'train-job-abc12', nodeID: 'node-1', voluntaryCtxSwitches: 892, involuntaryCtxSwitches: 412, cpuTimeNs: 48293000000, waitTimeNs: 1293400000, runCount: 12834, cpuMigrations: 7, smtMigrations: 2, l3Migrations: 3, numaMigrations: 0 },
  ],

  // NodeDetailPage reads: nodeId, success, lastError, configVersion, host, appliedAt, restartCount, config.*
  schedulerConfig: {
    results: [
      {
        nodeId: 'node-1',
        host: 'node-1.cluster.local',
        success: true,
        lastError: null,
        configVersion: '2024-01-01T00:00:00Z',
        appliedAt: '2024-01-01T00:00:00Z',
        restartCount: 0,
        config: {
          mode: 'gthulhu',
          schedulerEnabled: true,
          monitoringEnabled: true,
          sliceNsDefault: 20000000,
          sliceNsMin: 1000000,
          kernelMode: true,
          maxTimeWatchdog: true,
          earlyProcessing: false,
          builtinIdle: false,
        },
      },
      {
        nodeId: 'node-2',
        host: 'node-2.cluster.local',
        success: true,
        lastError: null,
        configVersion: '2024-01-01T00:00:00Z',
        appliedAt: '2024-01-01T00:00:00Z',
        restartCount: 0,
        config: {
          mode: 'gthulhu',
          schedulerEnabled: true,
          monitoringEnabled: true,
          sliceNsDefault: 20000000,
          sliceNsMin: 1000000,
          kernelMode: true,
          maxTimeWatchdog: true,
          earlyProcessing: false,
          builtinIdle: false,
        },
      },
    ],
  },

  // PodMetricsPage Adaptive Classification section
  classifyItems: [
    {
      namespace: 'trading',
      pod: 'order-processor-7f9b4-xk2ld',
      phase: 'stable',
      classification: { current_type: ['cpu_heavy', 'interactive'], confidence: 0.87 },
      drift: { drift_score: 0.312 },
      recommendation: { action: 'no_action' },
    },
    {
      namespace: 'analytics',
      pod: 'spark-worker-0',
      phase: 'stable',
      classification: { current_type: ['cpu_heavy'], confidence: 0.92 },
      drift: { drift_score: 0.105 },
      recommendation: { action: 'increase_cpu_limit' },
    },
    {
      namespace: 'ml',
      pod: 'train-job-abc12',
      phase: 'warming_up',
      classification: { current_type: ['needs_higher_priority'], confidence: 0.61 },
      drift: { drift_score: 1.841 },
      recommendation: { action: 'raise_priority' },
    },
  ],
};

// Passwords for demo login
export const PASSWORDS = { admin: 'admin', alice: 'alice123', bob: 'bob123' };

let _tokenCounter = 0;
export function makeToken() {
  return 'mock.' + btoa(JSON.stringify({ v: ++_tokenCounter, exp: Date.now() + 3_600_000 }));
}
