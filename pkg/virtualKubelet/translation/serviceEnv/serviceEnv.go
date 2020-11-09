package serviceEnv

import (
	apimgmgt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/controller"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/kubernetes/pkg/kubelet/envvars"
)

var masterServices = sets.NewString("kubernetes")

func TranslateServiceEnvVariables(pod *v1.Pod, localNS string, nattedNS string, apiController controller.ApiControllerCacheManager) (*v1.Pod, error) {
	resPod := pod.DeepCopy()

	enableServiceLinks := v1.DefaultEnableServiceLinks
	if resPod.Spec.EnableServiceLinks != nil {
		enableServiceLinks = *resPod.Spec.EnableServiceLinks
	}

	envs, err := getServiceEnvVarMap(localNS, enableServiceLinks, nattedNS, apiController)
	if err != nil {
		return nil, err
	}
	for i, container := range resPod.Spec.Containers {
		resPod.Spec.Containers[i] = *setInContainer(envs, container)
	}
	for i, container := range resPod.Spec.InitContainers {
		resPod.Spec.InitContainers[i] = *setInContainer(envs, container)
	}
	return resPod, nil
}

func setInContainer(envs map[string]string, container v1.Container) *v1.Container {
	for k, v := range envs {
		found := false
		for _, env := range container.Env {
			if env.Name == k {
				found = true
				break
			}
		}
		if !found {
			container.Env = append(container.Env, v1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}
	return container.DeepCopy()
}

func getServiceEnvVarMap(ns string, enableServiceLinks bool, remoteNs string, apiController controller.ApiControllerCacheManager) (map[string]string, error) {
	var (
		serviceMap = make(map[string]*v1.Service)
		m          = make(map[string]string)
	)

	// search for services in the same namespaces of the pod
	services := apiController.ListMirroredObjects(apimgmgt.Services, ns)

	var err error

	// project the services in namespace ns onto the master services
	for i := range services {
		// We always want to add environment variables for master kubernetes service
		// from the default namespace, even if enableServiceLinks is false.
		// We also add environment variables for other services in the same
		// namespace, if enableServiceLinks is true.

		tmp := services[i]
		service, ok := tmp.(*v1.Service)
		if !ok {
			continue
		}
		serviceName := service.Name

		if service.Namespace == metav1.NamespaceDefault && masterServices.Has(serviceName) {
			// TODO: we have to set env vars for our home kubernetes service
		} else if service.Namespace == ns && enableServiceLinks {
			err = addService(&serviceMap, apiController, remoteNs, serviceName, true)
		}
		if err != nil {
			continue
		}
	}

	mappedServices := make([]*v1.Service, 0, len(serviceMap))
	for key := range serviceMap {
		mappedServices = append(mappedServices, serviceMap[key])
	}

	for _, e := range envvars.FromServices(mappedServices) {
		m[e.Name] = e.Value
	}
	return m, nil
}

func addService(serviceMap *map[string]*v1.Service, apiController controller.ApiControllerCacheManager, namespace string, name string, checkNamespace bool) error {
	tmp, err := apiController.GetMirroringObjectByKey(apimgmgt.Services, namespace, name)
	if err != nil {
		return err
	}
	if tmp == nil {
		return nil
	}
	remoteSvc := tmp.(*v1.Service)
	// ignore services where ClusterIP is "None" or empty
	if !v1helper.IsServiceIPSet(remoteSvc) {
		return nil
	}

	if _, exists := (*serviceMap)[name]; !exists && (!checkNamespace || remoteSvc.Namespace == namespace) {
		(*serviceMap)[name] = remoteSvc
	}
	return nil
}
