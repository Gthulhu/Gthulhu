package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	nodePolicyGVR = schema.GroupVersionResource{
		Group:    "gthulhu.io",
		Version:  "v1alpha1",
		Resource: "nodeschedulingpolicies",
	}
	nodeIntentGVR = schema.GroupVersionResource{
		Group:    "gthulhu.io",
		Version:  "v1alpha1",
		Resource: "nodeschedulingintents",
	}
)

const (
	labelNodePolicyID = "gthulhu.io/node-policy-id"
)

// ---------------------------------------------------------------------------
// NodeSchedulingPolicy CRUD
// ---------------------------------------------------------------------------

func (r *repo) InsertNodePolicyAndIntents(ctx context.Context, policy *domain.NodeSchedulingPolicy, intents []*domain.NodeSchedulingIntent) error {
	if policy == nil {
		return errors.New("nil node policy")
	}
	if intents == nil {
		return errors.New("nil node intents")
	}
	now := time.Now().UnixMilli()
	if policy.ID.IsZero() {
		policy.ID = bson.NewObjectID()
	}
	if policy.CreatedTime == 0 {
		policy.CreatedTime = now
	}
	policy.UpdatedTime = now

	obj := domainNodePolicyToUnstructured(policy, r.crNamespace)
	created, err := r.k8sDynamic.Resource(nodePolicyGVR).Namespace(r.crNamespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create node policy CR: %w", err)
	}
	if id, e := bson.ObjectIDFromHex(created.GetName()); e == nil {
		policy.ID = id
	}

	createdIntentNames := make([]string, 0, len(intents))
	for _, intent := range intents {
		if intent.ID.IsZero() {
			intent.ID = bson.NewObjectID()
		}
		intent.PolicyID = policy.ID
		if intent.CreatedTime == 0 {
			intent.CreatedTime = now
		}
		if intent.UpdatedTime == 0 {
			intent.UpdatedTime = now
		}
		intentObj := domainNodeIntentToUnstructured(intent, r.crNamespace)
		if _, err := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).Create(ctx, intentObj, metav1.CreateOptions{}); err != nil {
			rollbackErrs := make([]string, 0, len(createdIntentNames)+1)
			for _, createdIntentName := range createdIntentNames {
				if delErr := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).Delete(ctx, createdIntentName, metav1.DeleteOptions{}); delErr != nil && !k8serrors.IsNotFound(delErr) {
					rollbackErrs = append(rollbackErrs, fmt.Sprintf("delete node intent CR %s: %v", createdIntentName, delErr))
				}
			}
			if delErr := r.k8sDynamic.Resource(nodePolicyGVR).Namespace(r.crNamespace).Delete(ctx, policy.ID.Hex(), metav1.DeleteOptions{}); delErr != nil && !k8serrors.IsNotFound(delErr) {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("delete node policy CR %s: %v", policy.ID.Hex(), delErr))
			}
			if len(rollbackErrs) > 0 {
				return fmt.Errorf("create node intent CR: %w; rollback errors: %s", err, strings.Join(rollbackErrs, "; "))
			}
			return fmt.Errorf("create node intent CR: %w", err)
		}
		createdIntentNames = append(createdIntentNames, intent.ID.Hex())
	}
	return nil
}

func (r *repo) InsertNodeIntents(ctx context.Context, intents []*domain.NodeSchedulingIntent) error {
	if len(intents) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	for _, intent := range intents {
		if intent.ID.IsZero() {
			intent.ID = bson.NewObjectID()
		}
		if intent.CreatedTime == 0 {
			intent.CreatedTime = now
		}
		if intent.UpdatedTime == 0 {
			intent.UpdatedTime = now
		}
		obj := domainNodeIntentToUnstructured(intent, r.crNamespace)
		if _, err := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).Create(ctx, obj, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create node intent CR: %w", err)
		}
	}
	return nil
}

