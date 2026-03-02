package k8sadapter

import "k8s.io/client-go/dynamic"

// NewDynamicClient creates a Kubernetes dynamic client using the same
// configuration strategy as the pod informer (in-cluster or kubeconfig file).
func NewDynamicClient(opt Options) (dynamic.Interface, error) {
	cfg, err := buildConfig(opt)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(cfg)
}
