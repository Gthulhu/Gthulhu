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
	strategyGVR = schema.GroupVersionResource{
		Group:    "gthulhu.io",
		Version:  "v1alpha1",
		Resource: "schedulingstrategies",
	}
	intentGVR = schema.GroupVersionResource{
		Group:    "gthulhu.io",
		Version:  "v1alpha1",
		Resource: "schedulingintents",
	}
)

const (
	labelCreatorID  = "gthulhu.io/creator-id"
	labelStrategyID = "gthulhu.io/strategy-id"
	labelState      = "gthulhu.io/state"
)

// ---------------------------------------------------------------------------
// Strategy CRUD
// ---------------------------------------------------------------------------

func (r *repo) InsertStrategyAndIntents(ctx context.Context, strategy *domain.ScheduleStrategy, intents []*domain.ScheduleIntent) error {
	if strategy == nil {
		return errors.New("nil strategy")
	}
	if intents == nil {
		return errors.New("nil intents")
	}
	now := time.Now().UnixMilli()
	if strategy.ID.IsZero() {
		strategy.ID = bson.NewObjectID()
	}
	if strategy.CreatedTime == 0 {
		strategy.CreatedTime = now
	}
	strategy.UpdatedTime = now

	obj := domainStrategyToUnstructured(strategy, r.crNamespace)
	created, err := r.k8sDynamic.Resource(strategyGVR).Namespace(r.crNamespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create strategy CR: %w", err)
	}
	createdIntentNames := make([]string, 0, len(intents))
	// Assign the ID back from the created object name (may differ on retry).
	if id, e := bson.ObjectIDFromHex(created.GetName()); e == nil {
		strategy.ID = id
	}

	for _, intent := range intents {
		if intent.ID.IsZero() {
			intent.ID = bson.NewObjectID()
		}
		intent.StrategyID = strategy.ID
		if intent.CreatedTime == 0 {
			intent.CreatedTime = now
		}
		if intent.UpdatedTime == 0 {
			intent.UpdatedTime = now
		}
		intentObj := domainIntentToUnstructured(intent, r.crNamespace)
		if _, err := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).Create(ctx, intentObj, metav1.CreateOptions{}); err != nil {
			rollbackErrs := make([]string, 0, len(createdIntentNames)+1)
			for _, createdIntentName := range createdIntentNames {
				delErr := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).Delete(ctx, createdIntentName, metav1.DeleteOptions{})
				if delErr != nil && !k8serrors.IsNotFound(delErr) {
					rollbackErrs = append(rollbackErrs, fmt.Sprintf("delete intent CR %s: %v", createdIntentName, delErr))
				}
			}
			delStrategyErr := r.k8sDynamic.Resource(strategyGVR).Namespace(r.crNamespace).Delete(ctx, strategy.ID.Hex(), metav1.DeleteOptions{})
			if delStrategyErr != nil && !k8serrors.IsNotFound(delStrategyErr) {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("delete strategy CR %s: %v", strategy.ID.Hex(), delStrategyErr))
			}

			if len(rollbackErrs) > 0 {
				return fmt.Errorf("create intent CR: %w; rollback errors: %s", err, strings.Join(rollbackErrs, "; "))
			}
			return fmt.Errorf("create intent CR: %w", err)
		}
		createdIntentNames = append(createdIntentNames, intent.ID.Hex())
	}
	return nil
}

func (r *repo) InsertIntents(ctx context.Context, intents []*domain.ScheduleIntent) error {
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
		obj := domainIntentToUnstructured(intent, r.crNamespace)
		if _, err := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).Create(ctx, obj, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create intent CR: %w", err)
		}
	}
	return nil
}

func (r *repo) BatchUpdateIntentsState(ctx context.Context, intentIDs []bson.ObjectID, newState domain.IntentState) error {
	now := time.Now().UnixMilli()
	for _, id := range intentIDs {
		name := id.Hex()
		obj, err := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("get intent CR %s: %w", name, err)
		}
		spec, found, err := unstructured.NestedMap(obj.Object, "spec")
		if err != nil {
			return fmt.Errorf("read spec for intent CR %s: %w", name, err)
		}
		if !found {
			return fmt.Errorf("spec not found for intent CR %s", name)
		}
		spec["state"] = int64(newState)
		spec["updatedTime"] = now
		if err := unstructured.SetNestedField(obj.Object, spec, "spec"); err != nil {
			return err
		}
		// Update the state label for efficient filtering.
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[labelState] = strconv.Itoa(int(newState))
		obj.SetLabels(labels)

		if _, err := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update intent CR %s: %w", name, err)
		}
	}
	return nil
}

