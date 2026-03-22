package client

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/Gthulhu/api/manager/domain"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

var podSchedulingMetricFamilies = map[string]func(item *domain.PodSchedulingMetricValue, value uint64){
	"gthulhu_pod_voluntary_ctx_switches_total": func(item *domain.PodSchedulingMetricValue, value uint64) {
		item.VoluntaryCtxSwitches = value
	},
	"gthulhu_pod_involuntary_ctx_switches_total": func(item *domain.PodSchedulingMetricValue, value uint64) {
		item.InvoluntaryCtxSwitches = value
	},
	"gthulhu_pod_cpu_time_nanoseconds_total": func(item *domain.PodSchedulingMetricValue, value uint64) {
		item.CPUTimeNs = value
	},
	"gthulhu_pod_wait_time_nanoseconds_total": func(item *domain.PodSchedulingMetricValue, value uint64) {
		item.WaitTimeNs = value
	},
	"gthulhu_pod_run_count_total": func(item *domain.PodSchedulingMetricValue, value uint64) {
		item.RunCount = value
	},
	"gthulhu_pod_cpu_migrations_total": func(item *domain.PodSchedulingMetricValue, value uint64) {
		item.CPUMigrations = value
	},
}

func (dm *DecisionMakerClient) GetPodSchedulingMetricValues(ctx context.Context, decisionMaker *domain.DecisionMakerPod) ([]*domain.PodSchedulingMetricValue, error) {
	endpoint := dm.scheme() + "://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/metrics"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := dm.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("decision maker %s returned non-OK status for metrics endpoint: %s", decisionMaker, resp.Status)
	}

	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse prometheus metrics response: %w", err)
	}

	items := map[string]*domain.PodSchedulingMetricValue{}
	for familyName, family := range families {
		setter, ok := podSchedulingMetricFamilies[familyName]
		if !ok {
			continue
		}

		for _, metric := range family.GetMetric() {
			labels := prometheusLabels(metric)
			podName := firstNonEmpty(labels["pod_name"], labels["pod"])
			namespace := firstNonEmpty(labels["namespace"], labels["kubernetes_namespace"])
			if podName == "" || namespace == "" {
				continue
			}

			nodeID := firstNonEmpty(labels["node_name"], labels["node"], decisionMaker.NodeID)
			key := namespace + "/" + podName + "/" + nodeID
			item, exists := items[key]
			if !exists {
				item = &domain.PodSchedulingMetricValue{
					Namespace: namespace,
					PodName:   podName,
					NodeID:    nodeID,
				}
				items[key] = item
			}

			setter(item, prometheusMetricValue(metric))
		}
	}

	result := make([]*domain.PodSchedulingMetricValue, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		if result[i].PodName != result[j].PodName {
			return result[i].PodName < result[j].PodName
		}
		return result[i].NodeID < result[j].NodeID
	})

	return result, nil
}

func prometheusLabels(metric *dto.Metric) map[string]string {
	labels := make(map[string]string, len(metric.GetLabel()))
	for _, label := range metric.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}
	return labels
}

func prometheusMetricValue(metric *dto.Metric) uint64 {
	switch {
	case metric.Counter != nil:
		return uint64(metric.GetCounter().GetValue())
	case metric.Gauge != nil:
		return uint64(metric.GetGauge().GetValue())
	case metric.Untyped != nil:
		return uint64(metric.GetUntyped().GetValue())
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
