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

type AutoScaler struct {
	plus   *v1.Plus
	scheme *runtime.Scheme
	logger logr.Logger
	client client.Client
}

func NewAutoScaler(plus *v1.Plus, scheme *runtime.Scheme, client client.Client, logger logr.Logger) *AutoScaler {
	d := &AutoScaler{
		plus:   plus,
		logger: logger.WithValues("Own", "AutoScaler"),
		scheme: scheme,
		client: client}
	return d
}

// apply this own resource, create or update
func (r *AutoScaler) Apply() error {
	for _, app := range r.plus.Spec.Apps {
		obj, err := r.generate(app)
		if err != nil {
			return err
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

func (r *AutoScaler) UpdateStatus() error {
	return nil
}

func (r *AutoScaler) Type() string {
	return "AutoScaler"
}

func (r *AutoScaler) generate(app *v1.PlusApp) (*autoscalingv1.HorizontalPodAutoscaler, error) {
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

func (r *AutoScaler) exist(app *v1.PlusApp) (bool, *autoscalingv1.HorizontalPodAutoscaler, error) {

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