func (r *repo) BatchUpdateNodeIntentsState(ctx context.Context, intentIDs []bson.ObjectID, newState domain.IntentState) error {
	now := time.Now().UnixMilli()
	for _, id := range intentIDs {
		name := id.Hex()
		obj, err := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("get node intent CR %s: %w", name, err)
		}
		spec, found, err := unstructured.NestedMap(obj.Object, "spec")
		if err != nil {
			return fmt.Errorf("read spec for node intent CR %s: %w", name, err)
		}
		if !found {
			return fmt.Errorf("spec not found for node intent CR %s", name)
		}
		spec["state"] = int64(newState)
		spec["updatedTime"] = now
		if err := unstructured.SetNestedField(obj.Object, spec, "spec"); err != nil {
			return err
		}
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[labelState] = strconv.Itoa(int(newState))
		obj.SetLabels(labels)

		if _, err := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update node intent CR %s: %w", name, err)
		}
	}
	return nil
}

func (r *repo) QueryNodePolicies(ctx context.Context, opt *domain.QueryNodePolicyOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}

	if len(opt.IDs) > 0 {
		for _, id := range opt.IDs {
			obj, err := r.k8sDynamic.Resource(nodePolicyGVR).Namespace(r.crNamespace).Get(ctx, id.Hex(), metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				return err
			}
			p, err := unstructuredToDomainNodePolicy(obj)
			if err != nil {
				return err
			}
			if matchesNodePolicyFilter(p, opt) {
				opt.Result = append(opt.Result, p)
			}
		}
		return nil
	}

	sel := buildLabelSelector(opt.CreatorIDs, labelCreatorID)
	list, err := r.k8sDynamic.Resource(nodePolicyGVR).Namespace(r.crNamespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return err
	}
	for i := range list.Items {
		p, err := unstructuredToDomainNodePolicy(&list.Items[i])
		if err != nil {
			return err
		}
		if matchesNodePolicyFilter(p, opt) {
			opt.Result = append(opt.Result, p)
		}
	}
	return nil
}

func (r *repo) QueryNodeIntents(ctx context.Context, opt *domain.QueryNodeIntentOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}

	if len(opt.IDs) > 0 {
		for _, id := range opt.IDs {
			obj, err := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).Get(ctx, id.Hex(), metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				return err
			}
			intent, err := unstructuredToDomainNodeIntent(obj)
			if err != nil {
				return err
			}
			if matchesNodeIntentFilter(intent, opt) {
				opt.Result = append(opt.Result, intent)
			}
		}
		return nil
	}

	selParts := []string{}
	if s := buildLabelSelector(opt.CreatorIDs, labelCreatorID); s != "" {
		selParts = append(selParts, s)
	}
	if s := buildLabelSelector(opt.PolicyIDs, labelNodePolicyID); s != "" {
		selParts = append(selParts, s)
	}
	if s := buildStateLabelSelector(opt.States); s != "" {
		selParts = append(selParts, s)
	}
	sel := strings.Join(selParts, ",")

	list, err := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return err
	}
	for i := range list.Items {
		intent, err := unstructuredToDomainNodeIntent(&list.Items[i])
		if err != nil {
			return err
		}
		if matchesNodeIntentFilter(intent, opt) {
			opt.Result = append(opt.Result, intent)
		}
	}
	return nil
}

