package k8sadapter

import (
	"context"
	"testing"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestNode(name string, labels map[string]string) *apiv1.Node {
	return &apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: apiv1.NodeStatus{
			Conditions: []apiv1.NodeCondition{
				{Type: apiv1.NodeReady, Status: apiv1.ConditionTrue},
			},
		},
	}
}

func TestQueryNodesBySelectorsLabelOnly(t *testing.T) {
	client := fake.NewSimpleClientset(
		newTestNode("node-a", map[string]string{"gpu": "true"}),
		newTestNode("node-b", map[string]string{"gpu": "false"}),
	)
	adapter := &Adapter{client: client}

	nodes, err := adapter.QueryNodesBySelectors(context.Background(), &domain.QueryNodesOptions{
		NodeSelectors: []domain.LabelSelector{{Key: "gpu", Value: "true"}},
	})
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "node-a", nodes[0].Name)
}

func TestQueryNodesBySelectorsNameFilter(t *testing.T) {
	client := fake.NewSimpleClientset(
		newTestNode("node-a", map[string]string{"gpu": "true"}),
		newTestNode("node-b", map[string]string{"gpu": "true"}),
	)
	adapter := &Adapter{client: client}

	nodes, err := adapter.QueryNodesBySelectors(context.Background(), &domain.QueryNodesOptions{
		NodeSelectors: []domain.LabelSelector{{Key: "gpu", Value: "true"}},
		NodeNames:     []string{"node-b"},
	})
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "node-b", nodes[0].Name)
}

func TestQueryNodesBySelectorsNilOptReturnsError(t *testing.T) {
	adapter := &Adapter{client: fake.NewSimpleClientset()}
	_, err := adapter.QueryNodesBySelectors(context.Background(), nil)
	require.Error(t, err)
}

func TestQueryNodesBySelectorsNoClientReturnsError(t *testing.T) {
	adapter := &Adapter{}
	_, err := adapter.QueryNodesBySelectors(context.Background(), &domain.QueryNodesOptions{})
	require.Error(t, err)
}

func newResourceSlice(name, driver, nodeName string, attrs map[string]interface{}) *unstructured.Unstructured {
	device := map[string]interface{}{
		"name": "dev0",
		"basic": map[string]interface{}{
			"attributes": attrs,
		},
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "resource.k8s.io/v1alpha3",
			"kind":       "ResourceSlice",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"driver":   driver,
				"nodeName": nodeName,
				"devices":  []interface{}{device},
			},
		},
	}
}

func newDRAFakeAdapter(t *testing.T, nodes []*apiv1.Node, slices ...*unstructured.Unstructured) *Adapter {
	t.Helper()
	objs := make([]*apiv1.Node, len(nodes))
	copy(objs, nodes)
	clientObjs := make([]runtime.Object, 0, len(nodes))
	for _, n := range nodes {
		clientObjs = append(clientObjs, n)
	}
	client := fake.NewSimpleClientset(clientObjs...)

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		resourceSliceGVR: "ResourceSliceList",
	}
	dynamicObjs := make([]runtime.Object, 0, len(slices))
	for _, s := range slices {
		dynamicObjs = append(dynamicObjs, s)
	}
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, dynamicObjs...)

	return &Adapter{client: client, dynamicClient: dynClient}
}

func TestQueryNodesByDRAMatchesDeviceClass(t *testing.T) {
	nodeA := newTestNode("node-a", nil)
	nodeB := newTestNode("node-b", nil)
	slice := newResourceSlice("slice-a", "gpu.example.com", "node-a", map[string]interface{}{
		"model": map[string]interface{}{"string": "a100"},
	})

	adapter := newDRAFakeAdapter(t, []*apiv1.Node{nodeA, nodeB}, slice)

	nodes, err := adapter.QueryNodesByDRA(context.Background(), &domain.QueryNodesByDRAOptions{
		DRASelectors: []domain.DRASelector{{DeviceClass: "gpu.example.com"}},
	})
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "node-a", nodes[0].Name)
}

func TestQueryNodesByDRAMatchesAttributes(t *testing.T) {
	nodeA := newTestNode("node-a", nil)
	slice := newResourceSlice("slice-a", "gpu.example.com", "node-a", map[string]interface{}{
		"model": map[string]interface{}{"string": "a100"},
	})

	adapter := newDRAFakeAdapter(t, []*apiv1.Node{nodeA}, slice)

	nodes, err := adapter.QueryNodesByDRA(context.Background(), &domain.QueryNodesByDRAOptions{
		DRASelectors: []domain.DRASelector{{
			DeviceClass: "gpu.example.com",
			Attributes:  []domain.LabelSelector{{Key: "model", Value: "h100"}},
		}},
	})
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestQueryNodesByDRANoSelectorsReturnsNil(t *testing.T) {
	adapter := newDRAFakeAdapter(t, nil)
	nodes, err := adapter.QueryNodesByDRA(context.Background(), &domain.QueryNodesByDRAOptions{})
	require.NoError(t, err)
	assert.Nil(t, nodes)
}

func TestQueryNodesByDRANilOptReturnsError(t *testing.T) {
	adapter := newDRAFakeAdapter(t, nil)
	_, err := adapter.QueryNodesByDRA(context.Background(), nil)
	require.Error(t, err)
}

func TestQueryNodesByDRANoClientReturnsError(t *testing.T) {
	adapter := &Adapter{}
	_, err := adapter.QueryNodesByDRA(context.Background(), &domain.QueryNodesByDRAOptions{
		DRASelectors: []domain.DRASelector{{DeviceClass: "gpu.example.com"}},
	})
	require.Error(t, err)
}
