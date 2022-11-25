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
func (d *AutoScaler) Apply() error {
	for _, app := range d.plus.Spec.Apps {
		obj, err := d.generate(app)
		if err != nil {
			return err
		}

		exist, found, err := d.exist(app)
		if err != nil {
			return err
		}

		if *obj.Spec.MinReplicas == -1 {
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
			return nil

		} else {
			obj.ResourceVersion = found.ResourceVersion
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

func (d *AutoScaler) UpdateStatus() error {
	return nil
}

func (d *AutoScaler) Type() string {
	return "AutoScaler"
}

func (d *AutoScaler) generate(app *v1.PlusApp) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	targetCPUUtilizationPercentage := int32(80)
	autoScaler := &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.plus.GetAppName(app),
			Namespace: d.plus.GetNamespace(),
		},
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
			MaxReplicas: app.MaxReplicas,
			MinReplicas: &app.MinReplicas,
			ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       d.plus.GetAppName(app),
			},
			TargetCPUUtilizationPercentage: &targetCPUUtilizationPercentage,
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(d.plus, autoScaler, d.scheme); err != nil {
		d.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return autoScaler, nil
}

func (d *AutoScaler) exist(app *v1.PlusApp) (bool, *autoscalingv1.HorizontalPodAutoscaler, error) {

	found := &autoscalingv1.HorizontalPodAutoscaler{}
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