func (r *repo) QueryStrategies(ctx context.Context, opt *domain.QueryStrategyOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}

	// If specific IDs are requested, fetch each by name.
	if len(opt.IDs) > 0 {
		for _, id := range opt.IDs {
			obj, err := r.k8sDynamic.Resource(strategyGVR).Namespace(r.crNamespace).Get(ctx, id.Hex(), metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				return err
			}
			s, err := unstructuredToDomainStrategy(obj)
			if err != nil {
				return err
			}
			if matchesStrategyFilter(s, opt) {
				opt.Result = append(opt.Result, s)
			}
		}
		return nil
	}

	// Build label selector for list queries.
	sel := buildLabelSelector(opt.CreatorIDs, labelCreatorID)
	list, err := r.k8sDynamic.Resource(strategyGVR).Namespace(r.crNamespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return err
	}
	for i := range list.Items {
		s, err := unstructuredToDomainStrategy(&list.Items[i])
		if err != nil {
			return err
		}
		if matchesStrategyFilter(s, opt) {
			opt.Result = append(opt.Result, s)
		}
	}
	return nil
}

func (r *repo) QueryIntents(ctx context.Context, opt *domain.QueryIntentOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}

	// Fetch by specific IDs when provided.
	if len(opt.IDs) > 0 {
		for _, id := range opt.IDs {
			obj, err := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).Get(ctx, id.Hex(), metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				return err
			}
			intent, err := unstructuredToDomainIntent(obj)
			if err != nil {
				return err
			}
			if matchesIntentFilter(intent, opt) {
				opt.Result = append(opt.Result, intent)
			}
		}
		return nil
	}

	// Build label selector for common filters.
	selParts := []string{}
	if s := buildLabelSelector(opt.CreatorIDs, labelCreatorID); s != "" {
		selParts = append(selParts, s)
	}
	if s := buildLabelSelector(opt.StrategyIDs, labelStrategyID); s != "" {
		selParts = append(selParts, s)
	}
	if s := buildStateLabelSelector(opt.States); s != "" {
		selParts = append(selParts, s)
	}
	sel := strings.Join(selParts, ",")

	list, err := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return err
	}
	for i := range list.Items {
		intent, err := unstructuredToDomainIntent(&list.Items[i])
		if err != nil {
			return err
		}
		if matchesIntentFilter(intent, opt) {
			opt.Result = append(opt.Result, intent)
		}
	}
	return nil
}

