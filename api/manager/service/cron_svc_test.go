package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCheckDMIntentsNoK8SAdapter(t *testing.T) {
	svc := &Service{}

	err := svc.CheckDMIntents(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNoClient)
}

func TestCheckDMIntentsQueryDecisionMakerPodsError(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	expectedErr := errors.New("query dms failed")
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryDecisionMakerPodsOptions) {
			require.NotNil(t, opt)
			assert.Equal(t, "app", opt.DecisionMakerLabel.Key)
			assert.Equal(t, "decisionmaker", opt.DecisionMakerLabel.Value)
		}).
		Return(nil, expectedErr).
		Once()

	svc := &Service{
		K8SAdapter: mockK8S,
	}
	err := svc.CheckDMIntents(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestCheckDMIntentsNoDecisionMakerPods(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{}, nil).
		Once()

	svc := &Service{
		K8SAdapter: mockK8S,
		// Repo intentionally left nil. If code regresses and tries to query intents,
		// this test will fail via nil dereference.
	}

	err := svc.CheckDMIntents(ctx)
	require.NoError(t, err)
}

func TestCheckDMIntentsDMAdapterNilForOnlineNode(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockRepo := domain.NewMockRepository(t)
	dm := &domain.DecisionMakerPod{
		NodeID: "node-online",
		Host:   "127.0.0.1",
		Port:   8080,
		State:  domain.NodeStateOnline,
	}

	mockRepo.EXPECT().
		QueryStrategies(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryStrategyOptions) {
			opt.Result = []*domain.ScheduleStrategy{}
		}).
		Return(nil).
		Once()
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{dm}, nil).
		Once()
	mockRepo.EXPECT().
		QueryIntents(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryIntentOptions) {
			opt.Result = []*domain.ScheduleIntent{}
		}).
		Return(nil).
		Once()

	svc := &Service{
		K8SAdapter: mockK8S,
		Repo:       mockRepo,
		DMAdapter:  nil,
	}
	err := svc.CheckDMIntents(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decision maker adapter is nil")
}

func TestCheckDMIntentsHappyPathOnlineOnly(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockRepo := domain.NewMockRepository(t)
	mockDM := domain.NewMockDecisionMakerAdapter(t)

	onlineDM := &domain.DecisionMakerPod{
		NodeID: "node-a",
		Host:   "127.0.0.1",
		Port:   8080,
		State:  domain.NodeStateOnline,
	}
	offlineDM := &domain.DecisionMakerPod{
		NodeID: "node-b",
		Host:   "127.0.0.1",
		Port:   8081,
		State:  domain.NodeStateOffline,
	}
	intents := []*domain.ScheduleIntent{
		{
			PodName:       "pod-b",
			PodID:         "pod-id-b",
			NodeID:        "node-b",
			K8sNamespace:  "ns-b",
			CommandRegex:  "busybox",
			Priority:      1,
			ExecutionTime: 22,
			PodLabels: map[string]string{
				"k2": "v2",
			},
		},
		{
			PodName:       "pod-a",
			PodID:         "pod-id-a",
			NodeID:        "node-a",
			K8sNamespace:  "ns-a",
			CommandRegex:  "nginx",
			Priority:      0,
			ExecutionTime: 11,
			PodLabels: map[string]string{
				"k1": "v1",
			},
		},
	}
	expectedRoot := buildScheduleIntentMerkleRoot([]*domain.ScheduleIntent{intents[1]})

	mockRepo.EXPECT().
		QueryStrategies(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryStrategyOptions) {
			opt.Result = []*domain.ScheduleStrategy{}
		}).
		Return(nil).
		Once()
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{onlineDM, offlineDM}, nil).
		Once()
	mockRepo.EXPECT().
		QueryIntents(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryIntentOptions) {
			opt.Result = intents
		}).
		Return(nil).
		Once()
	mockDM.EXPECT().
		GetIntentMerkleRoot(mock.Anything, onlineDM).
		Return(expectedRoot, nil).
		Once()

	svc := &Service{
		K8SAdapter: mockK8S,
		Repo:       mockRepo,
		DMAdapter:  mockDM,
	}

	err := svc.CheckDMIntents(ctx)
	require.NoError(t, err)
}

