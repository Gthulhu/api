package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kcache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// Global Kubernetes client, can be reused throughout the application after initialization
	kubeClient *kubernetes.Clientset

	// Define error types
	ErrNoKubeConfig      = errors.New("no Kubernetes configuration available")
	ErrKubeClientNotInit = errors.New("Kubernetes client not initialized")
	ErrNamespaceAccess   = errors.New("failed to access Kubernetes namespaces")
	ErrPodAccess         = errors.New("failed to access Kubernetes pods")
	ErrPodNotFound       = errors.New("pod not found in any namespace")

	// Define Pod label cache to reduce API call frequency
	podLabelCache     = make(map[string]apiv1.Pod)
	podLabelCacheMu   sync.RWMutex
	podLabelCacheTTL  = 30 * time.Second
	podLabelCacheTime = make(map[string]time.Time)

	// Control Kubernetes client status
	kubeClientMu sync.RWMutex
)

// Initialize Kubernetes client
// Supports two modes:
// 1. When running inside the cluster, use in-cluster configuration
// 2. When running outside the cluster, use kubeconfig configuration
func initKubernetesClient(options CommandLineOptions) error {
	kubeClientMu.Lock()
	defer kubeClientMu.Unlock()

	var config *rest.Config
	var err error

	// Decide which configuration to use based on command line options
	if options.InCluster {
		// Use in-cluster configuration
		log.Println("Using in-cluster Kubernetes configuration")
		config, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to create in-cluster config: %w", err)
		}
	} else if options.KubeConfigPath != "" {
		// Use the specified kubeconfig file
		log.Printf("Using Kubernetes config from: %s", options.KubeConfigPath)
		config, err = clientcmd.BuildConfigFromFlags("", options.KubeConfigPath)
		if err != nil {
			return fmt.Errorf("failed to build kubeconfig from %s: %w", options.KubeConfigPath, err)
		}
	} else {
		// Cannot access Kubernetes
		return ErrNoKubeConfig
	}

	// Create Kubernetes client
	config.Timeout = 10 * time.Second
	config.QPS = 20
	config.Burst = 50

	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	log.Println("Kubernetes client initialized successfully")
	return nil
}

// Verify if the Kubernetes connection is normal
func verifyKubernetesConnection() {
	for {
		time.Sleep(30 * time.Second)

		kubeClientMu.RLock()
		client := kubeClient
		kubeClientMu.RUnlock()

		if client == nil {
			log.Println("Kubernetes client not initialized, skipping connection verification")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil {
			log.Printf("Warning: Kubernetes connection verification failed: %v", err)
			// Do not reset the client, but log the error
		} else {
			log.Println("Kubernetes connection verified successfully")
		}
	}
}

// Get Pod labels from Kubernetes API, supports caching
func getKubernetesPod(podUID string, options CommandLineOptions) (apiv1.Pod, error) {
	// Check cache
	podLabelCacheMu.RLock()
	cachedLabels, exists := podLabelCache[podUID]
	cacheTime, timeExists := podLabelCacheTime[podUID]
	podLabelCacheMu.RUnlock()

	// If the cache exists and is not expired, return it directly
	if exists && timeExists && time.Since(cacheTime) < podLabelCacheTTL {
		log.Printf("Using cached labels for pod %s", podUID)
		return cachedLabels, nil
	}

	// Check Kubernetes client
	kubeClientMu.RLock()
	client := kubeClient
	kubeClientMu.RUnlock()

	if client == nil {
		// If the client is not initialized, try to initialize it
		if err := initKubernetesClient(options); err != nil {
			// Use mock data if initialization fails
			log.Printf("Warning: Kubernetes client initialization failed: %v, using mock data", err)
			podLabelCacheMu.Lock()
			podLabelCache[podUID] = apiv1.Pod{}
			podLabelCacheTime[podUID] = time.Now()
			podLabelCacheMu.Unlock()
			return apiv1.Pod{}, nil
		}

		kubeClientMu.RLock()
		client = kubeClient
		kubeClientMu.RUnlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all namespaces
	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing namespaces: %v", err)
		return apiv1.Pod{}, fmt.Errorf("%w: %v", ErrNamespaceAccess, err)
	}

	// Find the Pod that matches the UID in all namespaces
	for _, ns := range namespaces.Items {
		pods, err := client.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("Error listing pods in namespace %s: %v", ns.Name, err)
			continue
		}

		for _, pod := range pods.Items {
			// Compare Pod UID
			if string(pod.UID) == podUID {
				// Update cache
				podLabelCacheMu.Lock()
				podLabelCache[podUID] = pod
				podLabelCacheTime[podUID] = time.Now()
				podLabelCacheMu.Unlock()

				log.Printf("Found and cached labels for pod %s in namespace %s", podUID, ns.Name)
				return pod, nil
			}
		}
	}

	return apiv1.Pod{}, ErrPodNotFound
}

// StartPodWatcher starts watching Kubernetes pod events and invalidates cache on changes
func StartPodWatcher(cache *StrategyCache) error {
	kubeClientMu.RLock()
	client := kubeClient
	kubeClientMu.RUnlock()

	if client == nil {
		return ErrKubeClientNotInit
	}

	// Start watching pods in all namespaces using SharedInformer
	go func() {
		log.Println("Starting Kubernetes pod watcher (SharedInformer)...")

		// Shared informer factory across all namespaces; 0 disables periodic resync
		factory := informers.NewSharedInformerFactory(client, 0)
		podInformer := factory.Core().V1().Pods().Informer()

		// Register event handlers
		podInformer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if pod, ok := obj.(*apiv1.Pod); ok {
					// Update label cache
					podLabelCacheMu.Lock()
					podLabelCache[string(pod.UID)] = *pod
					podLabelCacheTime[string(pod.UID)] = time.Now()
					podLabelCacheMu.Unlock()
				}
				cache.Invalidate()
				log.Printf("Pod Added event: cache invalidated")
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				if pod, ok := newObj.(*apiv1.Pod); ok {
					podLabelCacheMu.Lock()
					podLabelCache[string(pod.UID)] = *pod
					podLabelCacheTime[string(pod.UID)] = time.Now()
					podLabelCacheMu.Unlock()
				}
				cache.Invalidate()
				log.Printf("Pod Updated event: cache invalidated")
			},
			DeleteFunc: func(obj interface{}) {
				switch t := obj.(type) {
				case *apiv1.Pod:
					podLabelCacheMu.Lock()
					delete(podLabelCache, string(t.UID))
					delete(podLabelCacheTime, string(t.UID))
					podLabelCacheMu.Unlock()
				case kcache.DeletedFinalStateUnknown:
					if pod, ok := t.Obj.(*apiv1.Pod); ok {
						podLabelCacheMu.Lock()
						delete(podLabelCache, string(pod.UID))
						delete(podLabelCacheTime, string(pod.UID))
						podLabelCacheMu.Unlock()
					}
				}
				cache.Invalidate()
				log.Printf("Pod Deleted event: cache invalidated")
			},
		})

		stopCh := make(chan struct{})
		factory.Start(stopCh)

		// Wait for caches to sync and then keep running; this will handle reconnects internally
		if ok := kcache.WaitForCacheSync(stopCh, podInformer.HasSynced); !ok {
			log.Printf("Pod informer cache sync failed; will continue to retry via client-go mechanisms")
		} else {
			log.Println("Pod informer started successfully")
		}

		// Block forever; use stopCh to stop if we add stop semantics later
		<-stopCh
	}()

	return nil
}
