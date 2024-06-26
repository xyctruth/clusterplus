package own

import (
	"context"
	"github.com/go-test/deep"
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
func (r *Deployment) Apply() error {
	for _, app := range r.plus.Spec.Apps {
		obj, err := r.generate(app)
		if err != nil {
			return err
		}

		exist, found, err := r.exist(app)
		if err != nil {
			return err
		}

		if *obj.Spec.Replicas == -1 {
			if exist {
				if err := r.client.Delete(context.TODO(), obj); err != nil {
					return err
				}
			}
			continue
		}

		if !exist {
			r.logger.Info("Not found, create it!")
			if err := r.client.Create(context.TODO(), obj); err != nil {
				return err
			}
		} else {
			if *found.Spec.Replicas >= app.MinReplicas && *found.Spec.Replicas <= app.MaxReplicas {
				obj.Spec.Replicas = found.Spec.Replicas
			}

			// 设置重启
			if app.RestartMark == "" {
				obj.Spec.Template.Annotations["apps.clusterplus.io/restart-mark"] = found.Spec.Template.Annotations["apps.clusterplus.io/restart-mark"]
			} else {
				obj.Spec.Template.Annotations["apps.clusterplus.io/restart-mark"] = app.RestartMark
			}

			r1 := obj.Spec.Template.Spec.Containers[0].Resources
			r2 := found.Spec.Template.Spec.Containers[0].Resources

			isMatch := true
			if r1.Requests.Cpu().String() != r2.Requests.Cpu().String() ||
				r1.Requests.Memory().String() != r2.Requests.Memory().String() ||
				r1.Limits.Cpu().String() != r2.Limits.Cpu().String() ||
				r1.Limits.Memory().String() != r2.Limits.Memory().String() {
				isMatch = false
			}

			if isMatch {
				obj.Spec.Template.Spec.Containers[0].Resources = found.Spec.Template.Spec.Containers[0].Resources
				isMatch = reflect.DeepEqual(obj.Spec, found.Spec)
			}

			if !isMatch {
				if diff := deep.Equal(obj.Spec, found.Spec); diff != nil {
					r.logger.Info("Updating!", "Diff", diff)
				} else {
					r.logger.Info("Updating! No Diff:")
				}
				if err := r.client.Update(context.TODO(), obj); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *Deployment) UpdateStatus() error {
	if r.plus.Status.AvailableReplicas == nil {
		r.plus.Status.AvailableReplicas = make(map[string]int32, 0)
	}

	for _, app := range r.plus.Spec.Apps {
		found := &appsv1.Deployment{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.plus.GetAppName(app), Namespace: r.plus.GetNamespace()}, found)
		if err != nil {
			return nil
		}
		r.plus.Status.AvailableReplicas[app.Version] = found.Status.AvailableReplicas
	}

	return nil
}

func (r *Deployment) Type() string {
	return "Deployment"
}

func (r *Deployment) generate(app *v1.PlusApp) (*appsv1.Deployment, error) {
	hostPathType := corev1.HostPathType("")
	progressDeadlineSeconds := int32(600)
	revisionHistoryLimit := int32(10)

	logPath := "/app/logs/"
	logsKey := r.plus.GetName() + "-logs"
	if app.LogPath != "" {
		logPath = app.LogPath
	}

	// 构建k8s Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.plus.GetAppName(app),
			Namespace:   r.plus.GetNamespace(),
			Labels:      r.plus.GenerateAppLabels(app),
			Annotations: r.plus.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			ProgressDeadlineSeconds: &progressDeadlineSeconds,
			RevisionHistoryLimit:    &revisionHistoryLimit,
			Replicas:                r.buildReplicas(app),
			Selector: &metav1.LabelSelector{
				MatchLabels: r.plus.GenerateAppLabels(app),
			},
			Strategy: r.buildStrategy(app),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      r.plus.GenerateAppTemplateLabels(app),
					Annotations: r.buildAnnotations(app),
				},
				Spec: corev1.PodSpec{
					HostAliases: app.HostAliases,
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: app.ImagePullSecrets},
					},
					InitContainers:                nil,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SecurityContext:               &corev1.PodSecurityContext{},
					SchedulerName:                 "default-scheduler",
					TerminationGracePeriodSeconds: r.buildTerminationGracePeriodSeconds(app),
					NodeSelector:                  app.NodeSelector,
					Containers: []corev1.Container{{
						Image:                    r.plus.GetAppImage(app),
						ImagePullPolicy:          corev1.PullAlways,
						Name:                     r.plus.GetAppName(app),
						Ports:                    r.buildPorts(app),
						Resources:                r.buildResources(app.Resources),
						Env:                      r.buildEnv(app.Env),
						Command:                  nil,
						ReadinessProbe:           r.buildReadinessProbe(app),
						LivenessProbe:            r.buildLivelinessProbe(app),
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "tz-config",
								MountPath: "/etc/localtime",
							},
							{
								Name:      logsKey,
								MountPath: logPath,
							},
						},
					}},
					Affinity: app.Affinity,
					Volumes: []corev1.Volume{{
						Name: "tz-config",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/usr/share/zoneinfo/Asia/Shanghai",
								Type: &hostPathType,
							},
						},
					}, {
						Name: logsKey,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}},
					Tolerations: app.Tolerations,
				},
			},
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(r.plus, deployment, r.scheme); err != nil {
		r.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return deployment, nil
}

