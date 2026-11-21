package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	dmdomain "github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestPreviewNodeSchedulingPolicyFanout(t *testing.T) {
	k8s := domain.NewMockK8SAdapter(t)
	dms := domain.NewMockDecisionMakerAdapter(t)
	repo := domain.NewMockRepository(t) // No writes or saved-policy lookup are expected.
	policy := &domain.NodeSchedulingPolicy{
		NodeSelectors: []domain.LabelSelector{{Key: "pool", Value: "gpu"}},
		NodeNames:     []string{"a", "b", "c", "excluded"},
		DRASelectors:  []domain.DRASelector{{DeviceClass: "gpu"}},
		CommandRegex:  "^worker$",
		Priority:      4,
		ExecutionTime: 44,
	}
	k8s.EXPECT().QueryNodesBySelectors(mock.Anything, &domain.QueryNodesOptions{
		NodeSelectors: policy.NodeSelectors, NodeNames: policy.NodeNames,
	}).Return([]*domain.Node{{Name: "c"}, {Name: "b"}, {Name: "a"}, {Name: "excluded"}}, nil).Once()
	k8s.EXPECT().QueryNodesByDRA(mock.Anything, &domain.QueryNodesByDRAOptions{DRASelectors: policy.DRASelectors}).
		Return([]*domain.Node{{Name: "c"}, {Name: "b"}, {Name: "a"}, {Name: "dra-only"}}, nil).Once()
	a := &domain.DecisionMakerPod{NodeID: "a", State: domain.NodeStateOnline, Host: "a", Port: 8080}
	aDuplicate := &domain.DecisionMakerPod{NodeID: "a", State: domain.NodeStateOnline, Host: "duplicate", Port: 8080}
	b := &domain.DecisionMakerPod{NodeID: "b", State: domain.NodeStateOnline, Host: "b", Port: 8080}
	c := &domain.DecisionMakerPod{NodeID: "c", State: domain.NodeStateOffline, Host: "c", Port: 8080}
	k8s.EXPECT().QueryDecisionMakerPods(mock.Anything, &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabelSelector, NodeIDs: []string{"a", "b", "c"},
	}).Return([]*domain.DecisionMakerPod{aDuplicate, c, b, a}, nil).Once()

	req := func(node string) dmdomain.NodePolicyPreviewRequest {
		return dmdomain.NodePolicyPreviewRequest{Policy: &dmdomain.NodePolicy{
			Node: node, NodeID: node, CommandRegex: "^worker$", Priority: 4, ExecutionTime: 44,
		}}
	}
	dms.EXPECT().PreviewNodeSchedulingPolicy(mock.Anything, a, req("a")).Return(&dmdomain.NodePolicyPreview{
		MatchedTasks: []dmdomain.TaskInfo{{TID: 4, StartTime: 10}},
		Conflicts:    []dmdomain.NodePolicyConflict{{Task: dmdomain.TaskInfo{TID: 4, StartTime: 10}}},
		Warnings:     []dmdomain.NodePolicyWarning{{Code: "kernelThread", Message: "example"}},
	}, nil).Once()
	dms.EXPECT().PreviewNodeSchedulingPolicy(mock.Anything, b, req("b")).Return(nil, errors.New("offline during preview")).Once()
	result, err := (&Service{Repo: repo, K8SAdapter: k8s, DMAdapter: dms}).PreviewNodeSchedulingPolicy(context.Background(), newTestClaims(t), "", policy)
	require.NoError(t, err)
	assert.Equal(t, policy, result.Policy)
	assert.Equal(t, []string{"a", "b", "c"}, result.MatchedNodes)
	assert.Equal(t, []dmdomain.TaskInfo{{Node: "a", TID: 4, StartTime: 10}}, result.MatchedTasks)
	require.Len(t, result.Conflicts, 1)
	assert.Equal(t, result.MatchedTasks[0], result.Conflicts[0].Task)
	require.Len(t, result.Warnings, 3)
	assert.Equal(t, []string{"a", "b", "c"}, []string{result.Warnings[0].Node, result.Warnings[1].Node, result.Warnings[2].Node})
	assert.Equal(t, []string{"kernelThread", "decision_maker_preview_failed", "decision_maker_unavailable"}, []string{result.Warnings[0].Code, result.Warnings[1].Code, result.Warnings[2].Code})
}

