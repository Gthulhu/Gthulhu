package k8sadapter

import (
	"sort"
	"sync"

	"github.com/Gthulhu/api/manager/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

var (
	resourceClaimV1GVR = schema.GroupVersionResource{Group: "resource.k8s.io", Version: "v1", Resource: "resourceclaims"}
	resourceSliceV1GVR = schema.GroupVersionResource{Group: "resource.k8s.io", Version: "v1", Resource: "resourceslices"}
)

var _ domain.ClaimBindingReader = (*Adapter)(nil)

type claimObserver struct {
	pods, claims, slices cache.SharedIndexInformer
	changed              chan struct{}
	mu                   sync.RWMutex
	bindings             []domain.ClaimDeviceBinding
	synced               bool
}

func newClaimObserver(pods cache.SharedIndexInformer, client dynamic.Interface) (*claimObserver, dynamicinformer.DynamicSharedInformerFactory) {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(client, 0)
	o := &claimObserver{
		pods: pods, claims: factory.ForResource(resourceClaimV1GVR).Informer(),
		slices: factory.ForResource(resourceSliceV1GVR).Informer(), changed: make(chan struct{}, 1),
	}
	notify := func(interface{}) {
		select {
		case o.changed <- struct{}{}:
		default:
		}
	}
	for _, informer := range []cache.SharedIndexInformer{o.pods, o.claims, o.slices} {
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: notify, UpdateFunc: func(_, obj interface{}) { notify(obj) }, DeleteFunc: notify,
		})
	}
	return o, factory
}

// run has a single reconciliation worker. Informer events (including deletion
// tombstones and relists) only enqueue work; queries never traverse informers.
func (o *claimObserver) run(stop <-chan struct{}) {
	defer func() {
		o.mu.Lock()
		o.bindings, o.synced = nil, false
		o.mu.Unlock()
	}()
	if !cache.WaitForCacheSync(stop, o.pods.HasSynced, o.claims.HasSynced) {
		return
	}
	for {
		o.reconcile()
		select {
		case <-stop:
			return
		case <-o.changed:
		}
	}
}

func (a *Adapter) QueryClaimDeviceBindings(podUID types.UID) ([]domain.ClaimDeviceBinding, bool) {
	if a == nil || a.claimObserver == nil {
		return nil, false
	}
	o := a.claimObserver
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]domain.ClaimDeviceBinding, 0, len(o.bindings))
	for _, binding := range o.bindings {
		if podUID != "" && binding.PodUID != podUID {
			continue
		}
		binding.NUMANodes = append([]int(nil), binding.NUMANodes...)
		result = append(result, binding)
	}
	return result, o.synced
}

func (o *claimObserver) reconcile() {
	var bindings []domain.ClaimDeviceBinding
	topology := o.topologyIndex()
	for _, obj := range o.pods.GetStore().List() {
		pod := obj.(*corev1.Pod)
		if pod.UID == "" || pod.Spec.NodeName == "" || pod.DeletionTimestamp != nil ||
			pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		seen := make(map[string]bool)
		for _, ref := range pod.Spec.ResourceClaims {
			name := ""
			if ref.ResourceClaimName != nil {
				name = *ref.ResourceClaimName
			} else if ref.ResourceClaimTemplateName != nil {
				for _, status := range pod.Status.ResourceClaimStatuses {
					if status.Name == ref.Name && status.ResourceClaimName != nil {
						name = *status.ResourceClaimName
						break
					}
				}
			}
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			obj, exists, _ := o.claims.GetStore().GetByKey(pod.Namespace + "/" + name)
			if !exists {
				continue
			}
			claim := obj.(*unstructured.Unstructured)
			if claim.GetUID() == "" || claim.GetDeletionTimestamp() != nil {
				continue
			}
			if ref.ResourceClaimName == nil && !metav1.IsControlledBy(claim, pod) {
				continue
			}
			results, _, _ := unstructured.NestedSlice(claim.Object, "status", "allocation", "devices", "results")
			for _, raw := range results {
				result, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				b := domain.ClaimDeviceBinding{
					ClaimUID: claim.GetUID(), ClaimGeneration: claim.GetGeneration(),
					ClaimNamespace: claim.GetNamespace(), ClaimName: claim.GetName(),
					PodUID: pod.UID, NodeName: pod.Spec.NodeName,
					Request: getUnstructuredStr(result, "request"),
					Driver:  getUnstructuredStr(result, "driver"), Pool: getUnstructuredStr(result, "pool"),
					Device: getUnstructuredStr(result, "device"),
				}
				if b.Driver == "" || b.Pool == "" || b.Device == "" {
					continue
				}
				if t, ok := topology[deviceKey{b.Driver, b.Pool, b.Device}]; ok &&
					(t.node == b.NodeName || t.allNodes) {
					b.NUMANodes, b.PCIeRoot, b.PCIBusID = t.numa, t.root, t.bus
					b.ResourceSliceUID, b.ResourceSliceGeneration, b.PoolGeneration = t.uid, t.generation, t.poolGeneration
				}
				bindings = append(bindings, b)
			}
		}
	}
	sort.Slice(bindings, func(i, j int) bool {
		a, b := bindings[i], bindings[j]
		ak := []string{string(a.PodUID), string(a.ClaimUID), a.Request, a.Driver, a.Pool, a.Device}
		bk := []string{string(b.PodUID), string(b.ClaimUID), b.Request, b.Driver, b.Pool, b.Device}
		for k := range ak {
			if ak[k] != bk[k] {
				return ak[k] < bk[k]
			}
		}
		return false
	})
	o.mu.Lock()
	o.bindings, o.synced = bindings, true
	o.mu.Unlock()
}

