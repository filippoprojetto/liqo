package factory

import (
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	defaultReplicas int32 = 1
)

func ReplicaControllerFromPod(pod *corev1.Pod) *corev1.ReplicationController {
	pod.Labels[virtualKubelet.ReflectedpodKey] = pod.Name

	rc := &corev1.ReplicationController{
		TypeMeta: metav1.TypeMeta{
			Kind: "replicationcontroller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        pod.Name,
			Namespace:   pod.Namespace,
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		},
		Spec: corev1.ReplicationControllerSpec{
			Replicas: &defaultReplicas,
			Selector: pod.Labels,
			Template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      pod.Labels,
					Annotations: pod.Annotations,
				},
				Spec: pod.Spec,
			},
		},
	}

	return rc
}
