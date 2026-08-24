package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTaskSource is a test double for TaskSource returning a fixed task list
// (or an error).
type fakeTaskSource struct {
	tasks []domain.TaskInfo
	err   error
}

func (f *fakeTaskSource) Snapshot(ctx context.Context) ([]domain.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tasks, nil
}

func TestProcessNodePoliciesBuildsMerkleRoot(t *testing.T) {
	svc := &Service{}
	policies := []*domain.NodePolicy{
		{PolicyID: "p1", NodeID: "node-a", CommandRegex: "^kthreadd$", Priority: 1, ExecutionTime: 1000},
	}
	err := svc.ProcessNodePolicies(context.Background(), policies)
	require.NoError(t, err)
	assert.NotEmpty(t, svc.GetNodePolicyMerkleRootHash())

	svc2 := &Service{}
	require.NoError(t, svc2.ProcessNodePolicies(context.Background(), nil))
	assert.Equal(t, svc.GetNodePolicyMerkleRootHash() != "", true)
	// Different (empty) input should not produce the same root as a non-empty one.
	assert.NotEqual(t, svc.GetNodePolicyMerkleRootHash(), svc2.GetNodePolicyMerkleRootHash())
}

func TestResolveNodeSchedulingIntentsNoPolicies(t *testing.T) {
	svc := &Service{taskSource: &fakeTaskSource{tasks: []domain.TaskInfo{{TGID: 1, TID: 1, Comm: "init"}}}}
	intents, err := svc.resolveNodeSchedulingIntents(context.Background())
	require.NoError(t, err)
	assert.Nil(t, intents)
}

func TestResolveNodeSchedulingIntentsNoTaskSource(t *testing.T) {
	svc := &Service{}
	require.NoError(t, svc.ProcessNodePolicies(context.Background(), []*domain.NodePolicy{
		{PolicyID: "p1", NodeID: "node-a", CommandRegex: ".*", Priority: 1, ExecutionTime: 1000},
	}))
	intents, err := svc.resolveNodeSchedulingIntents(context.Background())
	require.NoError(t, err)
	assert.Nil(t, intents)
}

func TestResolveNodeSchedulingIntentsMatchesCommRegex(t *testing.T) {
	svc := &Service{
		taskSource: &fakeTaskSource{tasks: []domain.TaskInfo{
			{TGID: 100, TID: 100, Comm: "kthreadd"},
			{TGID: 200, TID: 200, Comm: "sshd"},
			{TGID: 300, TID: 300, Comm: "pause"},
			{TGID: 400, TID: 400, Comm: "kworker/0:1"},
			// A non-leader thread (TID != TGID) whose comm matches: the case
			// the old top-level-only scan could never reach.
			{TGID: 500, TID: 501, Comm: "kthreadd"},
		}},
	}
	require.NoError(t, svc.ProcessNodePolicies(context.Background(), []*domain.NodePolicy{
		{PolicyID: "p1", NodeID: "node-a", CommandRegex: "^kthreadd$|^kworker.*", Priority: 5, ExecutionTime: 2000},
	}))

	intents, err := svc.resolveNodeSchedulingIntents(context.Background())
	require.NoError(t, err)
	require.Len(t, intents, 3)

	pids := map[int]bool{}
	for _, intent := range intents {
		pids[intent.PID] = true
		assert.Equal(t, 5, intent.Priority)
		assert.Equal(t, uint64(2000), intent.ExecutionTime)
	}
	assert.True(t, pids[100])
	assert.True(t, pids[400])
	assert.True(t, pids[501], "non-leader thread must be matched by its TID")
	assert.False(t, pids[200])
	assert.False(t, pids[300], "pause command must always be excluded")
}

