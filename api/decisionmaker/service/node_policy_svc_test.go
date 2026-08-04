package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProcessSource is a test double for ProcessSource returning a fixed
// pid->comm snapshot (or an error).
type fakeProcessSource struct {
	snapshot map[int]string
	err      error
}

func (f *fakeProcessSource) Snapshot(ctx context.Context) (map[int]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
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
	svc := &Service{processSource: &fakeProcessSource{snapshot: map[int]string{1: "init"}}}
	intents, err := svc.resolveNodeSchedulingIntents(context.Background())
	require.NoError(t, err)
	assert.Nil(t, intents)
}

func TestResolveNodeSchedulingIntentsNoProcessSource(t *testing.T) {
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
		processSource: &fakeProcessSource{snapshot: map[int]string{
			100: "kthreadd",
			200: "sshd",
			300: "pause",
			400: "kworker/0:1",
		}},
	}
	require.NoError(t, svc.ProcessNodePolicies(context.Background(), []*domain.NodePolicy{
		{PolicyID: "p1", NodeID: "node-a", CommandRegex: "^kthreadd$|^kworker.*", Priority: 5, ExecutionTime: 2000},
	}))

	intents, err := svc.resolveNodeSchedulingIntents(context.Background())
	require.NoError(t, err)
	require.Len(t, intents, 2)

	pids := map[int]bool{}
	for _, intent := range intents {
		pids[intent.PID] = true
		assert.Equal(t, 5, intent.Priority)
		assert.Equal(t, uint64(2000), intent.ExecutionTime)
	}
	assert.True(t, pids[100])
	assert.True(t, pids[400])
	assert.False(t, pids[200])
	assert.False(t, pids[300], "pause command must always be excluded")
}

func TestResolveNodeSchedulingIntentsInvalidRegexSkipped(t *testing.T) {
	svc := &Service{
		processSource: &fakeProcessSource{snapshot: map[int]string{100: "sshd"}},
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
		processSource: &fakeProcessSource{err: errors.New("boom")},
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
