package repository

import (
	"context"
	"testing"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newTestCRRepo() *repo {
	scheme := runtime.NewScheme()
	fakeClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			strategyGVR: "SchedulingStrategyList",
			intentGVR:   "SchedulingIntentList",
		},
	)
	return &repo{
		k8sDynamic:  fakeClient,
		crNamespace: "test-ns",
	}
}

func TestCRInsertStrategyAndIntents(t *testing.T) {
	r := newTestCRRepo()
	ctx := context.Background()

	creatorID := bson.NewObjectID()
	strategy := &domain.ScheduleStrategy{
		BaseEntity: domain.BaseEntity{
			CreatorID: creatorID,
			UpdaterID: creatorID,
		},
		StrategyNamespace: "prod",
		LabelSelectors: []domain.LabelSelector{
			{Key: "app", Value: "nginx"},
		},
		K8sNamespace:  []string{"default"},
		CommandRegex:  "nginx",
		Priority:      10,
		ExecutionTime: 5000,
	}
	intents := []*domain.ScheduleIntent{
		{
			BaseEntity:    domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID},
			PodID:         "pod-uid-1",
			PodName:       "nginx-abc",
			NodeID:        "node-1",
			K8sNamespace:  "default",
			CommandRegex:  "nginx",
			Priority:      10,
			ExecutionTime: 5000,
			PodLabels:     map[string]string{"app": "nginx"},
			State:         domain.IntentStateInitialized,
		},
	}

	err := r.InsertStrategyAndIntents(ctx, strategy, intents)
	require.NoError(t, err)
	assert.False(t, strategy.ID.IsZero(), "strategy ID should be set")
	assert.False(t, intents[0].ID.IsZero(), "intent ID should be set")
	assert.Equal(t, strategy.ID, intents[0].StrategyID, "intent should reference strategy")

	// Query strategy back
	opt := &domain.QueryStrategyOptions{IDs: []bson.ObjectID{strategy.ID}}
	err = r.QueryStrategies(ctx, opt)
	require.NoError(t, err)
	require.Len(t, opt.Result, 1)
	assert.Equal(t, strategy.ID, opt.Result[0].ID)
	assert.Equal(t, "prod", opt.Result[0].StrategyNamespace)
	assert.Equal(t, 10, opt.Result[0].Priority)
	assert.Equal(t, int64(5000), opt.Result[0].ExecutionTime)
	assert.Equal(t, "nginx", opt.Result[0].CommandRegex)
	require.Len(t, opt.Result[0].LabelSelectors, 1)
	assert.Equal(t, "app", opt.Result[0].LabelSelectors[0].Key)
	assert.Equal(t, "nginx", opt.Result[0].LabelSelectors[0].Value)
	require.Len(t, opt.Result[0].K8sNamespace, 1)
	assert.Equal(t, "default", opt.Result[0].K8sNamespace[0])

	// Query intent back
	intentOpt := &domain.QueryIntentOptions{IDs: []bson.ObjectID{intents[0].ID}}
	err = r.QueryIntents(ctx, intentOpt)
	require.NoError(t, err)
	require.Len(t, intentOpt.Result, 1)
	assert.Equal(t, intents[0].ID, intentOpt.Result[0].ID)
	assert.Equal(t, "pod-uid-1", intentOpt.Result[0].PodID)
	assert.Equal(t, "nginx-abc", intentOpt.Result[0].PodName)
	assert.Equal(t, "node-1", intentOpt.Result[0].NodeID)
	assert.Equal(t, domain.IntentStateInitialized, intentOpt.Result[0].State)
}

