package controller

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	reflectionCache "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/cache"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"sync"
	"time"
)

type APIReflectorsController interface {
	Stop()
	DispatchEvent(event apimgmt.ApiEvent)
}

type SpecializedAPIReflectorsController interface {
	Start()
}

type OutGoingAPIReflectorsController interface {
	APIReflectorsController
	SpecializedAPIReflectorsController

	buildOutgoingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector
}

type IncomingAPIReflectorsController interface {
	APIReflectorsController
	SpecializedAPIReflectorsController

	buildIncomingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector
	SetInforming(api apimgmt.ApiType, handler func(interface{}))
}

var defaultResyncPeriod = 10 * time.Second

type ReflectorsController struct {
	reflectionType           ri.ReflectionType
	outputChan               chan apimgmt.ApiEvent
	homeClient               kubernetes.Interface
	foreignClient            kubernetes.Interface
	homeInformerFactories    map[string]informers.SharedInformerFactory
	foreignInformerFactories map[string]informers.SharedInformerFactory
	cacheManager             *reflectionCache.CacheManager
	apiReflectors            map[apimgmt.ApiType]ri.APIReflector
	reflectionGroup          *sync.WaitGroup
	namespaceNatting         namespacesMapping.MapperController
	namespacedStops          map[string]chan struct{}
}

func (c *ReflectorsController) startNamespaceReflection(namespace string) {
	nattedNs, err := c.namespaceNatting.NatNamespace(namespace, false)
	if err != nil {
		klog.Errorf("error while natting namespace - ERR: %v", err)
		return
	}

	homeFactory := informers.NewSharedInformerFactoryWithOptions(c.homeClient, defaultResyncPeriod, informers.WithNamespace(namespace))
	foreignFactory := informers.NewSharedInformerFactoryWithOptions(c.foreignClient, defaultResyncPeriod, informers.WithNamespace(nattedNs))

	c.homeInformerFactories[namespace] = homeFactory
	c.foreignInformerFactories[nattedNs] = foreignFactory

	homeInformers := make(map[apimgmt.ApiType]cache.SharedIndexInformer)
	foreignInformers := make(map[apimgmt.ApiType]cache.SharedIndexInformer)

	for api, handler := range reflectionCache.InformerBuilders {
		homeInformer := handler(homeFactory)
		foreignInformer := handler(foreignFactory)

		if indexer, ok := reflectionCache.InformerIndexers[api]; ok {
			if err := homeInformer.AddIndexers(indexer()); err != nil {
				klog.Errorf("Error while setting up home informer - ERR: %v", err)
			}

			if err := foreignInformer.AddIndexers(indexer()); err != nil {
				klog.Errorf("Error while setting up foreign informer - ERR: %v", err)
			}
		}

		homeInformers[api] = homeInformer
		foreignInformers[api] = foreignInformer

		c.cacheManager.AddHomeApiCaches(namespace, homeInformers)
		c.cacheManager.AddForeignApiCaches(namespace, foreignInformers)
		c.apiReflectors[api].SetupHandlers(api, c.reflectionType, namespace, nattedNs)
	}

	c.namespacedStops[namespace] = make(chan struct{})
	c.reflectionGroup.Add(1)
	go func() {
		c.homeInformerFactories[namespace].Start(c.namespacedStops[namespace])
		c.foreignInformerFactories[nattedNs].Start(c.namespacedStops[namespace])

		<-c.namespacedStops[namespace]

		for _, reflector := range c.apiReflectors {
			reflector.(ri.SpecializedAPIReflector).CleanupNamespace(namespace)
		}

		delete(c.homeInformerFactories, namespace)
		delete(c.foreignInformerFactories, nattedNs)
		c.reflectionGroup.Done()
	}()
}

func (c *ReflectorsController) DispatchEvent(event apimgmt.ApiEvent) {
	c.apiReflectors[event.Api].(ri.SpecializedAPIReflector).HandleEvent(event.Event)
}

func (c *ReflectorsController) Stop() {
	for _, stop := range c.namespacedStops {
		close(stop)
	}
	c.reflectionGroup.Wait()
}