func TestPreviewNodeSchedulingPolicySavedPolicy(t *testing.T) {
	repo := domain.NewMockRepository(t)
	k8s := domain.NewMockK8SAdapter(t)
	dms := domain.NewMockDecisionMakerAdapter(t)
	claims := newTestClaims(t)
	owner, err := claims.GetBsonObjectUID()
	require.NoError(t, err)
	policy := &domain.NodeSchedulingPolicy{
		BaseEntity: domain.BaseEntity{ID: bson.NewObjectID(), CreatorID: owner},
		Generation: 9, NodeNames: []string{"a"}, CommandRegex: "worker",
	}
	filter := &domain.QueryNodePolicyOptions{IDs: []bson.ObjectID{policy.ID}, CreatorIDs: []bson.ObjectID{owner}}
	repo.EXPECT().QueryNodePolicies(mock.Anything, filter).Run(func(_ context.Context, q *domain.QueryNodePolicyOptions) {
		q.Result = []*domain.NodeSchedulingPolicy{policy}
	}).Return(nil).Once()
	k8s.EXPECT().QueryNodesBySelectors(mock.Anything, &domain.QueryNodesOptions{NodeNames: policy.NodeNames}).
		Return([]*domain.Node{{Name: "a"}}, nil).Once()
	a := &domain.DecisionMakerPod{NodeID: "a", State: domain.NodeStateOnline, Host: "a", Port: 8080}
	k8s.EXPECT().QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{a}, nil).Once()
	dms.EXPECT().PreviewNodeSchedulingPolicy(mock.Anything, a, dmdomain.NodePolicyPreviewRequest{Policy: &dmdomain.NodePolicy{
		Node: "a", NodeID: "a", PolicyID: policy.ID.Hex(), Generation: 9, CommandRegex: "worker",
	}}).Return(&dmdomain.NodePolicyPreview{}, nil).Once()
	svc := &Service{Repo: repo, K8SAdapter: k8s, DMAdapter: dms}
	_, err = svc.PreviewNodeSchedulingPolicy(context.Background(), claims, policy.ID.Hex(), nil)
	require.NoError(t, err)

	other := newTestClaims(t)
	otherID, err := other.GetBsonObjectUID()
	require.NoError(t, err)
	repo.EXPECT().QueryNodePolicies(mock.Anything, &domain.QueryNodePolicyOptions{
		IDs: []bson.ObjectID{policy.ID}, CreatorIDs: []bson.ObjectID{otherID},
	}).Return(nil).Once()
	_, err = svc.PreviewNodeSchedulingPolicy(context.Background(), other, policy.ID.Hex(), nil)
	var httpErr *errs.HTTPStatusError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusNotFound, httpErr.StatusCode)
}

func TestPreviewNodeSchedulingPolicyNoMatches(t *testing.T) {
	svc := &Service{}
	result, err := svc.PreviewNodeSchedulingPolicy(context.Background(), newTestClaims(t), "", &domain.NodeSchedulingPolicy{CommandRegex: "worker"})
	require.NoError(t, err)
	assert.Empty(t, result.MatchedNodes)
	assert.Empty(t, result.MatchedTasks)
	assert.Empty(t, result.Conflicts)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "no_matching_nodes", result.Warnings[0].Code)
}

func TestNodeSchedulingIntentGenerationHashes(t *testing.T) {
	id := bson.NewObjectID()
	legacy := &domain.NodeSchedulingIntent{PolicyID: id, NodeID: "node", CommandRegex: "^worker$", Priority: 4, ExecutionTime: 20}
	serialized := "policyID=" + id.Hex() + "|nodeID=node|commandRegex=^worker$|priority=4|executionTime=20"
	assert.Equal(t, util.HashStringSHA256Hex(serialized), hashNodeSchedulingIntent(legacy))
	older, newer := *legacy, *legacy
	older.Generation, newer.Generation = 2, 11
	assert.Equal(t, util.HashStringSHA256Hex(serialized+"|generation=11"), hashNodeSchedulingIntent(&newer))
	assert.Equal(t, []*domain.NodeSchedulingIntent{legacy, &older, &newer},
		sortNodeIntentsByKey([]*domain.NodeSchedulingIntent{&newer, legacy, &older}))
}

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