func TestCRQueryStrategiesByCreator(t *testing.T) {
	r := newTestCRRepo()
	ctx := context.Background()

	creator1 := bson.NewObjectID()
	creator2 := bson.NewObjectID()

	s1 := &domain.ScheduleStrategy{
		BaseEntity: domain.BaseEntity{CreatorID: creator1, UpdaterID: creator1},
		Priority:   1,
	}
	s2 := &domain.ScheduleStrategy{
		BaseEntity: domain.BaseEntity{CreatorID: creator2, UpdaterID: creator2},
		Priority:   2,
	}
	require.NoError(t, r.InsertStrategyAndIntents(ctx, s1, []*domain.ScheduleIntent{}))
	require.NoError(t, r.InsertStrategyAndIntents(ctx, s2, []*domain.ScheduleIntent{}))

	// Query by creator1
	opt := &domain.QueryStrategyOptions{CreatorIDs: []bson.ObjectID{creator1}}
	err := r.QueryStrategies(ctx, opt)
	require.NoError(t, err)
	require.Len(t, opt.Result, 1)
	assert.Equal(t, s1.ID, opt.Result[0].ID)
}

func TestCRDeleteStrategy(t *testing.T) {
	r := newTestCRRepo()
	ctx := context.Background()

	creatorID := bson.NewObjectID()
	s := &domain.ScheduleStrategy{
		BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID},
	}
	require.NoError(t, r.InsertStrategyAndIntents(ctx, s, []*domain.ScheduleIntent{}))

	err := r.DeleteStrategy(ctx, s.ID)
	require.NoError(t, err)

	opt := &domain.QueryStrategyOptions{IDs: []bson.ObjectID{s.ID}}
	err = r.QueryStrategies(ctx, opt)
	require.NoError(t, err)
	assert.Empty(t, opt.Result)
}

func TestCRDeleteIntentsByStrategyID(t *testing.T) {
	r := newTestCRRepo()
	ctx := context.Background()

	creatorID := bson.NewObjectID()
	strategy := &domain.ScheduleStrategy{
		BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID},
	}
	intents := []*domain.ScheduleIntent{
		{BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID}, PodID: "p1", NodeID: "n1", State: domain.IntentStateInitialized},
		{BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID}, PodID: "p2", NodeID: "n1", State: domain.IntentStateInitialized},
	}
	require.NoError(t, r.InsertStrategyAndIntents(ctx, strategy, intents))

	err := r.DeleteIntentsByStrategyID(ctx, strategy.ID)
	require.NoError(t, err)

	opt := &domain.QueryIntentOptions{StrategyIDs: []bson.ObjectID{strategy.ID}}
	err = r.QueryIntents(ctx, opt)
	require.NoError(t, err)
	assert.Empty(t, opt.Result)
}

func TestCRBatchUpdateIntentsState(t *testing.T) {
	r := newTestCRRepo()
	ctx := context.Background()

	creatorID := bson.NewObjectID()
	strategy := &domain.ScheduleStrategy{
		BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID},
	}
	intents := []*domain.ScheduleIntent{
		{BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID}, PodID: "p1", NodeID: "n1", State: domain.IntentStateInitialized},
	}
	require.NoError(t, r.InsertStrategyAndIntents(ctx, strategy, intents))

	err := r.BatchUpdateIntentsState(ctx, []bson.ObjectID{intents[0].ID}, domain.IntentStateSent)
	require.NoError(t, err)

	opt := &domain.QueryIntentOptions{IDs: []bson.ObjectID{intents[0].ID}}
	err = r.QueryIntents(ctx, opt)
	require.NoError(t, err)
	require.Len(t, opt.Result, 1)
	assert.Equal(t, domain.IntentStateSent, opt.Result[0].State)
}

func TestCRUpdateStrategy(t *testing.T) {
	r := newTestCRRepo()
	ctx := context.Background()

	creatorID := bson.NewObjectID()
	strategy := &domain.ScheduleStrategy{
		BaseEntity:        domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID},
		StrategyNamespace: "old",
		Priority:          1,
	}
	require.NoError(t, r.InsertStrategyAndIntents(ctx, strategy, []*domain.ScheduleIntent{}))

	// Update the strategy
	strategy.StrategyNamespace = "new"
	strategy.Priority = 99
	err := r.UpdateStrategy(ctx, strategy)
	require.NoError(t, err)

	// Verify update
	opt := &domain.QueryStrategyOptions{IDs: []bson.ObjectID{strategy.ID}}
	err = r.QueryStrategies(ctx, opt)
	require.NoError(t, err)
	require.Len(t, opt.Result, 1)
	assert.Equal(t, "new", opt.Result[0].StrategyNamespace)
	assert.Equal(t, 99, opt.Result[0].Priority)
}