func (r *repo) UpdateStrategy(ctx context.Context, strategy *domain.ScheduleStrategy) error {
	if strategy == nil {
		return errors.New("nil strategy")
	}
	obj := domainStrategyToUnstructured(strategy, r.crNamespace)

	// Preserve the existing resourceVersion for optimistic concurrency.
	existing, err := r.k8sDynamic.Resource(strategyGVR).Namespace(r.crNamespace).Get(ctx, strategy.ID.Hex(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get strategy CR for update: %w", err)
	}
	obj.SetResourceVersion(existing.GetResourceVersion())

	_, err = r.k8sDynamic.Resource(strategyGVR).Namespace(r.crNamespace).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

func (r *repo) DeleteStrategy(ctx context.Context, strategyID bson.ObjectID) error {
	err := r.k8sDynamic.Resource(strategyGVR).Namespace(r.crNamespace).Delete(ctx, strategyID.Hex(), metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *repo) DeleteIntents(ctx context.Context, intentIDs []bson.ObjectID) error {
	if len(intentIDs) == 0 {
		return nil
	}
	for _, id := range intentIDs {
		err := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).Delete(ctx, id.Hex(), metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete intent CR %s: %w", id.Hex(), err)
		}
	}
	return nil
}

func (r *repo) DeleteIntentsByStrategyID(ctx context.Context, strategyID bson.ObjectID) error {
	sel := labelStrategyID + "=" + strategyID.Hex()
	list, err := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return err
	}
	for _, item := range list.Items {
		if err := r.k8sDynamic.Resource(intentGVR).Namespace(r.crNamespace).Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete intent CR %s: %w", item.GetName(), err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func domainStrategyToUnstructured(s *domain.ScheduleStrategy, namespace string) *unstructured.Unstructured {
	labelSelectors := make([]interface{}, len(s.LabelSelectors))
	for i, ls := range s.LabelSelectors {
		labelSelectors[i] = map[string]interface{}{
			"key":   ls.Key,
			"value": ls.Value,
		}
	}
	k8sNS := make([]interface{}, len(s.K8sNamespace))
	for i, ns := range s.K8sNamespace {
		k8sNS[i] = ns
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gthulhu.io/v1alpha1",
			"kind":       "SchedulingStrategy",
			"metadata": map[string]interface{}{
				"name":      s.ID.Hex(),
				"namespace": namespace,
				"labels": map[string]interface{}{
					labelCreatorID: s.CreatorID.Hex(),
				},
			},
			"spec": map[string]interface{}{
				"strategyNamespace": s.StrategyNamespace,
				"labelSelectors":    labelSelectors,
				"k8sNamespaces":     k8sNS,
				"commandRegex":      s.CommandRegex,
				"priority":          int64(s.Priority),
				"executionTime":     s.ExecutionTime,
				"creatorID":         s.CreatorID.Hex(),
				"updaterID":         s.UpdaterID.Hex(),
				"createdTime":       s.CreatedTime,
				"updatedTime":       s.UpdatedTime,
			},
		},
	}
}

func unstructuredToDomainStrategy(obj *unstructured.Unstructured) (*domain.ScheduleStrategy, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil, fmt.Errorf("spec not found in strategy CR %s", obj.GetName())
	}

	id, err := bson.ObjectIDFromHex(obj.GetName())
	if err != nil {
		return nil, fmt.Errorf("invalid strategy CR name %s: %w", obj.GetName(), err)
	}

	strategy := &domain.ScheduleStrategy{
		BaseEntity: domain.BaseEntity{
			ID:          id,
			CreatedTime: getInt64(spec, "createdTime"),
			UpdatedTime: getInt64(spec, "updatedTime"),
		},
		StrategyNamespace: getStr(spec, "strategyNamespace"),
		CommandRegex:      getStr(spec, "commandRegex"),
		Priority:          int(getInt64(spec, "priority")),
		ExecutionTime:     getInt64(spec, "executionTime"),
	}

	creatorID, err := parseObjectIDField(spec, "creatorID")
	if err != nil {
		return nil, fmt.Errorf("invalid creatorID in strategy CR %s: %w", obj.GetName(), err)
	}
	strategy.CreatorID = creatorID

	updaterID, err := parseObjectIDField(spec, "updaterID")
	if err != nil {
		return nil, fmt.Errorf("invalid updaterID in strategy CR %s: %w", obj.GetName(), err)
	}
	strategy.UpdaterID = updaterID

	if raw, ok := spec["labelSelectors"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, item := range arr {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				strategy.LabelSelectors = append(strategy.LabelSelectors, domain.LabelSelector{
					Key:   getStr(m, "key"),
					Value: getStr(m, "value"),
				})
			}
		}
	}
	if raw, ok := spec["k8sNamespaces"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					strategy.K8sNamespace = append(strategy.K8sNamespace, s)
				}
			}
		}
	}
	return strategy, nil
}

func domainIntentToUnstructured(intent *domain.ScheduleIntent, namespace string) *unstructured.Unstructured {
	podLabels := map[string]interface{}{}
	for k, v := range intent.PodLabels {
		podLabels[k] = v
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gthulhu.io/v1alpha1",
			"kind":       "SchedulingIntent",
			"metadata": map[string]interface{}{
				"name":      intent.ID.Hex(),
				"namespace": namespace,
				"labels": map[string]interface{}{
					labelCreatorID:  intent.CreatorID.Hex(),
					labelStrategyID: intent.StrategyID.Hex(),
					labelState:      strconv.Itoa(int(intent.State)),
				},
			},
			"spec": map[string]interface{}{
				"strategyID":    intent.StrategyID.Hex(),
				"podID":         intent.PodID,
				"podName":       intent.PodName,
				"nodeID":        intent.NodeID,
				"k8sNamespace":  intent.K8sNamespace,
				"commandRegex":  intent.CommandRegex,
				"priority":      int64(intent.Priority),
				"executionTime": intent.ExecutionTime,
				"podLabels":     podLabels,
				"state":         int64(intent.State),
				"creatorID":     intent.CreatorID.Hex(),
				"updaterID":     intent.UpdaterID.Hex(),
				"createdTime":   intent.CreatedTime,
				"updatedTime":   intent.UpdatedTime,
			},
		},
	}
}

