package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	podLabelCache     = make(map[string]map[string]string)
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
func getKubernetesPodLabels(podUID string, options CommandLineOptions) (map[string]string, error) {
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
			mockData, mockErr := getMockPodLabels(podUID)
			if mockErr == nil {
				// Even mock data is stored in the cache
				podLabelCacheMu.Lock()
				podLabelCache[podUID] = mockData
				podLabelCacheTime[podUID] = time.Now()
				podLabelCacheMu.Unlock()
			}
			return mockData, nil
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
		log.Printf("Error listing namespaces: %v, using mock data", err)
		mockData, _ := getMockPodLabels(podUID)
		return mockData, fmt.Errorf("%w: %v", ErrNamespaceAccess, err)
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
				podLabelCache[podUID] = pod.Labels
				podLabelCacheTime[podUID] = time.Now()
				podLabelCacheMu.Unlock()

				log.Printf("Found and cached labels for pod %s in namespace %s", podUID, ns.Name)
				return pod.Labels, nil
			}
		}
	}

	// If no matching Pod is found, use mock data
	log.Printf("Pod with UID %s not found, using mock data", podUID)
	mockData, _ := getMockPodLabels(podUID)
	return mockData, ErrPodNotFound
}

// Get mock Pod label data (as a fallback solution)
func getMockPodLabels(podUID string) (map[string]string, error) {
	// Mock data - only used when Kubernetes API cannot be accessed
	mockLabels := map[string]map[string]string{
		"65979e01-4cb1-4d08-9dba-45530253ff00": {
			"app":  "monitoring",
			"nf":   "upf",
			"tier": "data-plane",
		},
		"75979e01-4cb1-4d08-9dba-45530253gg00": {
			"app":  "networking",
			"nf":   "smf",
			"tier": "control-plane",
		},
		"85979e01-4cb1-4d08-9dba-45530253hh00": {
			"app":  "database",
			"tier": "storage",
		},
	}

	if labels, exists := mockLabels[podUID]; exists {
		return labels, nil
	}

	// If no mock data is found, return an empty map
	return map[string]string{}, nil
}