// Check if the Deployment already exists
func (r *Deployment) exist(app *v1.PlusApp) (bool, *appsv1.Deployment, error) {

	found := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.plus.GetAppName(app), Namespace: r.plus.GetNamespace()}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		r.logger.Error(err, "Found error")
		return true, found, err
	}
	return true, found, nil
}

func (r *Deployment) buildReplicas(app *v1.PlusApp) *int32 {
	return &app.MinReplicas

}

func (r *Deployment) buildEnv(env []corev1.EnvVar) []corev1.EnvVar {
	for i, _ := range env {
		if env[i].ValueFrom != nil && env[i].ValueFrom.FieldRef != nil {
			if env[i].ValueFrom.FieldRef.APIVersion == "" {
				env[i].ValueFrom.FieldRef.APIVersion = "v1"
			}
		}
	}
	return env
}

func (r *Deployment) buildStrategy(app *v1.PlusApp) appsv1.DeploymentStrategy {
	if app.RollingUpdateType == appsv1.RecreateDeploymentStrategyType {
		return appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		}
	}

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

func (r *Deployment) buildTerminationGracePeriodSeconds(app *v1.PlusApp) *int64 {
	t := int64(30)
	if app.TerminationGracePeriodSeconds > 0 {
		t = app.TerminationGracePeriodSeconds
	}
	return &t
}

func (r *Deployment) buildResources(res corev1.ResourceRequirements) corev1.ResourceRequirements {
	return res
}

func (r *Deployment) buildPorts(app *v1.PlusApp) []corev1.ContainerPort {

	ports := make([]corev1.ContainerPort, 0, 1)
	ports = append(ports, corev1.ContainerPort{
		Name:          "",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: app.Port,
	})
	return ports
}

func (r *Deployment) buildReadinessProbe(app *v1.PlusApp) *corev1.Probe {
	return r.buildProbe(app.ReadinessProbe, app.Port)
}

func (r *Deployment) buildLivelinessProbe(app *v1.PlusApp) *corev1.Probe {
	return r.buildProbe(app.LivenessProbe, app.Port)
}

func (r *Deployment) buildProbe(probe *v1.PlusAppProbe, port int32) *corev1.Probe {
	if probe == nil {
		return nil
	}
	p := &corev1.Probe{
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
		TimeoutSeconds:      3,
	}

	if probe.InitialDelaySeconds > 0 {
		p.InitialDelaySeconds = probe.InitialDelaySeconds
	}

	if probe.TimeoutSeconds > 0 {
		p.TimeoutSeconds = probe.TimeoutSeconds
	}

	if len(probe.ExecCommand) > 0 {
		p.Exec = &corev1.ExecAction{
			Command: probe.ExecCommand,
		}
	}

	if probe.HttpPath != "" {
		p.ProbeHandler = corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: probe.HttpPath,
				Port: intstr.IntOrString{
					IntVal: port,
				},
				Scheme: "HTTP",
			},
		}
	}
	return p
}

func (r *Deployment) buildAnnotations(app *v1.PlusApp) map[string]string {
	m := make(map[string]string)
	m["apps.clusterplus.io/restart-mark"] = app.RestartMark
	for k, v := range app.TemplateAnnotations {
		m[k] = v
	}

	if app.ProxyResources.Requests.Cpu().String() != "0" {
		m["sidecar.istio.io/proxyCPU"] = app.ProxyResources.Requests.Cpu().String()
	}

	if app.ProxyResources.Limits.Cpu().String() != "0" {
		m["sidecar.istio.io/proxyCPULimit"] = app.ProxyResources.Limits.Cpu().String()
	}

	if app.ProxyResources.Requests.Memory().String() != "0" {
		m["sidecar.istio.io/proxyMemory"] = app.ProxyResources.Requests.Memory().String()
	}

	if app.ProxyResources.Limits.Memory().String() != "0" {
		m["sidecar.istio.io/proxyMemoryLimit"] = app.ProxyResources.Limits.Memory().String()
	}

	return m
}