func (r *repo) UpdateNodePolicy(ctx context.Context, policy *domain.NodeSchedulingPolicy) error {
	if policy == nil {
		return errors.New("nil node policy")
	}
	obj := domainNodePolicyToUnstructured(policy, r.crNamespace)

	existing, err := r.k8sDynamic.Resource(nodePolicyGVR).Namespace(r.crNamespace).Get(ctx, policy.ID.Hex(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node policy CR for update: %w", err)
	}
	obj.SetResourceVersion(existing.GetResourceVersion())

	_, err = r.k8sDynamic.Resource(nodePolicyGVR).Namespace(r.crNamespace).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

func (r *repo) DeleteNodePolicy(ctx context.Context, policyID bson.ObjectID) error {
	err := r.k8sDynamic.Resource(nodePolicyGVR).Namespace(r.crNamespace).Delete(ctx, policyID.Hex(), metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *repo) DeleteNodeIntents(ctx context.Context, intentIDs []bson.ObjectID) error {
	if len(intentIDs) == 0 {
		return nil
	}
	for _, id := range intentIDs {
		err := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).Delete(ctx, id.Hex(), metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete node intent CR %s: %w", id.Hex(), err)
		}
	}
	return nil
}

func (r *repo) DeleteNodeIntentsByPolicyID(ctx context.Context, policyID bson.ObjectID) error {
	sel := labelNodePolicyID + "=" + policyID.Hex()
	list, err := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return err
	}
	for _, item := range list.Items {
		if err := r.k8sDynamic.Resource(nodeIntentGVR).Namespace(r.crNamespace).Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete node intent CR %s: %w", item.GetName(), err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func domainNodePolicyToUnstructured(p *domain.NodeSchedulingPolicy, namespace string) *unstructured.Unstructured {
	nodeSelectors := make([]interface{}, len(p.NodeSelectors))
	for i, ls := range p.NodeSelectors {
		nodeSelectors[i] = map[string]interface{}{
			"key":   ls.Key,
			"value": ls.Value,
		}
	}
	nodeNames := make([]interface{}, len(p.NodeNames))
	for i, name := range p.NodeNames {
		nodeNames[i] = name
	}
	draSelectors := make([]interface{}, len(p.DRASelectors))
	for i, dra := range p.DRASelectors {
		attrs := make([]interface{}, len(dra.Attributes))
		for j, attr := range dra.Attributes {
			attrs[j] = map[string]interface{}{
				"key":   attr.Key,
				"value": attr.Value,
			}
		}
		draSelectors[i] = map[string]interface{}{
			"deviceClass": dra.DeviceClass,
			"attributes":  attrs,
		}
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gthulhu.io/v1alpha1",
			"kind":       "NodeSchedulingPolicy",
			"metadata": map[string]interface{}{
				"name":      p.ID.Hex(),
				"namespace": namespace,
				"labels": map[string]interface{}{
					labelCreatorID: p.CreatorID.Hex(),
				},
			},
			"spec": map[string]interface{}{
				"nodeSelectors": nodeSelectors,
				"nodeNames":     nodeNames,
				"draSelectors":  draSelectors,
				"commandRegex":  p.CommandRegex,
				"priority":      int64(p.Priority),
				"executionTime": p.ExecutionTime,
				"creatorID":     p.CreatorID.Hex(),
				"updaterID":     p.UpdaterID.Hex(),
				"createdTime":   p.CreatedTime,
				"updatedTime":   p.UpdatedTime,
			},
		},
	}
}

func unstructuredToDomainNodePolicy(obj *unstructured.Unstructured) (*domain.NodeSchedulingPolicy, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil, fmt.Errorf("spec not found in node policy CR %s", obj.GetName())
	}

	id, err := bson.ObjectIDFromHex(obj.GetName())
	if err != nil {
		return nil, fmt.Errorf("invalid node policy CR name %s: %w", obj.GetName(), err)
	}

	policy := &domain.NodeSchedulingPolicy{
		BaseEntity: domain.BaseEntity{
			ID:          id,
			CreatedTime: getInt64(spec, "createdTime"),
			UpdatedTime: getInt64(spec, "updatedTime"),
		},
		CommandRegex:  getStr(spec, "commandRegex"),
		Priority:      int(getInt64(spec, "priority")),
		ExecutionTime: getInt64(spec, "executionTime"),
	}

	creatorID, err := parseObjectIDField(spec, "creatorID")
	if err != nil {
		return nil, fmt.Errorf("invalid creatorID in node policy CR %s: %w", obj.GetName(), err)
	}
	policy.CreatorID = creatorID

	updaterID, err := parseObjectIDField(spec, "updaterID")
	if err != nil {
		return nil, fmt.Errorf("invalid updaterID in node policy CR %s: %w", obj.GetName(), err)
	}
	policy.UpdaterID = updaterID

	if raw, ok := spec["nodeSelectors"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, item := range arr {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				policy.NodeSelectors = append(policy.NodeSelectors, domain.LabelSelector{
					Key:   getStr(m, "key"),
					Value: getStr(m, "value"),
				})
			}
		}
	}
	if raw, ok := spec["nodeNames"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					policy.NodeNames = append(policy.NodeNames, s)
				}
			}
		}
	}
	if raw, ok := spec["draSelectors"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, item := range arr {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				dra := domain.DRASelector{DeviceClass: getStr(m, "deviceClass")}
				if attrsRaw, ok := m["attributes"]; ok {
					if attrsArr, ok := attrsRaw.([]interface{}); ok {
						for _, attrItem := range attrsArr {
							attrMap, ok := attrItem.(map[string]interface{})
							if !ok {
								continue
							}
							dra.Attributes = append(dra.Attributes, domain.LabelSelector{
								Key:   getStr(attrMap, "key"),
								Value: getStr(attrMap, "value"),
							})
						}
					}
				}
				policy.DRASelectors = append(policy.DRASelectors, dra)
			}
		}
	}
	return policy, nil
}

