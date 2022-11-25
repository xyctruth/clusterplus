package own

import (
	"context"
	"reflect"

	v1 "clusterplus.io/clusterplus/api/v1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Deployment struct {
	plus   *v1.Plus
	scheme *runtime.Scheme
	logger logr.Logger
	client client.Client
}

func NewDeployment(plus *v1.Plus, scheme *runtime.Scheme, client client.Client, logger logr.Logger) *Deployment {
	d := &Deployment{
		plus:   plus,
		logger: logger.WithValues("Own", "Deployment"),
		scheme: scheme,
		client: client}
	return d
}

// Apply this own resource, create or update
func (d *Deployment) Apply() error {
	for _, app := range d.plus.Spec.Apps {
		obj, err := d.generate(app)
		if err != nil {
			return err
		}

		exist, found, err := d.exist(app)
		if err != nil {
			return err
		}

		if *obj.Spec.Replicas == -1 {
			if exist {
				if err := d.client.Delete(context.TODO(), obj); err != nil {
					return err
				}
			}
			continue
		}

		if !exist {
			d.logger.Info("Not found, create it!")
			if err := d.client.Create(context.TODO(), obj); err != nil {
				return err
			}
		} else {
			if *obj.Spec.Replicas >= app.MinReplicas && *obj.Spec.Replicas <= app.MaxReplicas {
				obj.Spec.Replicas = found.Spec.Replicas
			}

			// 设置重启
			if app.RestartMark == "" {
				obj.Spec.Template.Annotations["apps.clusterplus.io/restart-mark"] = found.Spec.Template.Annotations["apps.clusterplus.io/restart-mark"]
			} else {
				obj.Spec.Template.Annotations["apps.clusterplus.io/restart-mark"] = app.RestartMark
			}

			if !reflect.DeepEqual(obj.Spec, found.Spec) {
				d.logger.Info("Updating!")
				if err := d.client.Update(context.TODO(), obj); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d *Deployment) UpdateStatus() error {
	if d.plus.Status.AvailableReplicas == nil {
		d.plus.Status.AvailableReplicas = make(map[string]int32, 0)
	}

	for _, app := range d.plus.Spec.Apps {
		found := &appsv1.Deployment{}
		err := d.client.Get(context.TODO(), types.NamespacedName{Name: d.plus.GetAppName(app), Namespace: d.plus.GetNamespace()}, found)
		if err != nil {
			return nil
		}
		if !reflect.DeepEqual(d.plus.Status.AvailableReplicas, found.Status.AvailableReplicas) {
			d.plus.Status.AvailableReplicas[app.Name] = found.Status.AvailableReplicas
		}
	}

	return nil
}

func (d *Deployment) Type() string {
	return "Deployment"
}

func (d *Deployment) generate(app *v1.PlusApp) (*appsv1.Deployment, error) {
	hostPathType := corev1.HostPathType("")
	terminationGracePeriodSeconds := int64(30)
	progressDeadlineSeconds := int32(600)
	revisionHistoryLimit := int32(10)
	// 构建k8s Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        d.plus.GetAppName(app),
			Namespace:   d.plus.GetNamespace(),
			Labels:      d.plus.GenerateAppLabels(app),
			Annotations: d.plus.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			ProgressDeadlineSeconds: &progressDeadlineSeconds,
			RevisionHistoryLimit:    &revisionHistoryLimit,
			Replicas:                d.buildReplicas(app),
			Selector: &metav1.LabelSelector{
				MatchLabels: d.plus.GenerateAppLabels(app),
			},
			Strategy: d.buildStrategy(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      d.plus.GenerateAppLabels(app),
					Annotations: d.buildAnnotations(app),
				},
				Spec: corev1.PodSpec{
					InitContainers:                nil,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SecurityContext:               &corev1.PodSecurityContext{},
					SchedulerName:                 "default-scheduler",
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{{
						Image:                    app.Image,
						ImagePullPolicy:          corev1.PullAlways,
						Name:                     d.plus.GetAppName(app),
						Ports:                    d.buildPorts(app),
						Resources:                app.Resources,
						Env:                      app.Env,
						Command:                  nil,
						ReadinessProbe:           d.buildReadinessProbe(app),
						LivenessProbe:            d.buildLivelinessProbe(app),
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "tz-config",
								MountPath: "/etc/localtime",
							},
						},
					}},
					//Affinity: &corev1.Affinity{
					//	NodeAffinity: &corev1.NodeAffinity{
					//		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					//			NodeSelectorTerms: []corev1.NodeSelectorTerm{
					//				{
					//					MatchExpressions: []corev1.NodeSelectorRequirement{
					//						{
					//							Key:      "env",
					//							Operator: corev1.NodeSelectorOpIn,
					//							Values: []string{
					//								"prod",
					//							},
					//						},
					//					},
					//				},
					//			},
					//		},
					//	},
					//},
					Volumes: []corev1.Volume{{
						Name: "tz-config",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/usr/share/zoneinfo/Asia/Shanghai",
								Type: &hostPathType,
							},
						},
					}},
				},
			},
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(d.plus, deployment, d.scheme); err != nil {
		d.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return deployment, nil
}