type deviceKey struct{ driver, pool, device string }
type poolKey struct{ driver, pool string }
type deviceTopology struct {
	node                       string
	allNodes                   bool
	numa                       []int
	root, bus                  string
	uid                        types.UID
	generation, poolGeneration int64
}

// topologyIndex uses only the newest complete pool generation. Missing,
// ambiguous or node-selector-based inventory leaves topology unresolved;
// it must never manufacture or suppress an allocation.
func (o *claimObserver) topologyIndex() map[deviceKey]deviceTopology {
	pools := make(map[poolKey][]*unstructured.Unstructured)
	for _, obj := range o.slices.GetStore().List() {
		s := obj.(*unstructured.Unstructured)
		if s.GetDeletionTimestamp() != nil {
			continue
		}
		key := poolKey{getUnstructuredStr(s.Object, "spec", "driver"), getUnstructuredStr(s.Object, "spec", "pool", "name")}
		pools[key] = append(pools[key], s)
	}
	index := make(map[deviceKey]deviceTopology)
	for pool, slices := range pools {
		var latest int64 = -1
		for _, s := range slices {
			g, _, _ := unstructured.NestedInt64(s.Object, "spec", "pool", "generation")
			if g > latest {
				latest = g
			}
		}
		var current []*unstructured.Unstructured
		for _, s := range slices {
			g, _, _ := unstructured.NestedInt64(s.Object, "spec", "pool", "generation")
			if g == latest {
				current = append(current, s)
			}
		}
		complete := true
		for _, s := range current {
			count, _, _ := unstructured.NestedInt64(s.Object, "spec", "pool", "resourceSliceCount")
			if count != int64(len(current)) {
				complete = false
			}
		}
		if !complete {
			continue
		}
		duplicates := make(map[deviceKey]bool)
		for _, s := range current {
			devices, _, _ := unstructured.NestedSlice(s.Object, "spec", "devices")
			for _, raw := range devices {
				device, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				key := deviceKey{pool.driver, pool.pool, getUnstructuredStr(device, "name")}
				if _, exists := index[key]; exists {
					duplicates[key] = true
				}
				attrs, _, _ := unstructured.NestedMap(device, "attributes")
				t := deviceTopology{
					node: getUnstructuredStr(s.Object, "spec", "nodeName"),
					uid:  s.GetUID(), generation: s.GetGeneration(), poolGeneration: latest,
				}
				t.allNodes, _, _ = unstructured.NestedBool(s.Object, "spec", "allNodes")
				if n, ok := topologyAttribute(attrs, pool.driver, "numaNode")["int"].(int64); ok && n >= 0 && int64(int(n)) == n {
					t.numa = []int{int(n)}
				}
				t.root, _ = topologyAttribute(attrs, pool.driver, "pcieRoot")["string"].(string)
				t.bus, _ = topologyAttribute(attrs, pool.driver, "pciBusID")["string"].(string)
				index[key] = t
			}
		}
		for key := range duplicates {
			delete(index, key)
		}
	}
	return index
}

func topologyAttribute(attrs map[string]interface{}, driver, name string) map[string]interface{} {
	for _, key := range []string{"resource.kubernetes.io/" + name, driver + "/" + name, name} {
		if attr, ok := attrs[key].(map[string]interface{}); ok {
			return attr
		}
	}
	return nil
}