func TestCheckDMIntentsComparesNodeScopedMerkleRoots(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockRepo := domain.NewMockRepository(t)
	mockDM := domain.NewMockDecisionMakerAdapter(t)

	dmNodeA := &domain.DecisionMakerPod{
		NodeID: "node-a",
		Host:   "127.0.0.1",
		Port:   8080,
		State:  domain.NodeStateOnline,
	}
	dmNodeB := &domain.DecisionMakerPod{
		NodeID: "node-b",
		Host:   "127.0.0.1",
		Port:   8081,
		State:  domain.NodeStateOnline,
	}

	intents := []*domain.ScheduleIntent{
		{
			PodName:       "pod-a-1",
			PodID:         "pod-id-a-1",
			NodeID:        "node-a",
			K8sNamespace:  "ns-a",
			CommandRegex:  "nginx",
			Priority:      1,
			ExecutionTime: 11,
			PodLabels: map[string]string{
				"app": "api",
			},
		},
		{
			PodName:       "pod-b-1",
			PodID:         "pod-id-b-1",
			NodeID:        "node-b",
			K8sNamespace:  "ns-b",
			CommandRegex:  "redis",
			Priority:      0,
			ExecutionTime: 22,
			PodLabels: map[string]string{
				"tier": "cache",
			},
		},
		{
			PodName:       "pod-a-2",
			PodID:         "pod-id-a-2",
			NodeID:        "node-a",
			K8sNamespace:  "ns-a",
			CommandRegex:  "busybox",
			Priority:      0,
			ExecutionTime: 33,
			PodLabels: map[string]string{
				"job": "worker",
			},
		},
	}

	expectedNodeARoot := buildScheduleIntentMerkleRoot([]*domain.ScheduleIntent{intents[0], intents[2]})
	expectedNodeBRoot := buildScheduleIntentMerkleRoot([]*domain.ScheduleIntent{intents[1]})

	mockRepo.EXPECT().
		QueryStrategies(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryStrategyOptions) {
			opt.Result = []*domain.ScheduleStrategy{}
		}).
		Return(nil).
		Once()
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{dmNodeA, dmNodeB}, nil).
		Once()
	mockRepo.EXPECT().
		QueryIntents(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryIntentOptions) {
			opt.Result = intents
		}).
		Return(nil).
		Once()
	mockDM.EXPECT().
		GetIntentMerkleRoot(mock.Anything, dmNodeA).
		Return(expectedNodeARoot, nil).
		Once()
	mockDM.EXPECT().
		GetIntentMerkleRoot(mock.Anything, dmNodeB).
		Return(expectedNodeBRoot, nil).
		Once()

	svc := &Service{
		K8SAdapter: mockK8S,
		Repo:       mockRepo,
		DMAdapter:  mockDM,
	}

	err := svc.CheckDMIntents(ctx)
	require.NoError(t, err)
}

func TestReconcileIntentsResendOnMerkleMismatch(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockRepo := domain.NewMockRepository(t)
	mockDM := domain.NewMockDecisionMakerAdapter(t)

	dm := &domain.DecisionMakerPod{
		NodeID: "node-a",
		Host:   "10.0.0.1",
		Port:   8080,
		State:  domain.NodeStateOnline,
	}
	intent := &domain.ScheduleIntent{
		BaseEntity:    domain.BaseEntity{ID: bson.NewObjectID()},
		PodName:       "pod-a",
		PodID:         "pod-id-a",
		NodeID:        "node-a",
		K8sNamespace:  "default",
		CommandRegex:  "nginx",
		Priority:      1,
		ExecutionTime: 10,
		PodLabels:     map[string]string{"app": "web"},
	}
	expectedRoot := buildScheduleIntentMerkleRoot([]*domain.ScheduleIntent{intent})

	// refreshStaleIntents: no strategies → no stale checks
	mockRepo.EXPECT().
		QueryStrategies(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryStrategyOptions) {
			opt.Result = []*domain.ScheduleStrategy{}
		}).
		Return(nil).Once()

	// resyncIntentsToDMs
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{dm}, nil).Once()
	mockRepo.EXPECT().
		QueryIntents(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryIntentOptions) {
			opt.Result = []*domain.ScheduleIntent{intent}
		}).
		Return(nil).Once()

	// DM returns a different hash → triggers re-send
	mockDM.EXPECT().
		GetIntentMerkleRoot(mock.Anything, dm).
		Return("stale-hash", nil).Once()
	mockDM.EXPECT().
		SendSchedulingIntent(mock.Anything, dm, []*domain.ScheduleIntent{intent}).
		Return(nil).Once()
	mockRepo.EXPECT().
		BatchUpdateIntentsState(mock.Anything, []bson.ObjectID{intent.ID}, domain.IntentStateSent).
		Return(nil).Once()

	svc := &Service{
		K8SAdapter: mockK8S,
		Repo:       mockRepo,
		DMAdapter:  mockDM,
	}

	err := svc.ReconcileIntents(ctx)
	require.NoError(t, err)

	// Verify that the root hash we expected matches (sanity)
	assert.NotEqual(t, "stale-hash", expectedRoot)
}