// Check if the Deployment already exists
func (d *Deployment) exist(app *v1.PlusApp) (bool, *appsv1.Deployment, error) {

	found := &appsv1.Deployment{}
	err := d.client.Get(context.TODO(), types.NamespacedName{Name: d.plus.GetAppName(app), Namespace: d.plus.GetNamespace()}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		d.logger.Error(err, "Found error")
		return true, found, err
	}
	return true, found, nil
}

func (d *Deployment) buildReplicas(app *v1.PlusApp) *int32 {
	return &app.MinReplicas

}

func (d *Deployment) buildStrategy() appsv1.DeploymentStrategy {
	maxSurge := intstr.FromString("25%")
	maxUnavailable := intstr.FromString("25%")
	return appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxSurge:       &maxSurge,
			MaxUnavailable: &maxUnavailable,
		},
	}
}

func (d *Deployment) buildPorts(app *v1.PlusApp) []corev1.ContainerPort {

	ports := make([]corev1.ContainerPort, 0, 1)
	ports = append(ports, corev1.ContainerPort{
		Name:          "",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: app.Port,
	})
	return ports
}

func (d *Deployment) buildReadinessProbe(app *v1.PlusApp) *corev1.Probe {

	//if app.Protocol == "http" {
	//	return &corev1.Probe{
	//		ProbeHandler: corev1.ProbeHandler{
	//			HTTPGet: &corev1.HTTPGetAction{
	//				Path: "/",
	//				Port: intstr.IntOrString{
	//					IntVal: app.Port,
	//				},
	//				Scheme: "HTTP",
	//			},
	//		},
	//		InitialDelaySeconds: 5,
	//		PeriodSeconds:       5,
	//		SuccessThreshold:    1,
	//		FailureThreshold:    10,
	//		TimeoutSeconds:      10,
	//	}
	//}
	//
	//if app.Protocol == "grpc" {
	//	return &corev1.Probe{
	//		ProbeHandler: corev1.ProbeHandler{
	//			Exec: &corev1.ExecAction{
	//				//Command: []string{"/bin/grpc_health_probe", fmt.Sprintf("-addr=:%d", app.Port)},
	//				Command: []string{"ls"},
	//			},
	//		},
	//		InitialDelaySeconds: 5,
	//		PeriodSeconds:       5,
	//		SuccessThreshold:    1,
	//		FailureThreshold:    10,
	//		TimeoutSeconds:      10,
	//	}
	//}
	return nil
}

func (d *Deployment) buildLivelinessProbe(app *v1.PlusApp) *corev1.Probe {
	//if app.Protocol == "http" {
	//	return &corev1.Probe{
	//		ProbeHandler: corev1.ProbeHandler{
	//			HTTPGet: &corev1.HTTPGetAction{
	//				Path: "/ping",
	//				Port: intstr.IntOrString{
	//					IntVal: app.Port,
	//				},
	//				Scheme: "HTTP",
	//			},
	//		},
	//		InitialDelaySeconds: 60,
	//		PeriodSeconds:       10,
	//		SuccessThreshold:    1,
	//		FailureThreshold:    10,
	//		TimeoutSeconds:      10,
	//	}
	//}
	//
	//if app.Protocol == "grpc" {
	//	return &corev1.Probe{
	//		ProbeHandler: corev1.ProbeHandler{
	//			Exec: &corev1.ExecAction{
	//				//Command: []string{"/bin/grpc_health_probe", fmt.Sprintf("-addr=:%d", app.Port)},
	//				Command: []string{"ls"},
	//			},
	//		},
	//		InitialDelaySeconds: 60,
	//		PeriodSeconds:       10,
	//		SuccessThreshold:    1,
	//		FailureThreshold:    10,
	//		TimeoutSeconds:      10,
	//	}
	//}
	return nil
}

func (d *Deployment) buildAnnotations(app *v1.PlusApp) map[string]string {
	m := make(map[string]string)
	m["apps.clusterplus.io/restart-mark"] = app.RestartMark
	return m
}
