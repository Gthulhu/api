package cache

import (
	"sync"
	"time"

	"github.com/Gthulhu/api/util"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kcache "k8s.io/client-go/tools/cache"
)

var (
	// Define Pod label cache to reduce API call frequency
	podLabelCache     = make(map[string]apiv1.Pod)
	podLabelCacheMu   sync.RWMutex
	podLabelCacheTTL  = 30 * time.Second
	podLabelCacheTime = make(map[string]time.Time)
)

// StartPodWatcher starts watching Kubernetes pod events and invalidates cache on changes
func StartPodWatcher(cache *StrategyCache, kubeClient *kubernetes.Clientset) (stopCh chan struct{}, err error) {
	client := kubeClient
	stopCh = make(chan struct{})

	// Start watching pods in all namespaces using SharedInformer
	go func() {
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
				util.GetLogger().Info("Pod Added event: cache invalidated")
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				if pod, ok := newObj.(*apiv1.Pod); ok {
					podLabelCacheMu.Lock()
					podLabelCache[string(pod.UID)] = *pod
					podLabelCacheTime[string(pod.UID)] = time.Now()
					podLabelCacheMu.Unlock()
				}
				cache.Invalidate()
				util.GetLogger().Info("Pod Updated event: cache invalidated")
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
				util.GetLogger().Info("Pod Deleted event: cache invalidated")
			},
		})

		factory.Start(stopCh)

		// Wait for caches to sync and then keep running; this will handle reconnects internally
		if ok := kcache.WaitForCacheSync(stopCh, podInformer.HasSynced); !ok {
			util.GetLogger().Warn("Pod informer cache sync failed; will continue to retry via client-go mechanisms")
		} else {
			util.GetLogger().Info("Pod informer started successfully")
		}

		// Block forever; use stopCh to stop if we add stop semantics later
		<-stopCh
	}()

	return stopCh, nil
}

// GetKubernetesPod retrieves pod information from the cache if available
func GetKubernetesPod(podUID string) (apiv1.Pod, bool) {
	// Check cache
	podLabelCacheMu.RLock()
	cachedLabels, exists := podLabelCache[podUID]
	cacheTime, timeExists := podLabelCacheTime[podUID]
	podLabelCacheMu.RUnlock()

	// If the cache exists and is not expired, return it directly
	if exists && timeExists && time.Since(cacheTime) < podLabelCacheTTL {
		return cachedLabels, true
	}

	return apiv1.Pod{}, false
}

// SetKubernetesPodCache sets the pod information in the cache
func SetKubernetesPodCache(podUID string, pod apiv1.Pod) {
	podLabelCacheMu.Lock()
	podLabelCache[podUID] = pod
	podLabelCacheTime[podUID] = time.Now()
	podLabelCacheMu.Unlock()
}
