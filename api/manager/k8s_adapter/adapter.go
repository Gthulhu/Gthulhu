package k8sadapter

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/logger"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	_ domain.K8SAdapter = (*Adapter)(nil)
)

type Options struct {
	KubeConfigPath string
	InCluster      bool
}

type Adapter struct {
	client         kubernetes.Interface
	podCache       map[string]apiv1.Pod
	podCacheMu     sync.RWMutex
	stopCh         chan struct{}
	startWatcher   sync.Once
	stopWatcher    sync.Once
	cacheHasSynced atomic.Bool
}

func NewAdapter(opt Options) (*Adapter, error) {
	config, err := buildConfig(opt)
	if err != nil {
		return nil, err
	}

	config.Timeout = 10 * time.Second
	config.QPS = 20
	config.Burst = 50

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	adapter := &Adapter{
		client:   client,
		podCache: make(map[string]apiv1.Pod),
		stopCh:   make(chan struct{}),
	}
	adapter.startPodWatcher()

	return adapter, nil
}

func buildConfig(opt Options) (*rest.Config, error) {
	if opt.InCluster {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("build in-cluster config: %w", err)
		}
		return cfg, nil
	}

	if opt.KubeConfigPath == "" {
		return nil, domain.ErrNoKubeConfig
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", opt.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("build kubeconfig from %s: %w", opt.KubeConfigPath, err)
	}

	return cfg, nil
}

func (a *Adapter) startPodWatcher() {
	a.startWatcher.Do(func() {
		informerFactory := informers.NewSharedInformerFactory(a.client, 0)
		podInformer := informerFactory.Core().V1().Pods().Informer()

		podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod, ok := obj.(*apiv1.Pod)
				if !ok {
					return
				}
				logger.Logger(context.Background()).Debug().Msgf("pod added: %s/%s", pod.Namespace, pod.Name)
				a.setPodCache(*pod)
			},
			UpdateFunc: func(_, newObj interface{}) {
				pod, ok := newObj.(*apiv1.Pod)
				if !ok {
					return
				}
				logger.Logger(context.Background()).Debug().Msgf("pod updated: %s/%s", pod.Namespace, pod.Name)
				a.setPodCache(*pod)
			},
			DeleteFunc: func(obj interface{}) {
				switch pod := obj.(type) {
				case *apiv1.Pod:
					logger.Logger(context.Background()).Debug().Msgf("pod deleted: %s/%s", pod.Namespace, pod.Name)
					a.deletePodCache(string(pod.UID))
				case cache.DeletedFinalStateUnknown:
					if p, ok := pod.Obj.(*apiv1.Pod); ok {
						a.deletePodCache(string(p.UID))
					}
				}
			},
		})

		informerFactory.Start(a.stopCh)

		synced := cache.WaitForCacheSync(a.stopCh, podInformer.HasSynced)
		a.cacheHasSynced.Store(synced)
		logger.Logger(context.Background()).Info().Msg("starting k8s pod watcher")
	})
}

func (a *Adapter) StopPodWatcher() {
	a.stopWatcher.Do(func() {
		if a.stopCh != nil {
			close(a.stopCh)
		}
	})
}

func (a *Adapter) QueryPods(ctx context.Context, opt *domain.QueryPodsOptions) ([]*domain.Pod, error) {
	if opt == nil {
		return nil, domain.ErrNilQueryInput
	}
	if a == nil || a.client == nil {
		return nil, domain.ErrNoClient
	}

	labelSelector := buildLabelSelector(opt.LabelSelectors)
	namespaces := opt.K8SNamespace
	if len(namespaces) == 0 {
		namespaces = []string{metav1.NamespaceAll}
	}

	var cmdRegex *regexp.Regexp
	if opt.CommandRegex != "" {
		re, err := regexp.Compile(opt.CommandRegex)
		if err != nil {
			return nil, fmt.Errorf("compile command regex: %w", err)
		}
		cmdRegex = re
	}

	pods, err := a.listPods(ctx, namespaces, labelSelector)
	if err != nil {
		return nil, err
	}
	results := make([]*domain.Pod, 0, len(pods))

	for _, pod := range pods {
		containers := buildContainers(pod, cmdRegex)
		if cmdRegex != nil && len(containers) == 0 {
			continue
		}

		podLabels := copyLabels(pod.Labels)
		result := &domain.Pod{
			Name:         pod.Name,
			K8SNamespace: pod.Namespace,
			Labels:       podLabels,
			PodID:        string(pod.UID),
			NodeID:       pod.Spec.NodeName,
			Containers:   containers,
		}
		results = append(results, result)
	}

	return results, nil
}

