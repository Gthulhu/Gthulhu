package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var psmGVR = schema.GroupVersionResource{
	Group:    "gthulhu.io",
	Version:  "v1alpha1",
	Resource: "podschedulingmetrics",
}

// ---------------------------------------------------------------------------
// PodSchedulingMetrics CRUD
// ---------------------------------------------------------------------------

func (r *repo) CreatePSM(ctx context.Context, psm *domain.PodSchedulingMetrics) error {
	if psm == nil {
		return errors.New("nil psm")
	}
	now := time.Now().UnixMilli()
	if psm.CreatedTime == 0 {
		psm.CreatedTime = now
	}
	psm.UpdatedTime = now

	obj := domainPSMToUnstructured(psm, r.crNamespace)
	_, err := r.k8sDynamic.Resource(psmGVR).Namespace(r.crNamespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create PodSchedulingMetrics CR: %w", err)
	}
	return nil
}

func (r *repo) QueryPSMs(ctx context.Context, opt *domain.QueryPSMOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}

	// Fetch by specific names/IDs.
	if len(opt.IDs) > 0 {
		for _, rawID := range opt.IDs {
			name := fmt.Sprintf("%v", rawID)
			obj, err := r.k8sDynamic.Resource(psmGVR).Namespace(r.crNamespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				return err
			}
			psm, err := unstructuredToDomainPSM(obj)
			if err != nil {
				return err
			}
			opt.Result = append(opt.Result, psm)
		}
		return nil
	}

	// List all.
	list, err := r.k8sDynamic.Resource(psmGVR).Namespace(r.crNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range list.Items {
		psm, err := unstructuredToDomainPSM(&list.Items[i])
		if err != nil {
			return err
		}
		opt.Result = append(opt.Result, psm)
	}
	return nil
}