func TestReconcileIntentsNoResendOnMatchingMerkle(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockRepo := domain.NewMockRepository(t)
	mockDM := domain.NewMockDecisionMakerAdapter(t)

	dm := &domain.DecisionMakerPod{
		NodeID: "node-a",
		Host:   "10.0.0.1",
		Port:   8080,
		State:  domain.NodeStateOnline,
	}
	intent := &domain.ScheduleIntent{
		PodName:       "pod-a",
		PodID:         "pod-id-a",
		NodeID:        "node-a",
		K8sNamespace:  "default",
		CommandRegex:  "nginx",
		Priority:      1,
		ExecutionTime: 10,
		PodLabels:     map[string]string{"app": "web"},
	}
	expectedRoot := buildScheduleIntentMerkleRoot([]*domain.ScheduleIntent{intent})

	mockRepo.EXPECT().
		QueryStrategies(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryStrategyOptions) {
			opt.Result = []*domain.ScheduleStrategy{}
		}).
		Return(nil).Once()
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{dm}, nil).Once()
	mockRepo.EXPECT().
		QueryIntents(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryIntentOptions) {
			opt.Result = []*domain.ScheduleIntent{intent}
		}).
		Return(nil).Once()

	// DM returns matching hash → no re-send should happen
	mockDM.EXPECT().
		GetIntentMerkleRoot(mock.Anything, dm).
		Return(expectedRoot, nil).Once()
	// SendSchedulingIntent should NOT be called (test will fail if it is)

	svc := &Service{
		K8SAdapter: mockK8S,
		Repo:       mockRepo,
		DMAdapter:  mockDM,
	}

	err := svc.ReconcileIntents(ctx)
	require.NoError(t, err)
}

