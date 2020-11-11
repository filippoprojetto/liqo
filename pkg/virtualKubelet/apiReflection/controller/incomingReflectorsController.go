package controller

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/cache"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/incoming"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sync"
)

type IncomingReflectorsController struct {
	*ReflectorsController
}

func NewIncomingReflectorsController(homeClient, foreignClient kubernetes.Interface, cacheManager *cache.CacheManager,
	outputChan chan apimgmt.ApiEvent,
	namespaceNatting namespacesMapping.MapperController,
	opts map[options.OptionKey]options.Option) IncomingAPIReflectorsController {
	controller := &IncomingReflectorsController{
		&ReflectorsController{
			reflectionType:           ri.IncomingReflection,
			outputChan:               outputChan,
			homeClient:               homeClient,
			foreignClient:            foreignClient,
			homeInformerFactories:    make(map[string]informers.SharedInformerFactory),
			foreignInformerFactories: make(map[string]informers.SharedInformerFactory),
			apiReflectors:            make(map[apimgmt.ApiType]ri.APIReflector),
			namespaceNatting:         namespaceNatting,
			namespacedStops:          make(map[string]chan struct{}),
			reflectionGroup:          &sync.WaitGroup{},
			cacheManager: cacheManager,
		},
	}

	for api := range incoming.ReflectorBuilder {
		controller.apiReflectors[api] = controller.buildIncomingReflector(api, opts)
	}

	return controller
}

func (c *IncomingReflectorsController) buildIncomingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector {
	apiReflector := &reflectors.GenericAPIReflector{
		Api:              api,
		OutputChan:       c.outputChan,
		ForeignClient:    c.foreignClient,
		HomeClient:       c.homeClient,
		CacheManager: c.cacheManager,
		NamespaceNatting: c.namespaceNatting,
	}
	specReflector := incoming.ReflectorBuilder[api](apiReflector, opts)
	specReflector.SetSpecializedPreProcessingHandlers()

	return specReflector
}

func (c *IncomingReflectorsController) Start() {
	for {
		select {
		case ns := <-c.namespaceNatting.PollStartIncomingReflection():
			c.startNamespaceReflection(ns)
		case ns := <-c.namespaceNatting.PollStopIncomingReflection():
			c.stopNamespaceReflection(ns)
		}
	}
}

func (c *IncomingReflectorsController) SetInforming(api apimgmt.ApiType, handler func(interface{})) {
	c.apiReflectors[api].(ri.APIReflector).SetInforming(handler)
}

func (c *IncomingReflectorsController) stopNamespaceReflection(namespace string) {
	close(c.namespacedStops[namespace])
}
