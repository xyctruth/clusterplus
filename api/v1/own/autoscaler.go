package own

import (
	v1 "clusterplus.io/clusterplus/api/v1"
	"context"
	"github.com/go-logr/logr"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type AutoScaling struct {
	plus   *v1.Plus
	scheme *runtime.Scheme
	logger logr.Logger
	client client.Client
}

func NewAutoScaling(plus *v1.Plus, scheme *runtime.Scheme, client client.Client, logger logr.Logger) *AutoScaling {
	d := &AutoScaling{
		plus:   plus,
		logger: logger.WithValues("Own", "AutoScaling"),
		scheme: scheme,
		client: client}
	return d
}

// Apply this own resource, create or update
func (r *AutoScaling) Apply() error {
	for _, app := range r.plus.Spec.Apps {
		obj, err := r.generate(app)
		if err != nil {
			return err
		}

		if obj == nil {
			return nil
		}

		exist, found, err := r.exist(app)
		if err != nil {
			return err
		}

		if *obj.Spec.MinReplicas == -1 {
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
			return nil

		} else {
			obj.ResourceVersion = found.ResourceVersion
			if !reflect.DeepEqual(obj.Spec, found.Spec) {
				r.logger.Info("Updating!")
				if err := r.client.Update(context.TODO(), obj); err != nil {
					return err
				}
			}
		}
	}
	return nil

}

func (r *AutoScaling) UpdateStatus() error {
	return nil
}

func (r *AutoScaling) Type() string {
	return "AutoScaling"
}

func (r *AutoScaling) generate(app *v1.PlusApp) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	if app.Scale.Type == "keda" {
		return nil, nil
	}

	targetCPUUtilizationPercentage := int32(80)
	autoscaling := &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.plus.GetAppName(app),
			Namespace: r.plus.GetNamespace(),
		},
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
			MaxReplicas: app.MaxReplicas,
			MinReplicas: &app.MinReplicas,
			ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       r.plus.GetAppName(app),
			},
			TargetCPUUtilizationPercentage: &targetCPUUtilizationPercentage,
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(r.plus, autoscaling, r.scheme); err != nil {
		r.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return autoscaling, nil
}

func (r *AutoScaling) exist(app *v1.PlusApp) (bool, *autoscalingv1.HorizontalPodAutoscaler, error) {

	found := &autoscalingv1.HorizontalPodAutoscaler{}
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