func TestReconcileIntentsRefreshStaleIntents(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockRepo := domain.NewMockRepository(t)
	mockDM := domain.NewMockDecisionMakerAdapter(t)

	strategyID := bson.NewObjectID()
	strategy := &domain.ScheduleStrategy{
		BaseEntity:     domain.BaseEntity{ID: strategyID, CreatorID: bson.NewObjectID(), UpdaterID: bson.NewObjectID()},
		K8sNamespace:   []string{"default"},
		LabelSelectors: []domain.LabelSelector{{Key: "app", Value: "web"}},
		CommandRegex:   "nginx",
		Priority:       1,
		ExecutionTime:  10,
	}

	// Stale intent: references a pod that no longer exists
	staleIntentID := bson.NewObjectID()
	staleIntent := &domain.ScheduleIntent{
		BaseEntity:    domain.BaseEntity{ID: staleIntentID},
		StrategyID:    strategyID,
		PodName:       "old-pod",
		PodID:         "old-pod-id",
		NodeID:        "node-a",
		K8sNamespace:  "default",
		CommandRegex:  "nginx",
		Priority:      1,
		ExecutionTime: 10,
		PodLabels:     map[string]string{"app": "web"},
		State:         domain.IntentStateSent,
	}

	// New pod that replaced the old one
	newPod := &domain.Pod{
		Name:         "new-pod",
		PodID:        "new-pod-id",
		NodeID:       "node-a",
		K8SNamespace: "default",
		Labels:       map[string]string{"app": "web"},
	}

	dm := &domain.DecisionMakerPod{
		NodeID: "node-a",
		Host:   "10.0.0.1",
		Port:   8080,
		State:  domain.NodeStateOnline,
	}

	// Step 1: refreshStaleIntents
	mockRepo.EXPECT().
		QueryStrategies(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryStrategyOptions) {
			opt.Result = []*domain.ScheduleStrategy{strategy}
		}).
		Return(nil).Once()
	mockK8S.EXPECT().
		QueryPods(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryPodsOptions) {
			assert.Equal(t, []string{"default"}, opt.K8SNamespace)
			assert.Equal(t, "nginx", opt.CommandRegex)
		}).
		Return([]*domain.Pod{newPod}, nil).Once()
	mockRepo.EXPECT().
		QueryIntents(mock.Anything, mock.MatchedBy(func(opt *domain.QueryIntentOptions) bool {
			return len(opt.StrategyIDs) == 1 && opt.StrategyIDs[0] == strategyID
		})).
		Run(func(_ context.Context, opt *domain.QueryIntentOptions) {
			opt.Result = []*domain.ScheduleIntent{staleIntent}
		}).
		Return(nil).Once()
	// Delete stale intent
	mockRepo.EXPECT().
		DeleteIntents(mock.Anything, []bson.ObjectID{staleIntentID}).
		Return(nil).Once()
	// Notify DM to remove stale pod intents from its in-memory cache
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.MatchedBy(func(opt *domain.QueryDecisionMakerPodsOptions) bool {
			return len(opt.NodeIDs) == 1 && opt.NodeIDs[0] == "node-a"
		})).
		Return([]*domain.DecisionMakerPod{dm}, nil).Once()
	mockDM.EXPECT().
		DeleteSchedulingIntents(mock.Anything, dm, mock.MatchedBy(func(req *domain.DeleteIntentsRequest) bool {
			return len(req.PodIDs) == 1 && req.PodIDs[0] == "old-pod-id"
		})).
		Return(nil).Once()
	// Insert new intent for the replacement pod
	mockRepo.EXPECT().
		InsertIntents(mock.Anything, mock.MatchedBy(func(intents []*domain.ScheduleIntent) bool {
			return len(intents) == 1 && intents[0].PodID == "new-pod-id"
		})).
		Return(nil).Once()

	// Step 2: resyncIntentsToDMs - DM returns empty hash (matching empty intents)
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{dm}, nil).Once()
	mockRepo.EXPECT().
		QueryIntents(mock.Anything, mock.MatchedBy(func(opt *domain.QueryIntentOptions) bool {
			return len(opt.StrategyIDs) == 0 // resync queries all intents
		})).
		Run(func(_ context.Context, opt *domain.QueryIntentOptions) {
			opt.Result = []*domain.ScheduleIntent{} // will be filled with new intents after refresh
		}).
		Return(nil).Once()
	// DM returns empty hash which matches empty intents → no re-send
	emptyRootHash := util.BuildMerkleTree(nil).Hash
	mockDM.EXPECT().
		GetIntentMerkleRoot(mock.Anything, dm).
		Return(emptyRootHash, nil).Once()

	svc := &Service{
		K8SAdapter: mockK8S,
		Repo:       mockRepo,
		DMAdapter:  mockDM,
	}

	err := svc.ReconcileIntents(ctx)
	require.NoError(t, err)
}

func TestSortScheduleIntentsByKeyAndHashDeterministic(t *testing.T) {
	intentA := &domain.ScheduleIntent{
		PodName:       "pod-a",
		PodID:         "pod-id-a",
		NodeID:        "node-a",
		K8sNamespace:  "default",
		CommandRegex:  "nginx",
		Priority:      1,
		ExecutionTime: 10,
		PodLabels: map[string]string{
			"b": "2",
			"a": "1",
		},
	}
	intentB := &domain.ScheduleIntent{
		PodName:       "pod-b",
		PodID:         "pod-id-b",
		NodeID:        "node-b",
		K8sNamespace:  "kube-system",
		CommandRegex:  "busybox",
		Priority:      0,
		ExecutionTime: 20,
		PodLabels: map[string]string{
			"k1": "v1",
		},
	}
	sorted := sortScheduleIntentsByKey([]*domain.ScheduleIntent{intentB, intentA})

	require.Len(t, sorted, 2)
	assert.Equal(t, intentA.PodName, sorted[0].PodName)
	assert.Equal(t, intentB.PodName, sorted[1].PodName)
	assert.Equal(
		t,
		util.HashStringSHA256Hex("podName=pod-a|podID=pod-id-a|nodeID=node-a|k8sNamespace=default|commandRegex=nginx|priority=1|executionTime=10|podLabels=a=1,b=2"),
		hashScheduleIntent(intentA),
	)
	assert.Equal(t, hashScheduleIntent(intentA), hashScheduleIntent(&domain.ScheduleIntent{
		PodName:       intentA.PodName,
		PodID:         intentA.PodID,
		NodeID:        intentA.NodeID,
		K8sNamespace:  intentA.K8sNamespace,
		CommandRegex:  intentA.CommandRegex,
		Priority:      intentA.Priority,
		ExecutionTime: intentA.ExecutionTime,
		PodLabels: map[string]string{
			"a": "1",
			"b": "2",
		},
	}))
}