func domainNodeIntentToUnstructured(intent *domain.NodeSchedulingIntent, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gthulhu.io/v1alpha1",
			"kind":       "NodeSchedulingIntent",
			"metadata": map[string]interface{}{
				"name":      intent.ID.Hex(),
				"namespace": namespace,
				"labels": map[string]interface{}{
					labelCreatorID:    intent.CreatorID.Hex(),
					labelNodePolicyID: intent.PolicyID.Hex(),
					labelState:        strconv.Itoa(int(intent.State)),
				},
			},
			"spec": map[string]interface{}{
				"policyID":      intent.PolicyID.Hex(),
				"nodeID":        intent.NodeID,
				"commandRegex":  intent.CommandRegex,
				"priority":      int64(intent.Priority),
				"executionTime": intent.ExecutionTime,
				"state":         int64(intent.State),
				"creatorID":     intent.CreatorID.Hex(),
				"updaterID":     intent.UpdaterID.Hex(),
				"createdTime":   intent.CreatedTime,
				"updatedTime":   intent.UpdatedTime,
			},
		},
	}
}

func unstructuredToDomainNodeIntent(obj *unstructured.Unstructured) (*domain.NodeSchedulingIntent, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil, fmt.Errorf("spec not found in node intent CR %s", obj.GetName())
	}

	id, err := bson.ObjectIDFromHex(obj.GetName())
	if err != nil {
		return nil, fmt.Errorf("invalid node intent CR name %s: %w", obj.GetName(), err)
	}

	intent := &domain.NodeSchedulingIntent{
		BaseEntity: domain.BaseEntity{
			ID:          id,
			CreatedTime: getInt64(spec, "createdTime"),
			UpdatedTime: getInt64(spec, "updatedTime"),
		},
		NodeID:        getStr(spec, "nodeID"),
		CommandRegex:  getStr(spec, "commandRegex"),
		Priority:      int(getInt64(spec, "priority")),
		ExecutionTime: getInt64(spec, "executionTime"),
		State:         domain.IntentState(getInt64(spec, "state")),
	}

	creatorID, err := parseObjectIDField(spec, "creatorID")
	if err != nil {
		return nil, fmt.Errorf("invalid creatorID in node intent CR %s: %w", obj.GetName(), err)
	}
	intent.CreatorID = creatorID

	updaterID, err := parseObjectIDField(spec, "updaterID")
	if err != nil {
		return nil, fmt.Errorf("invalid updaterID in node intent CR %s: %w", obj.GetName(), err)
	}
	intent.UpdaterID = updaterID

	policyID, err := parseObjectIDField(spec, "policyID")
	if err != nil {
		return nil, fmt.Errorf("invalid policyID in node intent CR %s: %w", obj.GetName(), err)
	}
	intent.PolicyID = policyID

	return intent, nil
}

// ---------------------------------------------------------------------------
// Filter helpers
// ---------------------------------------------------------------------------

func matchesNodePolicyFilter(p *domain.NodeSchedulingPolicy, opt *domain.QueryNodePolicyOptions) bool {
	if len(opt.CreatorIDs) > 0 && !containsOID(opt.CreatorIDs, p.CreatorID) {
		return false
	}
	return true
}

func matchesNodeIntentFilter(intent *domain.NodeSchedulingIntent, opt *domain.QueryNodeIntentOptions) bool {
	if len(opt.CreatorIDs) > 0 && !containsOID(opt.CreatorIDs, intent.CreatorID) {
		return false
	}
	if len(opt.PolicyIDs) > 0 && !containsOID(opt.PolicyIDs, intent.PolicyID) {
		return false
	}
	if len(opt.NodeIDs) > 0 && !containsStr(opt.NodeIDs, intent.NodeID) {
		return false
	}
	if len(opt.States) > 0 && !containsState(opt.States, intent.State) {
		return false
	}
	return true
}
