package incoming

import (
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"strings"
)

type ReplicationControllerIncomingReflector struct {
	ri.APIReflector
}

func (r *ReplicationControllerIncomingReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		AddFunc:    r.preAdd,
		UpdateFunc: r.preUpdate,
		DeleteFunc: r.preDelete,
	})
}

func (r *ReplicationControllerIncomingReflector) HandleEvent(_ interface{}) {}



func (r *ReplicationControllerIncomingReflector) preAdd(_ interface{}) interface{} {
	return nil
}

func (r *ReplicationControllerIncomingReflector) preUpdate(_, _ interface{}) interface{} {
	return nil
}

func (r *ReplicationControllerIncomingReflector) preDelete(_ interface{}) interface{} {
	return nil
}

func (r *ReplicationControllerIncomingReflector) GetMirroredObject(namespace, name string) interface{} {
	informer := r.ForeignInformer(namespace)
	if informer == nil {
		return nil
	}

	key := r.Keyer(namespace, name)
	obj, err := r.GetObjFromForeignCache(namespace, key)
	if err != nil {
		err = errors.Wrapf(err, "replication controller %v", key)
		klog.Error(err)
		return nil
	}

	return obj.(*corev1.ReplicationController).DeepCopy()
}

func (r *ReplicationControllerIncomingReflector) KeyerFromObj(obj interface{}, remoteNamespace string) string {
	cm, ok := obj.(*corev1.ReplicationController)
	if !ok {
		return ""
	}
	return strings.Join([]string{remoteNamespace, cm.Name}, "/")
}

func (r *ReplicationControllerIncomingReflector) ListMirroredObjects(namespace string) []interface{} {
	return r.ForeignInformer(namespace).GetStore().List()
}

func (r *ReplicationControllerIncomingReflector) CleanupNamespace(_ string) {}

func AddReplicationControllerIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["replicationcontrollers"] = func(obj interface{}) ([]string, error) {
		rc, ok := obj.(*corev1.ReplicationController)
		if !ok {
			return []string{}, errors.New("cannot convert obj to replicationController")
		}
		return []string{
			strings.Join([]string{rc.Namespace, rc.Name}, "/"),
			rc.Name,
		}, nil
	}
	return i
}