func (a *Adapter) QueryDecisionMakerPods(ctx context.Context, opt *domain.QueryDecisionMakerPodsOptions) ([]*domain.DecisionMakerPod, error) {
	if opt == nil {
		return nil, domain.ErrNilQueryInput
	}

	if a == nil || a.client == nil {
		return nil, domain.ErrNoClient
	}

	labelSelector := buildLabelSelector([]domain.LabelSelector{opt.DecisionMakerLabel})
	namespaces := opt.K8SNamespace
	if len(namespaces) == 0 {
		namespaces = []string{metav1.NamespaceAll}
	}

	nodeFilters := make(map[string]struct{}, len(opt.NodeIDs))
	for _, id := range opt.NodeIDs {
		nodeFilters[id] = struct{}{}
	}

	pods, err := a.listPods(ctx, namespaces, labelSelector)
	if err != nil {
		return nil, err
	}
	results := make([]*domain.DecisionMakerPod, 0, len(pods))
	for _, pod := range pods {
		if len(nodeFilters) > 0 {
			if _, ok := nodeFilters[pod.Spec.NodeName]; !ok {
				continue
			}
		}

		host := pod.Status.PodIP
		if host == "" {
			host = pod.Status.HostIP
		}

		results = append(results, &domain.DecisionMakerPod{
			NodeID: pod.Spec.NodeName,
			Port:   firstContainerPort(pod),
			Host:   host,
			State:  mapPodState(pod.Status.Phase),
		})
	}

	return results, nil
}

func (a *Adapter) listPods(ctx context.Context, namespaces []string, labelSelector string) ([]apiv1.Pod, error) {
	selector, err := labels.Parse(labelSelector)
	if err != nil {
		return nil, fmt.Errorf("parse label selector %q: %w", labelSelector, err)
	}

	if a.cacheHasSynced.Load() {
		return a.podsFromCache(namespaces, selector), nil
	}

	return a.listPodsLive(ctx, namespaces, labelSelector)
}

func (a *Adapter) podsFromCache(namespaces []string, selector labels.Selector) []apiv1.Pod {
	nsAll := len(namespaces) == 0 || (len(namespaces) == 1 && namespaces[0] == metav1.NamespaceAll)
	nsSet := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		nsSet[ns] = struct{}{}
	}

	a.podCacheMu.RLock()
	defer a.podCacheMu.RUnlock()

	pods := make([]apiv1.Pod, 0, len(a.podCache))
	for _, pod := range a.podCache {
		if !nsAll {
			if _, ok := nsSet[pod.Namespace]; !ok {
				continue
			}
		}
		if selector.String() != "" && !selector.Matches(labels.Set(pod.Labels)) {
			continue
		}
		pods = append(pods, pod)
	}
	return pods
}

func (a *Adapter) listPodsLive(ctx context.Context, namespaces []string, labelSelector string) ([]apiv1.Pod, error) {
	results := make([]apiv1.Pod, 0)
	for _, ns := range namespaces {
		pods, err := a.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return nil, fmt.Errorf("list pods in namespace %s: %w", ns, err)
		}
		for _, pod := range pods.Items {
			a.setPodCache(pod)
			results = append(results, pod)
		}
	}
	return results, nil
}

func (a *Adapter) setPodCache(pod apiv1.Pod) {
	a.podCacheMu.Lock()
	a.podCache[string(pod.UID)] = pod
	a.podCacheMu.Unlock()
}

func (a *Adapter) deletePodCache(uid string) {
	a.podCacheMu.Lock()
	delete(a.podCache, uid)
	a.podCacheMu.Unlock()
}

func buildLabelSelector(selectors []domain.LabelSelector) string {
	labels := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		if selector.Key == "" {
			continue
		}
		if selector.Value == "" {
			labels = append(labels, selector.Key)
			continue
		}
		labels = append(labels, fmt.Sprintf("%s=%s", selector.Key, selector.Value))
	}
	return strings.Join(labels, ",")
}

func buildContainers(pod apiv1.Pod, cmdRegex *regexp.Regexp) []domain.Container {
	statusByName := make(map[string]string, len(pod.Status.ContainerStatuses))
	for _, status := range pod.Status.ContainerStatuses {
		statusByName[status.Name] = status.ContainerID
	}

	result := make([]domain.Container, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		command := append([]string{}, container.Command...)
		command = append(command, container.Args...)

		if cmdRegex != nil && !cmdRegex.MatchString(strings.Join(command, " ")) {
			continue
		}

		result = append(result, domain.Container{
			ContainerID: statusByName[container.Name],
			Name:        container.Name,
			Command:     command,
		})
	}
	return result
}

func copyLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(labels))
	for k, v := range labels {
		cloned[k] = v
	}
	return cloned
}

func firstContainerPort(pod apiv1.Pod) int {
	for _, container := range pod.Spec.Containers {
		if len(container.Ports) == 0 {
			continue
		}
		return int(container.Ports[0].ContainerPort)
	}
	return 0
}

func mapPodState(phase apiv1.PodPhase) domain.NodeState {
	switch phase {
	case apiv1.PodRunning:
		return domain.NodeStateOnline
	case apiv1.PodPending:
		return domain.NodeStateUnknown
	default:
		return domain.NodeStateOffline
	}
}

// ListNodes returns all nodes in the Kubernetes cluster
func (a *Adapter) ListNodes(ctx context.Context) ([]*domain.Node, error) {
	if a == nil || a.client == nil {
		return nil, domain.ErrNoClient
	}

	nodeList, err := a.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	results := make([]*domain.Node, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		status := "Unknown"
		for _, condition := range node.Status.Conditions {
			if condition.Type == apiv1.NodeReady {
				if condition.Status == apiv1.ConditionTrue {
					status = "Ready"
				} else {
					status = "NotReady"
				}
				break
			}
		}

		results = append(results, &domain.Node{
			Name:   node.Name,
			Labels: copyLabels(node.Labels),
			Status: status,
		})
	}

	return results, nil
}
