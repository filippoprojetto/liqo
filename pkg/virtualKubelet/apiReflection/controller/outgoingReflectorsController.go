package controller

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/cache"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/outgoing"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"sync"
)

type OutgoingReflectorsController struct {
	*ReflectorsController
}

func NewOutgoingReflectorsController(homeClient, foreignClient kubernetes.Interface, cacheManager *cache.CacheManager,
	outputChan chan apimgmt.ApiEvent,
	namespaceNatting namespacesMapping.MapperController,
	opts map[options.OptionKey]options.Option) OutGoingAPIReflectorsController {
	controller := &OutgoingReflectorsController{
		&ReflectorsController{
			reflectionType:           ri.OutgoingReflection,
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

	for api := range outgoing.ReflectorBuilders {
		controller.apiReflectors[api] = controller.buildOutgoingReflector(api, opts)
	}

	return controller
}

func (c *OutgoingReflectorsController) buildOutgoingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	apiReflector := &reflectors.GenericAPIReflector{
		Api:              api,
		OutputChan:       c.outputChan,
		ForeignClient:    c.foreignClient,
		HomeClient:       c.homeClient,
		CacheManager: c.cacheManager,
		NamespaceNatting: c.namespaceNatting,
	}
	specReflector := outgoing.ReflectorBuilders[api](apiReflector, opts)
	specReflector.SetSpecializedPreProcessingHandlers()

	return specReflector
}

func (c *OutgoingReflectorsController) Start() {
	for {
		select {
		case ns := <-c.namespaceNatting.PollStartOutgoingReflection():
			c.startNamespaceReflection(ns)
			klog.V(2).Infof("outgoing reflection for namespace %v started", ns)
		case ns := <-c.namespaceNatting.PollStopOutgoingReflection():
			c.stopNamespaceReflection(ns)
			klog.V(2).Infof("incoming reflection for namespace %v started", ns)
		}
	}
}

func (c *OutgoingReflectorsController) stopNamespaceReflection(namespace string) {
	close(c.namespacedStops[namespace])
}
