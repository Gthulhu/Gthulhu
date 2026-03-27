package domain

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type QueryUserOptions struct {
	IDs       []bson.ObjectID
	UserNames []string
	Result    []*User
}

type QueryRoleOptions struct {
	IDs    []bson.ObjectID
	Names  []string
	Result []*Role
}

type QueryPermissionOptions struct {
	IDs       []bson.ObjectID
	Keys      []string
	Resources []string
	Result    []*Permission
}

type QueryAuditLogOptions struct {
	TimestampGTE int64
	TimestampLTE int64
	UserIDs      []bson.ObjectID
	Result       []*AuditLog
}

type QueryStrategyOptions struct {
	IDs           []bson.ObjectID
	K8SNamespaces []string
	Result        []*ScheduleStrategy
	CreatorIDs    []bson.ObjectID
}

type QueryIntentOptions struct {
	IDs           []bson.ObjectID
	K8SNamespaces []string
	StrategyIDs   []bson.ObjectID
	States        []IntentState
	PodIDs        []string
	Result        []*ScheduleIntent
	CreatorIDs    []bson.ObjectID
}

type Repository interface {
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	QueryUsers(ctx context.Context, opt *QueryUserOptions) error
	CreateRole(ctx context.Context, role *Role) error
	UpdateRole(ctx context.Context, role *Role) error
	QueryRoles(ctx context.Context, opt *QueryRoleOptions) error
	CreatePermission(ctx context.Context, permission *Permission) error
	UpdatePermission(ctx context.Context, permission *Permission) error
	QueryPermissions(ctx context.Context, opt *QueryPermissionOptions) error
	CreateAuditLog(ctx context.Context, log *AuditLog) error
	QueryAuditLogs(ctx context.Context, opt *QueryAuditLogOptions) error

	InsertStrategyAndIntents(ctx context.Context, strategy *ScheduleStrategy, intents []*ScheduleIntent) error
	InsertIntents(ctx context.Context, intents []*ScheduleIntent) error
	BatchUpdateIntentsState(ctx context.Context, intentIDs []bson.ObjectID, newState IntentState) error
	QueryStrategies(ctx context.Context, opt *QueryStrategyOptions) error
	QueryIntents(ctx context.Context, opt *QueryIntentOptions) error
	UpdateStrategy(ctx context.Context, strategy *ScheduleStrategy) error
	DeleteStrategy(ctx context.Context, strategyID bson.ObjectID) error
	DeleteIntents(ctx context.Context, intentIDs []bson.ObjectID) error
	DeleteIntentsByStrategyID(ctx context.Context, strategyID bson.ObjectID) error

	CreatePSM(ctx context.Context, psm *PodSchedulingMetrics) error
	QueryPSMs(ctx context.Context, opt *QueryPSMOptions) error
	UpdatePSM(ctx context.Context, psm *PodSchedulingMetrics) error
	DeletePSM(ctx context.Context, name string) error
}

type Service interface {
	CreateNewUser(ctx context.Context, operator *Claims, username, password string) error
	CreateAdminUserIfNotExists(ctx context.Context, username, password string) error
	Login(ctx context.Context, email, password string) (TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutAll(ctx context.Context, user *Claims) error
	ChangePassword(ctx context.Context, user *Claims, oldPassword, newPassword string) error
	ResetPassword(ctx context.Context, operator *Claims, id, newPassword string) error
	UpdateUserPermissions(ctx context.Context, operator *Claims, id string, opt UpdateUserPermissionsOptions) error
	VerifyJWTToken(ctx context.Context, tokenString string, permissionKey PermissionKey) (Claims, RolePolicy, error)
	QueryUsers(ctx context.Context, opt *QueryUserOptions) error

	CreateRole(ctx context.Context, operator *Claims, role *Role) error
	UpdateRole(ctx context.Context, operator *Claims, roleID string, opt UpdateRoleOptions) error
	DeleteRole(ctx context.Context, operator *Claims, roleID string) error
	QueryRoles(ctx context.Context, opt *QueryRoleOptions) error
	QueryPermissions(ctx context.Context, opt *QueryPermissionOptions) error

	CreateScheduleStrategy(ctx context.Context, operator *Claims, strategy *ScheduleStrategy) error
	ListScheduleStrategies(ctx context.Context, filterOpts *QueryStrategyOptions) error
	ListScheduleIntents(ctx context.Context, filterOpts *QueryIntentOptions) error
	UpdateScheduleStrategy(ctx context.Context, operator *Claims, strategyID string, strategy *ScheduleStrategy) error
	DeleteScheduleStrategy(ctx context.Context, operator *Claims, strategyID string) error
	DeleteScheduleIntents(ctx context.Context, operator *Claims, intentIDs []string) error
	GetPodPIDMapping(ctx context.Context, nodeID string) (*PodPIDMappingResponse, error)
	ListNodes(ctx context.Context) ([]*Node, error)
	ReconcileIntents(ctx context.Context) error

	CreatePodSchedulingMetrics(ctx context.Context, operator *Claims, psm *PodSchedulingMetrics) error
	ListPodSchedulingMetrics(ctx context.Context, opt *QueryPSMOptions) error
	ListPodSchedulingMetricValues(ctx context.Context) (*PodSchedulingMetricValuesResult, error)
	UpdatePodSchedulingMetrics(ctx context.Context, operator *Claims, name string, psm *PodSchedulingMetrics) error
	DeletePodSchedulingMetrics(ctx context.Context, operator *Claims, name string) error
}

type QueryPodsOptions struct {
	K8SNamespace   []string
	LabelSelectors []LabelSelector
	CommandRegex   string
}

type QueryDecisionMakerPodsOptions struct {
	K8SNamespace       []string
	NodeIDs            []string
	DecisionMakerLabel LabelSelector
}

type K8SAdapter interface {
	QueryPods(ctx context.Context, opt *QueryPodsOptions) ([]*Pod, error)
	QueryDecisionMakerPods(ctx context.Context, opt *QueryDecisionMakerPodsOptions) ([]*DecisionMakerPod, error)
	ListNodes(ctx context.Context) ([]*Node, error)
}

type DeleteIntentsRequest struct {
	PodIDs []string // Delete all intents for these pods
	All    bool     // If true, deletes all intents on the decision maker
}

type DecisionMakerAdapter interface {
	SendSchedulingIntent(ctx context.Context, decisionMaker *DecisionMakerPod, intents []*ScheduleIntent) error
	GetIntentMerkleRoot(ctx context.Context, decisionMaker *DecisionMakerPod) (string, error)
	DeleteSchedulingIntents(ctx context.Context, decisionMaker *DecisionMakerPod, req *DeleteIntentsRequest) error
	GetPodPIDMapping(ctx context.Context, decisionMaker *DecisionMakerPod) (*PodPIDMappingResponse, error)
	GetPodSchedulingMetricValues(ctx context.Context, decisionMaker *DecisionMakerPod) ([]*PodSchedulingMetricValue, error)
}
