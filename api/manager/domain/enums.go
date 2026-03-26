package domain

type PermissionKey string

const (
	CreateUser             PermissionKey = "user.create"
	UserRead               PermissionKey = "user.read"
	ChangeUserPermission   PermissionKey = "user.permission.update"
	ResetUserPassword      PermissionKey = "user.password.reset"
	RoleCrete              PermissionKey = "role.create"
	RoleRead               PermissionKey = "role.read"
	RoleUpdate             PermissionKey = "role.update"
	RoleDelete             PermissionKey = "role.delete"
	PermissionRead         PermissionKey = "permission.read"
	ScheduleStrategyCreate PermissionKey = "schedule_strategy.create"
	ScheduleStrategyRead   PermissionKey = "schedule_strategy.read"
	ScheduleStrategyUpdate PermissionKey = "schedule_strategy.update"
	ScheduleStrategyDelete PermissionKey = "schedule_strategy.delete"
	ScheduleIntentRead     PermissionKey = "schedule_intent.read"
	ScheduleIntentDelete   PermissionKey = "schedule_intent.delete"
	PodPIDMappingRead      PermissionKey = "pod_pid_mapping.read"
	PSMCreate              PermissionKey = "pod_scheduling_metrics.create"
	PSMRead                PermissionKey = "pod_scheduling_metrics.read"
	PSMUpdate              PermissionKey = "pod_scheduling_metrics.update"
	PSMDelete              PermissionKey = "pod_scheduling_metrics.delete"
	SchedulerConfigRead    PermissionKey = "scheduler_config.read"
	SchedulerConfigUpdate  PermissionKey = "scheduler_config.update"
)

const (
	AdminRole = "admin"
)

type NodeState int8

const (
	NodeStateUnknown NodeState = iota
	NodeStateOnline
	NodeStateOffline
)

type IntentState int8

const (
	IntentStateUnknown IntentState = iota
	IntentStateInitialized
	IntentStateSent
)