func TestCRInsertAndDeleteIntents(t *testing.T) {
	r := newTestCRRepo()
	ctx := context.Background()

	creatorID := bson.NewObjectID()
	strategyID := bson.NewObjectID()
	intents := []*domain.ScheduleIntent{
		{BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID}, StrategyID: strategyID, PodID: "p1", NodeID: "n1", State: domain.IntentStateInitialized},
		{BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID}, StrategyID: strategyID, PodID: "p2", NodeID: "n1", State: domain.IntentStateInitialized},
	}
	require.NoError(t, r.InsertIntents(ctx, intents))

	// Delete one intent
	err := r.DeleteIntents(ctx, []bson.ObjectID{intents[0].ID})
	require.NoError(t, err)

	// Verify only one remains
	opt := &domain.QueryIntentOptions{StrategyIDs: []bson.ObjectID{strategyID}}
	err = r.QueryIntents(ctx, opt)
	require.NoError(t, err)
	require.Len(t, opt.Result, 1)
	assert.Equal(t, intents[1].ID, opt.Result[0].ID)
}

func TestCRQueryIntentsByCreator(t *testing.T) {
	r := newTestCRRepo()
	ctx := context.Background()

	creator1 := bson.NewObjectID()
	creator2 := bson.NewObjectID()
	strategyID := bson.NewObjectID()

	intents1 := []*domain.ScheduleIntent{
		{BaseEntity: domain.BaseEntity{CreatorID: creator1, UpdaterID: creator1}, StrategyID: strategyID, PodID: "p1", NodeID: "n1", State: domain.IntentStateInitialized},
	}
	intents2 := []*domain.ScheduleIntent{
		{BaseEntity: domain.BaseEntity{CreatorID: creator2, UpdaterID: creator2}, StrategyID: strategyID, PodID: "p2", NodeID: "n1", State: domain.IntentStateInitialized},
	}
	require.NoError(t, r.InsertIntents(ctx, intents1))
	require.NoError(t, r.InsertIntents(ctx, intents2))

	// Query by creator1
	opt := &domain.QueryIntentOptions{CreatorIDs: []bson.ObjectID{creator1}}
	err := r.QueryIntents(ctx, opt)
	require.NoError(t, err)
	require.Len(t, opt.Result, 1)
	assert.Equal(t, "p1", opt.Result[0].PodID)
}

func TestCRQueryIntentsByStates(t *testing.T) {
	r := newTestCRRepo()
	ctx := context.Background()

	creatorID := bson.NewObjectID()
	strategyID := bson.NewObjectID()
	intents := []*domain.ScheduleIntent{
		{BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID}, StrategyID: strategyID, PodID: "p1", NodeID: "n1", State: domain.IntentStateInitialized},
		{BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID}, StrategyID: strategyID, PodID: "p2", NodeID: "n1", State: domain.IntentStateSent},
		{BaseEntity: domain.BaseEntity{CreatorID: creatorID, UpdaterID: creatorID}, StrategyID: strategyID, PodID: "p3", NodeID: "n1", State: domain.IntentStateUnknown},
	}
	require.NoError(t, r.InsertIntents(ctx, intents))

	opt := &domain.QueryIntentOptions{States: []domain.IntentState{domain.IntentStateSent, domain.IntentStateUnknown}}
	err := r.QueryIntents(ctx, opt)
	require.NoError(t, err)
	require.Len(t, opt.Result, 2)
	assert.ElementsMatch(t, []string{"p2", "p3"}, []string{opt.Result[0].PodID, opt.Result[1].PodID})
}
