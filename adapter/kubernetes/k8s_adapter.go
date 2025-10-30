package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/Gthulhu/api/util"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Define error types
var (
	ErrNoKubeConfig      = errors.New("no Kubernetes configuration available")
	ErrKubeClientNotInit = errors.New("Kubernetes client not initialized")
	ErrNamespaceAccess   = errors.New("failed to access Kubernetes namespaces")
	ErrPodAccess         = errors.New("failed to access Kubernetes pods")
	ErrPodNotFound       = errors.New("pod not found in any namespace")
)

type K8sAdapter interface {
	GetPodByPodUID(ctx context.Context, podUID string) (apiv1.Pod, error)
	GetClient() *kubernetes.Clientset
}

type k8sClient struct {
	kubeClient *kubernetes.Clientset
}

func (k *k8sClient) GetPodByPodUID(ctx context.Context, podUID string) (apiv1.Pod, error) {
	namespaces, err := k.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing namespaces: %v", err)
		return apiv1.Pod{}, fmt.Errorf("%w: %v", ErrNamespaceAccess, err)
	}

	// Find the Pod that matches the UID in all namespaces
	for _, ns := range namespaces.Items {
		pods, err := k.kubeClient.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("Error listing pods in namespace %s: %v", ns.Name, err)
			continue
		}

		for _, pod := range pods.Items {
			// Compare Pod UID
			if string(pod.UID) == podUID {
				// Update cache
				// TODO: implement caching
				// podLabelCacheMu.Lock()
				// podLabelCache[podUID] = pod
				// podLabelCacheTime[podUID] = time.Now()
				// podLabelCacheMu.Unlock()

				log.Printf("Found and cached labels for pod %s in namespace %s", podUID, ns.Name)
				return pod, nil
			}
		}
	}

	return apiv1.Pod{}, ErrPodNotFound
}

func (k *k8sClient) GetClient() *kubernetes.Clientset {
	return k.kubeClient
}

// Options contains Kubernetes adapter options
type Options struct {
	KubeConfigPath string
	InCluster      bool
}

// NewK8SAdapter creates a new Kubernetes adapter based on command line options.
// Supports two modes:
// 1. When running inside the cluster, use in-cluster configuration
// 2. When running outside the cluster, use kubeconfig configuration
func NewK8SAdapter(options Options) (K8sAdapter, error) {

	var config *rest.Config
	var err error

	// Decide which configuration to use based on command line options
	if options.InCluster {
		// Use in-cluster configuration
		util.GetLogger().Info("Using in-cluster Kubernetes configuration")
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
		}
	} else if options.KubeConfigPath != "" {
		// Use the specified kubeconfig file
		util.GetLogger().Info("Using Kubernetes config", slog.String("path", options.KubeConfigPath))
		config, err = clientcmd.BuildConfigFromFlags("", options.KubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig from %s: %w", options.KubeConfigPath, err)
		}
	} else {
		// Cannot access Kubernetes
		return nil, ErrNoKubeConfig
	}

	// Create Kubernetes client
	config.Timeout = 10 * time.Second
	config.QPS = 20
	config.Burst = 50

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &k8sClient{kubeClient: kubeClient}, nil
}