// TestResolveNodeSchedulingIntentsDeterministicOverlap verifies that two
// policies matching the same TID resolve to one intent per TID, and to the same
// winner regardless of the order the policies were pushed, so two decision
// makers with the same policy set do not diverge.
func TestResolveNodeSchedulingIntentsDeterministicOverlap(t *testing.T) {
	tasks := []domain.TaskInfo{{TGID: 100, TID: 100, Comm: "engine"}}
	pA := &domain.NodePolicy{PolicyID: "a", NodeID: "n", CommandRegex: "^engine$", Priority: 1, ExecutionTime: 100}
	pB := &domain.NodePolicy{PolicyID: "b", NodeID: "n", CommandRegex: "engine.*", Priority: 2, ExecutionTime: 200}

	resolve := func(order ...*domain.NodePolicy) []*domain.SchedulingIntents {
		svc := &Service{taskSource: &fakeTaskSource{tasks: tasks}}
		require.NoError(t, svc.ProcessNodePolicies(context.Background(), order))
		got, err := svc.resolveNodeSchedulingIntents(context.Background())
		require.NoError(t, err)
		return got
	}
	ab := resolve(pA, pB)
	ba := resolve(pB, pA)

	require.Len(t, ab, 1, "one intent per TID")
	require.Len(t, ba, 1)
	assert.Equal(t, 100, ab[0].PID)
	assert.Equal(t, ab[0].CommandRegex, ba[0].CommandRegex, "overlap winner must not depend on push order")
	assert.Equal(t, ab[0].Priority, ba[0].Priority)
}

func TestResolveNodeSchedulingIntentsInvalidRegexSkipped(t *testing.T) {
	svc := &Service{
		taskSource: &fakeTaskSource{tasks: []domain.TaskInfo{{TGID: 100, TID: 100, Comm: "sshd"}}},
	}
	require.NoError(t, svc.ProcessNodePolicies(context.Background(), []*domain.NodePolicy{
		{PolicyID: "p1", NodeID: "node-a", CommandRegex: "(", Priority: 1, ExecutionTime: 1000},
	}))
	intents, err := svc.resolveNodeSchedulingIntents(context.Background())
	require.NoError(t, err)
	assert.Nil(t, intents)
}

func TestResolveNodeSchedulingIntentsSnapshotError(t *testing.T) {
	svc := &Service{
		taskSource: &fakeTaskSource{err: errors.New("boom")},
	}
	require.NoError(t, svc.ProcessNodePolicies(context.Background(), []*domain.NodePolicy{
		{PolicyID: "p1", NodeID: "node-a", CommandRegex: ".*", Priority: 1, ExecutionTime: 1000},
	}))
	_, err := svc.resolveNodeSchedulingIntents(context.Background())
	require.Error(t, err)
}

func TestDeleteNodePolicyByID(t *testing.T) {
	svc := &Service{}
	require.NoError(t, svc.ProcessNodePolicies(context.Background(), []*domain.NodePolicy{
		{PolicyID: "p1", NodeID: "node-a", CommandRegex: "a", Priority: 1, ExecutionTime: 1},
		{PolicyID: "p2", NodeID: "node-b", CommandRegex: "b", Priority: 2, ExecutionTime: 2},
	}))

	require.NoError(t, svc.DeleteNodePolicyByID(context.Background(), "p1"))
	svc.nodePolicyCacheMu.RLock()
	remaining := svc.nodePolicyCache
	svc.nodePolicyCacheMu.RUnlock()
	require.Len(t, remaining, 1)
	assert.Equal(t, "p2", remaining[0].PolicyID)
}

func TestDeleteAllNodePolicies(t *testing.T) {
	svc := &Service{}
	require.NoError(t, svc.ProcessNodePolicies(context.Background(), []*domain.NodePolicy{
		{PolicyID: "p1", NodeID: "node-a", CommandRegex: "a", Priority: 1, ExecutionTime: 1},
	}))
	require.NoError(t, svc.DeleteAllNodePolicies(context.Background()))
	svc.nodePolicyCacheMu.RLock()
	remaining := svc.nodePolicyCache
	svc.nodePolicyCacheMu.RUnlock()
	assert.Empty(t, remaining)

	emptyRootHash := svc.GetNodePolicyMerkleRootHash()
	svc2 := &Service{}
	require.NoError(t, svc2.ProcessNodePolicies(context.Background(), nil))
	assert.Equal(t, svc2.GetNodePolicyMerkleRootHash(), emptyRootHash)
}
