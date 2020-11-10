package reflectorsInterfaces

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type ReflectionType int

const (
	OutgoingReflection ReflectionType = iota
	IncomingReflection
)

type APIPreProcessing interface {
	PreProcessIsAllowed(obj interface{}) bool
	PreProcessAdd(obj interface{}) interface{}
	PreProcessUpdate(newObj, oldObj interface{}) interface{}
	PreProcessDelete(obj interface{}) interface{}
}

type APIReflector interface {
	APIPreProcessing

	Inform(obj apimgmt.ApiEvent)
	Keyer(namespace, name string) string

	GetForeignClient() kubernetes.Interface
	GetHomeClient() kubernetes.Interface
	GetCacheManager() CacheManagerReader
	NattingTable() namespacesMapping.NamespaceNatter
	SetupHandlers(api apimgmt.ApiType, reflectionType ReflectionType, namespace, nattedNs string)
	SetPreProcessingHandlers(PreProcessingHandlers)

	SetInforming(handler func(interface{}))
	PushToInforming(interface{})
}

type SpecializedAPIReflector interface {
	SetSpecializedPreProcessingHandlers()
	HandleEvent(interface{})
	KeyerFromObj(obj interface{}, remoteNamespace string) string
	CleanupNamespace(namespace string)
}

type OutgoingAPIReflector interface {
	APIReflector
	SpecializedAPIReflector
}

type IncomingAPIReflector interface {
	APIReflector
	SpecializedAPIReflector
}

type PreProcessingHandlers struct {
	IsAllowed  func(obj interface{}) bool
	AddFunc    func(obj interface{}) interface{}
	UpdateFunc func(newObj, oldObj interface{}) interface{}
	DeleteFunc func(obj interface{}) interface{}
}

type APICache map[apimgmt.ApiType]cache.SharedIndexInformer
type NamespacedAPICaches map[string]APICache

type CacheManagerAdder interface {
	AddHomeApiCaches(namespace string, apiCache APICache)
	AddForeignApiCaches(namespace string, apiCache APICache)
	AddHomeEventHandlers(apimgmt.ApiType, string, *cache.ResourceEventHandlerFuncs)
	AddForeignEventHandlers(apimgmt.ApiType, string, *cache.ResourceEventHandlerFuncs)
}

type CacheManagerReader interface {
	GetHomeNamespacedObject(api apimgmt.ApiType, namespace, key string) (interface{}, error)
	GetForeignNamespacedObject(api apimgmt.ApiType, namespace, key string) (interface{}, error)
	ListHomeNamespacedObject(api apimgmt.ApiType, namespace string) []interface{}
	ListForeignNamespacedObject(api apimgmt.ApiType, namespace string) []interface{}
	ResyncListHomeNamespacedObject(api apimgmt.ApiType, namespace string) []interface{}
	ResyncListForeignNamespacedObject(api apimgmt.ApiType, namespace string) []interface{}
}

type CacheManagerReaderAdder interface {
	CacheManagerAdder
	CacheManagerReader
}
