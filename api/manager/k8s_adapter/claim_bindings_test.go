package k8sadapter

import (
	"context"
	"testing"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func bindingPod(name, claim string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: types.UID(name)},
		Spec: corev1.PodSpec{
			NodeName: "node-a",
			ResourceClaims: []corev1.PodResourceClaim{{
				Name: "gpu", ResourceClaimName: &claim,
			}},
		},
	}
}

func allocatedClaim(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "resource.k8s.io/v1", "kind": "ResourceClaim",
		"metadata": map[string]interface{}{
			"name": name, "namespace": "ns", "uid": name + "-uid", "generation": int64(1),
		},
		"status": map[string]interface{}{
			"allocation": map[string]interface{}{
				"devices": map[string]interface{}{
					"results": []interface{}{map[string]interface{}{
						"request": "gpu", "driver": "gpu.example.com", "pool": "pool-a", "device": "dev0",
					}},
				},
			},
		},
	}}
}

func topologySlice() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "resource.k8s.io/v1", "kind": "ResourceSlice",
		"metadata": map[string]interface{}{"name": "slice", "uid": "slice-uid", "generation": int64(2)},
		"spec": map[string]interface{}{
			"driver": "gpu.example.com", "nodeName": "node-a",
			"pool": map[string]interface{}{"name": "pool-a", "generation": int64(3), "resourceSliceCount": int64(1)},
			"devices": []interface{}{map[string]interface{}{
				"name": "dev0",
				"attributes": map[string]interface{}{
					"gpu.example.com/numaNode":        map[string]interface{}{"int": int64(1)},
					"resource.kubernetes.io/pcieRoot": map[string]interface{}{"string": "root-a"},
					"pciBusID":                        map[string]interface{}{"string": "0000:01:00.0"},
				},
			}},
		},
	}}
}

func bindingAdapter(t *testing.T, pods []*corev1.Pod, objects ...runtime.Object) (*Adapter, *fake.Clientset, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	podObjects := make([]runtime.Object, len(pods))
	for i := range pods {
		podObjects[i] = pods[i]
	}
	client := fake.NewSimpleClientset(podObjects...)
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			resourceClaimV1GVR: "ResourceClaimList", resourceSliceV1GVR: "ResourceSliceList",
		}, objects...)
	a := &Adapter{client: client, dynamicClient: dyn, podCache: make(map[string]corev1.Pod), stopCh: make(chan struct{})}
	t.Cleanup(a.StopPodWatcher)
	a.startPodWatcher()
	return a, client, dyn
}

func awaitBindings(t *testing.T, a *Adapter, check func([]domain.ClaimDeviceBinding) bool) []domain.ClaimDeviceBinding {
	t.Helper()
	var got []domain.ClaimDeviceBinding
	require.Eventually(t, func() bool {
		var synced bool
		got, synced = a.QueryClaimDeviceBindings("")
		return synced && check(got)
	}, 5*time.Second, 10*time.Millisecond)
	return got
}

func assertReadOnly(t *testing.T, actions []clienttesting.Action) {
	t.Helper()
	for _, action := range actions {
		assert.Contains(t, []string{"list", "watch"}, action.GetVerb())
	}
}

func TestClaimBindingsObserveAllocationAndTopology(t *testing.T) {
	claim := allocatedClaim("claim")
	results, _, _ := unstructured.NestedSlice(claim.Object, "status", "allocation", "devices", "results")
	second := map[string]interface{}{"request": "other", "driver": "gpu.example.com", "pool": "pool-a", "device": "dev1"}
	require.NoError(t, unstructured.SetNestedSlice(claim.Object, append(results, second), "status", "allocation", "devices", "results"))
	a, client, dyn := bindingAdapter(t, []*corev1.Pod{bindingPod("pod", "claim")}, claim, topologySlice())
	got := awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool {
		return len(b) == 2 && b[0].ResourceSliceUID != ""
	})
	assert.Equal(t, domain.ClaimDeviceBinding{
		ClaimUID: "claim-uid", ClaimGeneration: 1, ClaimNamespace: "ns", ClaimName: "claim",
		PodUID: "pod", NodeName: "node-a", Request: "gpu", Driver: "gpu.example.com",
		Pool: "pool-a", Device: "dev0", NUMANodes: []int{1}, PCIeRoot: "root-a", PCIBusID: "0000:01:00.0",
		ResourceSliceUID: "slice-uid", ResourceSliceGeneration: 2, PoolGeneration: 3,
	}, got[0])
	assert.Empty(t, got[1].ResourceSliceUID, "unresolved topology must not suppress an allocation")
	got[0].NUMANodes[0] = 99
	got[0].Device = "mutated"
	again, synced := a.QueryClaimDeviceBindings("pod")
	require.True(t, synced)
	assert.Equal(t, []int{1}, again[0].NUMANodes)
	assert.Equal(t, "dev0", again[0].Device)
	missing, _ := a.QueryClaimDeviceBindings("unknown-pod")
	assert.Empty(t, missing)
	// Querying is entirely in-memory, even if the API is unavailable.
	before := len(dyn.Actions()) + len(client.Actions())
	dyn.PrependReactor("*", "*", func(clienttesting.Action) (bool, runtime.Object, error) {
		t.Error("query attempted an API request")
		return true, nil, nil
	})
	_, _ = a.QueryClaimDeviceBindings("")
	assert.Equal(t, before, len(dyn.Actions())+len(client.Actions()))
	assertReadOnly(t, dyn.Actions())
	assertReadOnly(t, client.Actions())
}