func (r *repo) UpdatePSM(ctx context.Context, psm *domain.PodSchedulingMetrics) error {
	if psm == nil {
		return errors.New("nil psm")
	}
	obj := domainPSMToUnstructured(psm, r.crNamespace)

	existing, err := r.k8sDynamic.Resource(psmGVR).Namespace(r.crNamespace).Get(ctx, psm.ID.Hex(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get PodSchedulingMetrics CR for update: %w", err)
	}
	obj.SetResourceVersion(existing.GetResourceVersion())

	_, err = r.k8sDynamic.Resource(psmGVR).Namespace(r.crNamespace).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

func (r *repo) DeletePSM(ctx context.Context, name string) error {
	err := r.k8sDynamic.Resource(psmGVR).Namespace(r.crNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func domainPSMToUnstructured(psm *domain.PodSchedulingMetrics, namespace string) *unstructured.Unstructured {
	labelSelectors := make([]interface{}, len(psm.LabelSelectors))
	for i, ls := range psm.LabelSelectors {
		labelSelectors[i] = map[string]interface{}{
			"key":   ls.Key,
			"value": ls.Value,
		}
	}
	k8sNS := make([]interface{}, len(psm.K8sNamespaces))
	for i, ns := range psm.K8sNamespaces {
		k8sNS[i] = ns
	}

	spec := map[string]interface{}{
		"labelSelectors":            labelSelectors,
		"k8sNamespaces":             k8sNS,
		"commandRegex":              psm.CommandRegex,
		"collectionIntervalSeconds": int64(psm.CollectionIntervalSeconds),
		"enabled":                   psm.Enabled,
		"creatorID":                 psm.CreatorID.Hex(),
		"updaterID":                 psm.UpdaterID.Hex(),
		"createdTime":               psm.CreatedTime,
		"updatedTime":               psm.UpdatedTime,
	}

	if psm.Metrics != nil {
		spec["metrics"] = map[string]interface{}{
			"voluntaryCtxSwitches":   psm.Metrics.VoluntaryCtxSwitches,
			"involuntaryCtxSwitches": psm.Metrics.InvoluntaryCtxSwitches,
			"cpuTimeNs":              psm.Metrics.CPUTimeNs,
			"waitTimeNs":             psm.Metrics.WaitTimeNs,
			"runCount":               psm.Metrics.RunCount,
			"cpuMigrations":          psm.Metrics.CPUMigrations,
		}
	}

	if psm.Scaling != nil {
		scalingMap := map[string]interface{}{
			"enabled":         psm.Scaling.Enabled,
			"metricName":      psm.Scaling.MetricName,
			"targetValue":     psm.Scaling.TargetValue,
			"minReplicaCount": int64(psm.Scaling.MinReplicaCount),
			"maxReplicaCount": int64(psm.Scaling.MaxReplicaCount),
			"cooldownPeriod":  int64(psm.Scaling.CooldownPeriod),
		}
		if psm.Scaling.ScaleTargetRef != nil {
			scalingMap["scaleTargetRef"] = map[string]interface{}{
				"apiVersion": psm.Scaling.ScaleTargetRef.APIVersion,
				"kind":       psm.Scaling.ScaleTargetRef.Kind,
				"name":       psm.Scaling.ScaleTargetRef.Name,
			}
		}
		spec["scaling"] = scalingMap
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gthulhu.io/v1alpha1",
			"kind":       "PodSchedulingMetrics",
			"metadata": map[string]interface{}{
				"name":      psm.ID.Hex(),
				"namespace": namespace,
				"labels": map[string]interface{}{
					labelCreatorID: psm.CreatorID.Hex(),
				},
			},
			"spec": spec,
		},
	}
}

func unstructuredToDomainPSM(obj *unstructured.Unstructured) (*domain.PodSchedulingMetrics, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil, fmt.Errorf("spec not found in PodSchedulingMetrics CR %s", obj.GetName())
	}

	psm := &domain.PodSchedulingMetrics{
		BaseEntity: domain.BaseEntity{
			CreatedTime: getInt64(spec, "createdTime"),
			UpdatedTime: getInt64(spec, "updatedTime"),
		},
		CommandRegex:              getStr(spec, "commandRegex"),
		CollectionIntervalSeconds: int32(getInt64(spec, "collectionIntervalSeconds")),
		Enabled:                   getBool(spec, "enabled"),
	}

	// Parse ID from metadata name.
	if id, err := parseObjectIDField(map[string]interface{}{"id": obj.GetName()}, "id"); err == nil {
		psm.ID = id
	}

	if cid, err := parseObjectIDField(spec, "creatorID"); err == nil {
		psm.CreatorID = cid
	}
	if uid, err := parseObjectIDField(spec, "updaterID"); err == nil {
		psm.UpdaterID = uid
	}

	// labelSelectors
	if raw, ok := spec["labelSelectors"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, item := range arr {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				psm.LabelSelectors = append(psm.LabelSelectors, domain.LabelSelector{
					Key:   getStr(m, "key"),
					Value: getStr(m, "value"),
				})
			}
		}
	}

	// k8sNamespaces
	if raw, ok := spec["k8sNamespaces"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					psm.K8sNamespaces = append(psm.K8sNamespaces, s)
				}
			}
		}
	}

	// metrics
	if raw, ok := spec["metrics"]; ok {
		if m, ok := raw.(map[string]interface{}); ok {
			psm.Metrics = &domain.PSMMetrics{
				VoluntaryCtxSwitches:   getBool(m, "voluntaryCtxSwitches"),
				InvoluntaryCtxSwitches: getBool(m, "involuntaryCtxSwitches"),
				CPUTimeNs:              getBool(m, "cpuTimeNs"),
				WaitTimeNs:             getBool(m, "waitTimeNs"),
				RunCount:               getBool(m, "runCount"),
				CPUMigrations:          getBool(m, "cpuMigrations"),
			}
		}
	}

	// scaling
	if raw, ok := spec["scaling"]; ok {
		if m, ok := raw.(map[string]interface{}); ok {
			psm.Scaling = &domain.PSMScaling{
				Enabled:         getBool(m, "enabled"),
				MetricName:      getStr(m, "metricName"),
				TargetValue:     getStr(m, "targetValue"),
				MinReplicaCount: int32(getInt64(m, "minReplicaCount")),
				MaxReplicaCount: int32(getInt64(m, "maxReplicaCount")),
				CooldownPeriod:  int32(getInt64(m, "cooldownPeriod")),
			}
			if ref, ok := m["scaleTargetRef"]; ok {
				if refMap, ok := ref.(map[string]interface{}); ok {
					psm.Scaling.ScaleTargetRef = &domain.PSMScaleTargetRef{
						APIVersion: getStr(refMap, "apiVersion"),
						Kind:       getStr(refMap, "kind"),
						Name:       getStr(refMap, "name"),
					}
				}
			}
		}
	}

	return psm, nil
}

func getBool(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}
