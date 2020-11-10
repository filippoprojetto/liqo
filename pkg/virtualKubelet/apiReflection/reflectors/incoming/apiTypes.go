package incoming

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
	"k8s.io/client-go/tools/cache"
)

var ReflectorBuilder = map[apimgmt.ApiType]func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector{
	apimgmt.Pods: podsReflectorBuilder,
}

func podsReflectorBuilder(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector {
	return &PodsIncomingReflector{
		APIReflector:          reflector,
		RemoteRemappedPodCIDR: opts[types.RemoteRemappedPodCIDR]}
}

var InformerSelectors = map[apimgmt.ApiType]func(homeIn, foreignIn ri.APICache) (ri.APICache, ri.APICache){
	apimgmt.Pods:               podsInformerSelector,
	apimgmt.ReplicaControllers: replicationControllerInformerSelector,
}

func podsInformerSelector(homeIn, foreignIn ri.APICache) (ri.APICache, ri.APICache) {
	homeOut := make(map[apimgmt.ApiType]cache.SharedIndexInformer)
	foreignOut := make(map[apimgmt.ApiType]cache.SharedIndexInformer)

	homeOut[apimgmt.Pods] = homeIn[apimgmt.Pods]
	foreignOut[apimgmt.Pods] = foreignIn[apimgmt.Pods]

	return homeOut, foreignOut
}

func replicationControllerInformerSelector(homeIn, foreignIn ri.APICache) (ri.APICache, ri.APICache) {
	homeOut := make(map[apimgmt.ApiType]cache.SharedIndexInformer)
	foreignOut := make(map[apimgmt.ApiType]cache.SharedIndexInformer)

	homeOut[apimgmt.ReplicaControllers] = homeIn[apimgmt.ReplicaControllers]
	foreignOut[apimgmt.ReplicaControllers] = foreignIn[apimgmt.ReplicaControllers]

	return homeOut, foreignOut
}
