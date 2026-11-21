package repository

import (
	"context"
	"testing"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestCRNodePolicyGenerationRoundTrip(t *testing.T) {
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			nodePolicyGVR: "NodeSchedulingPolicyList", nodeIntentGVR: "NodeSchedulingIntentList",
		})
	// The dynamic fake has no apiserver strategy to set metadata.generation.
	fake.PrependReactor("create", nodePolicyGVR.Resource, func(action ktesting.Action) (bool, runtime.Object, error) {
		obj := action.(ktesting.CreateAction).GetObject().(*unstructured.Unstructured)
		assert.Zero(t, obj.GetGeneration(), "Manager does not write policy generation")
		obj.SetGeneration(1)
		obj.SetResourceVersion("1")
		return false, nil, nil
	})
	fake.PrependReactor("update", nodePolicyGVR.Resource, func(action ktesting.Action) (bool, runtime.Object, error) {
		obj := action.(ktesting.UpdateAction).GetObject().(*unstructured.Unstructured)
		assert.Equal(t, "1", obj.GetResourceVersion())
		assert.Zero(t, obj.GetGeneration())
		obj.SetGeneration(2)
		obj.SetResourceVersion("2")
		return false, nil, nil
	})

	ctx := context.Background()
	r := &repo{k8sDynamic: fake, crNamespace: "test"}
	policy := &domain.NodeSchedulingPolicy{CommandRegex: "^worker$", NodeNames: []string{"node-a"}}
	node := &domain.Node{Name: "node-a"}
	intent := domain.NewNodeSchedulingIntent(policy, node)
	require.NoError(t, r.InsertNodePolicyAndIntents(ctx, policy, []*domain.NodeSchedulingIntent{&intent}))
	assert.Equal(t, int64(1), policy.Generation)
	assert.Equal(t, int64(1), intent.Generation)

	policy.CommandRegex = "^updated$"
	require.NoError(t, r.UpdateNodePolicy(ctx, policy))
	assert.Equal(t, int64(2), policy.Generation)
	opt := &domain.QueryNodePolicyOptions{IDs: []bson.ObjectID{policy.ID}}
	require.NoError(t, r.QueryNodePolicies(ctx, opt))
	require.Len(t, opt.Result, 1)
	assert.Equal(t, int64(2), opt.Result[0].Generation)

	secondIntent := domain.NewNodeSchedulingIntent(opt.Result[0], node)
	require.NoError(t, r.InsertNodeIntents(ctx, []*domain.NodeSchedulingIntent{&secondIntent}))
	require.NoError(t, r.BatchUpdateNodeIntentsState(ctx, []bson.ObjectID{secondIntent.ID}, domain.IntentStateSent))

	intentOpt := &domain.QueryNodeIntentOptions{IDs: []bson.ObjectID{intent.ID, secondIntent.ID}}
	require.NoError(t, r.QueryNodeIntents(ctx, intentOpt))
	require.Len(t, intentOpt.Result, 2)
	assert.Equal(t, int64(1), intentOpt.Result[0].Generation)
	assert.Equal(t, int64(2), intentOpt.Result[1].Generation)
	assert.Equal(t, domain.IntentStateSent, intentOpt.Result[1].State)

	obj, err := fake.Resource(nodeIntentGVR).Namespace(r.crNamespace).Get(ctx, secondIntent.ID.Hex(), metav1.GetOptions{})
	require.NoError(t, err)
	generation, found, err := unstructured.NestedInt64(obj.Object, "spec", "generation")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, int64(2), generation)
	unstructured.RemoveNestedField(obj.Object, "spec", "generation")
	legacyIntent, err := unstructuredToDomainNodeIntent(obj)
	require.NoError(t, err)
	assert.Zero(t, legacyIntent.Generation)
}
