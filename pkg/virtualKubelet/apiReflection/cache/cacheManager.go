package cache

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

type CacheManager struct {
	homeInformers ri.NamespacedAPICaches
	foreignInformers ri.NamespacedAPICaches
}

func (cm *CacheManager) AddHomeApiCaches(namespace string, apiCache ri.APICache) {
	if cm.homeInformers == nil {
		cm.homeInformers = ri.NamespacedAPICaches{}
	}

	cm.homeInformers[namespace] = apiCache
}

func (cm *CacheManager) AddForeignApiCaches(namespace string, apiCache ri.APICache) {
	if cm.foreignInformers == nil {
		cm.foreignInformers = ri.NamespacedAPICaches{}
	}

	cm.foreignInformers[namespace] = apiCache
}

func (cm *CacheManager) AddHomeEventHandlers(api apimgmt.ApiType, namespace string, handlers *cache.ResourceEventHandlerFuncs) {
	informer, ok := cm.homeInformers[namespace][api]
	if !ok {
		klog.Errorf("cannot set handlers, home informer for api %v in namespace %v does not exist", apimgmt.ApiNames[api], namespace)
		return
	}

	informer.AddEventHandler(handlers)
}

func (cm *CacheManager) AddForeignEventHandlers(api apimgmt.ApiType, namespace string, handlers *cache.ResourceEventHandlerFuncs) {
	informer, ok := cm.foreignInformers[namespace][api]
	if !ok {
		klog.Errorf("cannot set handlers, foreign informer for api %v in namespace %v does not exist", apimgmt.ApiNames[api], namespace)
		return
	}

	informer.AddEventHandler(handlers)
}

func (cm *CacheManager) GetHomeNamespacedObject(api apimgmt.ApiType, namespace, key string) (interface{}, error) {
	return GetObject(cm.homeInformers[namespace][api], key)
}

func (cm *CacheManager) GetForeignNamespacedObject(api apimgmt.ApiType, namespace, key string) (interface{}, error) {
	return GetObject(cm.foreignInformers[namespace][api], key)
}

func (cm *CacheManager) ListHomeNamespacedObject(api apimgmt.ApiType, namespace string) []interface{} {
	objects, err := ListObjects(cm.homeInformers[namespace][api])
	if err != nil {
		klog.Errorf("error while listing home objects - ERR: %v", err)
		return nil
	}

	return objects
}

func (cm *CacheManager) ListForeignNamespacedObject(api apimgmt.ApiType, namespace string) []interface{} {
	objects, err := ListObjects(cm.foreignInformers[namespace][api])
	if err != nil {
		klog.Errorf("error while listing foreign objects - ERR: %v", err)
		return nil
	}

	return objects
}

func (cm *CacheManager) ResyncListHomeNamespacedObject(api apimgmt.ApiType, namespace string) []interface{} {
	objects, err := ResyncListObjects(cm.homeInformers[namespace][api])
	if err != nil {
		klog.Errorf("error while listing home objects - ERR: %v", err)
		return nil
	}

	return objects
}

func (cm *CacheManager) ResyncListForeignNamespacedObject(api apimgmt.ApiType, namespace string) []interface{} {
	objects, err := ResyncListObjects(cm.foreignInformers[namespace][api])
	if err != nil {
		klog.Errorf("error while listing foreign objects - ERR: %v", err)
		return nil
	}

	return objects
}
