package k8sadapter

import (
	"time"

	"k8s.io/client-go/dynamic"
)

// NewDynamicClient creates a Kubernetes dynamic client using the same
// configuration strategy as the pod informer (in-cluster or kubeconfig file).
func NewDynamicClient(opt Options) (dynamic.Interface, error) {
	cfg, err := buildConfig(opt)
	if err != nil {
		return nil, err
	}
	cfg.Timeout = 10 * time.Second
	cfg.QPS = 20
	cfg.Burst = 50
	return dynamic.NewForConfig(cfg)
}