func TestClaimBindingsLifecycle(t *testing.T) {
	claim := allocatedClaim("claim")
	a, _, dyn := bindingAdapter(t, []*corev1.Pod{bindingPod("pod", "claim")}, claim)
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 })

	claim.SetGeneration(2)
	require.NoError(t, dyn.Tracker().Update(resourceClaimV1GVR, claim, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool {
		return len(b) == 1 && b[0].ClaimGeneration == 2
	})
	// Reallocation can happen without a metadata generation change.
	results := []interface{}{map[string]interface{}{
		"request": "gpu", "driver": "other.example.com", "pool": "other-pool", "device": "new-device",
	}}
	require.NoError(t, unstructured.SetNestedSlice(claim.Object, results, "status", "allocation", "devices", "results"))
	require.NoError(t, dyn.Tracker().Update(resourceClaimV1GVR, claim, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool {
		return len(b) == 1 && b[0].Device == "new-device" && b[0].Driver == "other.example.com"
	})
	unstructured.RemoveNestedField(claim.Object, "status", "allocation")
	require.NoError(t, dyn.Tracker().Update(resourceClaimV1GVR, claim, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 0 })

	claim = allocatedClaim("claim")
	claim.SetGeneration(3)
	require.NoError(t, dyn.Tracker().Update(resourceClaimV1GVR, claim, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool {
		return len(b) == 1 && b[0].ClaimGeneration == 3
	})
	require.NoError(t, dyn.Tracker().Delete(resourceClaimV1GVR, "ns", "claim"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 0 })
	claim.SetUID("replacement-uid")
	claim.SetGeneration(1)
	require.NoError(t, dyn.Tracker().Create(resourceClaimV1GVR, claim, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool {
		return len(b) == 1 && b[0].ClaimUID == "replacement-uid" && b[0].ClaimGeneration == 1
	})
	now := metav1.Now()
	claim.SetDeletionTimestamp(&now)
	require.NoError(t, dyn.Tracker().Update(resourceClaimV1GVR, claim, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 0 })
	assertReadOnly(t, dyn.Actions())
	a.StopPodWatcher()
	require.Eventually(t, func() bool {
		b, synced := a.QueryClaimDeviceBindings("")
		return !synced && len(b) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestClaimBindingsPodReferences(t *testing.T) {
	generated := bindingPod("generated", "unused")
	template := "template"
	generated.Spec.ResourceClaims[0].ResourceClaimName = nil
	generated.Spec.ResourceClaims[0].ResourceClaimTemplateName = &template
	generatedName := "generated-claim"
	generated.Status.ResourceClaimStatuses = []corev1.PodResourceClaimStatus{{Name: "gpu", ResourceClaimName: &generatedName}}
	shared := bindingPod("shared", "claim")
	// Duplicate aliases should not duplicate bindings.
	shared.Spec.ResourceClaims = append(shared.Spec.ResourceClaims, shared.Spec.ResourceClaims[0])
	crossNamespace := bindingPod("cross-namespace", "claim")
	crossNamespace.Namespace = "other"
	unscheduled := bindingPod("unscheduled", "claim")
	unscheduled.Spec.NodeName = ""
	terminal := bindingPod("terminal", "claim")
	terminal.Status.Phase = corev1.PodSucceeded
	a, client, _ := bindingAdapter(t,
		[]*corev1.Pod{bindingPod("pod", "claim"), shared, generated, crossNamespace, unscheduled, terminal},
		allocatedClaim("claim"), allocatedClaim("generated-claim"), allocatedClaim("unreferenced"))
	got := awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 3 })
	assert.Equal(t, []types.UID{"generated", "pod", "shared"}, []types.UID{got[0].PodUID, got[1].PodUID, got[2].PodUID})
	assert.Equal(t, "generated-claim", got[0].ClaimName)
	podGVR := corev1.SchemeGroupVersion.WithResource("pods")
	require.NoError(t, client.Tracker().Delete(podGVR, "ns", "pod"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 2 })
	shared.Spec.NodeName = "node-b"
	require.NoError(t, client.Tracker().Update(podGVR, shared, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool {
		return len(b) == 2 && b[1].NodeName == "node-b"
	})
	shared.Status.Phase = corev1.PodFailed
	require.NoError(t, client.Tracker().Update(podGVR, shared, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 })
}

func TestClaimBindingsNeverInferAllocationFromInventory(t *testing.T) {
	claim := allocatedClaim("claim")
	unstructured.RemoveNestedField(claim.Object, "status")
	a, _, dyn := bindingAdapter(t, []*corev1.Pod{bindingPod("pod", "claim")}, claim, topologySlice())
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 0 })
	// Malformed replacement status must not leave a previous valid allocation active.
	claim = allocatedClaim("claim")
	require.NoError(t, dyn.Tracker().Update(resourceClaimV1GVR, claim, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 })
	require.NoError(t, unstructured.SetNestedField(claim.Object, "invalid", "status", "allocation", "devices", "results"))
	require.NoError(t, dyn.Tracker().Update(resourceClaimV1GVR, claim, "ns"))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 0 })
}

func TestClaimBindingTopologyLifecycle(t *testing.T) {
	slice := topologySlice()
	a, _, dyn := bindingAdapter(t, []*corev1.Pod{bindingPod("pod", "claim")}, allocatedClaim("claim"), slice)
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 && b[0].PCIeRoot == "root-a" })
	require.NoError(t, dyn.Tracker().Delete(resourceSliceV1GVR, "", slice.GetName()))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 && b[0].ResourceSliceUID == "" })
	require.NoError(t, dyn.Tracker().Create(resourceSliceV1GVR, slice, ""))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 && b[0].ResourceSliceUID != "" })

	newSlice := topologySlice()
	newSlice.SetName("new-slice")
	newSlice.SetUID("new-slice-uid")
	require.NoError(t, unstructured.SetNestedField(newSlice.Object, int64(4), "spec", "pool", "generation"))
	require.NoError(t, unstructured.SetNestedField(newSlice.Object, int64(2), "spec", "pool", "resourceSliceCount"))
	require.NoError(t, dyn.Tracker().Create(resourceSliceV1GVR, newSlice, ""))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 && b[0].ResourceSliceUID == "" })
	require.NoError(t, unstructured.SetNestedField(newSlice.Object, int64(1), "spec", "pool", "resourceSliceCount"))
	require.NoError(t, dyn.Tracker().Update(resourceSliceV1GVR, newSlice, ""))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 && b[0].PoolGeneration == 4 })
	require.NoError(t, unstructured.SetNestedField(newSlice.Object, "node-b", "spec", "nodeName"))
	require.NoError(t, dyn.Tracker().Update(resourceSliceV1GVR, newSlice, ""))
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 && b[0].ResourceSliceUID == "" })
}

func TestClaimBindingQueryBeforeStartup(t *testing.T) {
	for _, a := range []*Adapter{nil, {}} {
		b, synced := a.QueryClaimDeviceBindings("")
		assert.Empty(t, b)
		assert.False(t, synced)
	}
}

func TestClaimBindingWatchReceivesNewClaims(t *testing.T) {
	a, _, dyn := bindingAdapter(t, []*corev1.Pod{bindingPod("pod", "claim")})
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 0 })
	_, err := dyn.Resource(resourceClaimV1GVR).Namespace("ns").Create(context.Background(), allocatedClaim("claim"), metav1.CreateOptions{})
	require.NoError(t, err)
	awaitBindings(t, a, func(b []domain.ClaimDeviceBinding) bool { return len(b) == 1 })
}