func unstructuredToDomainIntent(obj *unstructured.Unstructured) (*domain.ScheduleIntent, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil, fmt.Errorf("spec not found in intent CR %s", obj.GetName())
	}

	id, err := bson.ObjectIDFromHex(obj.GetName())
	if err != nil {
		return nil, fmt.Errorf("invalid intent CR name %s: %w", obj.GetName(), err)
	}

	intent := &domain.ScheduleIntent{
		BaseEntity: domain.BaseEntity{
			ID:          id,
			CreatedTime: getInt64(spec, "createdTime"),
			UpdatedTime: getInt64(spec, "updatedTime"),
		},
		PodID:         getStr(spec, "podID"),
		PodName:       getStr(spec, "podName"),
		NodeID:        getStr(spec, "nodeID"),
		K8sNamespace:  getStr(spec, "k8sNamespace"),
		CommandRegex:  getStr(spec, "commandRegex"),
		Priority:      int(getInt64(spec, "priority")),
		ExecutionTime: getInt64(spec, "executionTime"),
		State:         domain.IntentState(getInt64(spec, "state")),
	}

	creatorID, err := parseObjectIDField(spec, "creatorID")
	if err != nil {
		return nil, fmt.Errorf("invalid creatorID in intent CR %s: %w", obj.GetName(), err)
	}
	intent.CreatorID = creatorID

	updaterID, err := parseObjectIDField(spec, "updaterID")
	if err != nil {
		return nil, fmt.Errorf("invalid updaterID in intent CR %s: %w", obj.GetName(), err)
	}
	intent.UpdaterID = updaterID

	strategyID, err := parseObjectIDField(spec, "strategyID")
	if err != nil {
		return nil, fmt.Errorf("invalid strategyID in intent CR %s: %w", obj.GetName(), err)
	}
	intent.StrategyID = strategyID

	if raw, ok := spec["podLabels"]; ok {
		if m, ok := raw.(map[string]interface{}); ok {
			intent.PodLabels = make(map[string]string, len(m))
			for k, v := range m {
				if s, ok := v.(string); ok {
					intent.PodLabels[k] = s
				}
			}
		}
	}
	return intent, nil
}

// ---------------------------------------------------------------------------
// Filter helpers
// ---------------------------------------------------------------------------

func matchesStrategyFilter(s *domain.ScheduleStrategy, opt *domain.QueryStrategyOptions) bool {
	if len(opt.CreatorIDs) > 0 && !containsOID(opt.CreatorIDs, s.CreatorID) {
		return false
	}
	if len(opt.K8SNamespaces) > 0 {
		if !sliceOverlap(opt.K8SNamespaces, s.K8sNamespace) {
			return false
		}
	}
	return true
}

func matchesIntentFilter(intent *domain.ScheduleIntent, opt *domain.QueryIntentOptions) bool {
	if len(opt.CreatorIDs) > 0 && !containsOID(opt.CreatorIDs, intent.CreatorID) {
		return false
	}
	if len(opt.StrategyIDs) > 0 && !containsOID(opt.StrategyIDs, intent.StrategyID) {
		return false
	}
	if len(opt.K8SNamespaces) > 0 && !containsStr(opt.K8SNamespaces, intent.K8sNamespace) {
		return false
	}
	if len(opt.States) > 0 && !containsState(opt.States, intent.State) {
		return false
	}
	if len(opt.PodIDs) > 0 && !containsStr(opt.PodIDs, intent.PodID) {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

func buildLabelSelector(ids []bson.ObjectID, label string) string {
	if len(ids) == 0 {
		return ""
	}
	if len(ids) == 1 {
		return label + "=" + ids[0].Hex()
	}
	vals := make([]string, len(ids))
	for i, id := range ids {
		vals[i] = id.Hex()
	}
	return label + " in (" + strings.Join(vals, ",") + ")"
}

func containsOID(ids []bson.ObjectID, target bson.ObjectID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func containsStr(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func containsState(haystack []domain.IntentState, needle domain.IntentState) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func sliceOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	for _, v := range a {
		if _, ok := set[v]; ok {
			return true
		}
	}
	return false
}

func buildStateLabelSelector(states []domain.IntentState) string {
	if len(states) == 0 {
		return ""
	}
	if len(states) == 1 {
		return labelState + "=" + strconv.Itoa(int(states[0]))
	}
	vals := make([]string, len(states))
	for i, state := range states {
		vals[i] = strconv.Itoa(int(state))
	}
	return labelState + " in (" + strings.Join(vals, ",") + ")"
}

func parseObjectIDField(m map[string]interface{}, key string) (bson.ObjectID, error) {
	v, ok := m[key]
	if !ok {
		return bson.ObjectID{}, fmt.Errorf("missing field %s", key)
	}
	value, ok := v.(string)
	if !ok {
		return bson.ObjectID{}, fmt.Errorf("field %s is not a string", key)
	}
	if value == "" {
		return bson.ObjectID{}, fmt.Errorf("field %s is empty", key)
	}
	id, err := bson.ObjectIDFromHex(value)
	if err != nil {
		return bson.ObjectID{}, err
	}
	return id, nil
}

func getStr(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func getInt64(m map[string]interface{}, key string) int64 {
	switch v := m[key].(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	case int:
		return int64(v)
	default:
		return 0
	}
}
