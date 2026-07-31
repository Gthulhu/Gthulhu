package service

import (
	"context"
	"testing"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func newTestClaims(t *testing.T) *domain.Claims {
	t.Helper()
	return &domain.Claims{UID: bson.NewObjectID().Hex()}
}

func TestResolveNodesForPolicyNoSelectorsReturnsEmpty(t *testing.T) {
	svc := &Service{}
	nodes, err := svc.resolveNodesForPolicy(context.Background(), &domain.NodeSchedulingPolicy{})
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestResolveNodesForPolicyLabelSelectorsOnly(t *testing.T) {
	mockK8S := domain.NewMockK8SAdapter(t)
	nodeA := &domain.Node{Name: "node-a"}
	nodeB := &domain.Node{Name: "node-b"}

	mockK8S.EXPECT().
		QueryNodesBySelectors(mock.Anything, mock.MatchedBy(func(opt *domain.QueryNodesOptions) bool {
			return len(opt.NodeSelectors) == 1 && opt.NodeSelectors[0].Key == "gpu"
		})).
		Return([]*domain.Node{nodeA, nodeB}, nil).Once()

	svc := &Service{K8SAdapter: mockK8S}
	policy := &domain.NodeSchedulingPolicy{
		NodeSelectors: []domain.LabelSelector{{Key: "gpu", Value: "true"}},
	}
	nodes, err := svc.resolveNodesForPolicy(context.Background(), policy)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
}

func TestResolveNodesForPolicyIntersectsLabelAndDRA(t *testing.T) {
	mockK8S := domain.NewMockK8SAdapter(t)
	nodeA := &domain.Node{Name: "node-a"}
	nodeB := &domain.Node{Name: "node-b"}

	mockK8S.EXPECT().
		QueryNodesBySelectors(mock.Anything, mock.Anything).
		Return([]*domain.Node{nodeA, nodeB}, nil).Once()
	mockK8S.EXPECT().
		QueryNodesByDRA(mock.Anything, mock.MatchedBy(func(opt *domain.QueryNodesByDRAOptions) bool {
			return len(opt.DRASelectors) == 1 && opt.DRASelectors[0].DeviceClass == "gpu.example.com"
		})).
		Return([]*domain.Node{nodeA}, nil).Once()

	svc := &Service{K8SAdapter: mockK8S}
	policy := &domain.NodeSchedulingPolicy{
		NodeSelectors: []domain.LabelSelector{{Key: "region", Value: "us"}},
		DRASelectors:  []domain.DRASelector{{DeviceClass: "gpu.example.com"}},
	}
	nodes, err := svc.resolveNodesForPolicy(context.Background(), policy)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "node-a", nodes[0].Name)
}

func TestResolveNodesForPolicyDRAOnly(t *testing.T) {
	mockK8S := domain.NewMockK8SAdapter(t)
	nodeA := &domain.Node{Name: "node-a"}

	mockK8S.EXPECT().
		QueryNodesByDRA(mock.Anything, mock.Anything).
		Return([]*domain.Node{nodeA}, nil).Once()

	svc := &Service{K8SAdapter: mockK8S}
	policy := &domain.NodeSchedulingPolicy{
		DRASelectors: []domain.DRASelector{{DeviceClass: "gpu.example.com"}},
	}
	nodes, err := svc.resolveNodesForPolicy(context.Background(), policy)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
}

func TestCreateNodeSchedulingPolicyNoMatchingNodesReturnsNotFound(t *testing.T) {
	mockK8S := domain.NewMockK8SAdapter(t)
	mockK8S.EXPECT().
		QueryNodesBySelectors(mock.Anything, mock.Anything).
		Return(nil, nil).Once()

	svc := &Service{K8SAdapter: mockK8S}
	claims := newTestClaims(t)
	policy := &domain.NodeSchedulingPolicy{
		NodeSelectors: []domain.LabelSelector{{Key: "gpu", Value: "true"}},
		CommandRegex:  "sshd",
		Priority:      1,
		ExecutionTime: 1000,
	}
	err := svc.CreateNodeSchedulingPolicy(context.Background(), claims, policy)
	require.Error(t, err)
}

func TestCreateNodeSchedulingPolicyHappyPath(t *testing.T) {
	mockK8S := domain.NewMockK8SAdapter(t)
	mockRepo := domain.NewMockRepository(t)
	mockDM := domain.NewMockDecisionMakerAdapter(t)

	nodeA := &domain.Node{Name: "node-a"}
	dmA := &domain.DecisionMakerPod{NodeID: "node-a", Host: "10.0.0.1", Port: 8080, State: domain.NodeStateOnline}

	mockK8S.EXPECT().
		QueryNodesBySelectors(mock.Anything, mock.Anything).
		Return([]*domain.Node{nodeA}, nil).Once()

	mockRepo.EXPECT().
		InsertNodePolicyAndIntents(mock.Anything, mock.Anything, mock.MatchedBy(func(intents []*domain.NodeSchedulingIntent) bool {
			return len(intents) == 1 && intents[0].NodeID == "node-a"
		})).
		Return(nil).Once()

	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{dmA}, nil).Once()
	mockDM.EXPECT().
		SendNodeSchedulingPolicies(mock.Anything, dmA, mock.Anything).
		Return(nil).Once()
	mockRepo.EXPECT().
		BatchUpdateNodeIntentsState(mock.Anything, mock.Anything, domain.IntentStateSent).
		Return(nil).Once()

	svc := &Service{K8SAdapter: mockK8S, Repo: mockRepo, DMAdapter: mockDM}
	claims := newTestClaims(t)
	policy := &domain.NodeSchedulingPolicy{
		NodeSelectors: []domain.LabelSelector{{Key: "gpu", Value: "true"}},
		CommandRegex:  "sshd",
		Priority:      1,
		ExecutionTime: 1000,
	}
	err := svc.CreateNodeSchedulingPolicy(context.Background(), claims, policy)
	require.NoError(t, err)
}

func TestDeleteNodeSchedulingPolicyNotFound(t *testing.T) {
	mockRepo := domain.NewMockRepository(t)
	mockRepo.EXPECT().
		QueryNodePolicies(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryNodePolicyOptions) {
			opt.Result = nil
		}).
		Return(nil).Once()

	svc := &Service{Repo: mockRepo}
	claims := newTestClaims(t)
	err := svc.DeleteNodeSchedulingPolicy(context.Background(), claims, bson.NewObjectID().Hex())
	require.Error(t, err)
}
