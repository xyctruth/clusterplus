package own

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "clusterplus.io/clusterplus/api/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Service struct {
	plus   *v1.Plus
	scheme *runtime.Scheme
	logger logr.Logger
	client client.Client
}

func NewService(plus *v1.Plus, scheme *runtime.Scheme, client client.Client, logger logr.Logger) *Service {
	d := &Service{
		plus:   plus,
		logger: logger.WithValues("Own", "Service"),
		scheme: scheme, client: client}
	return d
}

// Apply this own resource, create or update
func (r *Service) Apply() error {
	for _, app := range r.plus.Spec.Apps {
		obj, err := r.generate(app)
		if err != nil {
			return err
		}

		exist, found, err := r.exist(app)
		if err != nil {
			return err
		}

		if !exist {
			r.logger.Info("Not found, create it!")
			if err := r.client.Create(context.TODO(), obj); err != nil {
				return err
			}
			return nil
		} else {
			obj.ResourceVersion = found.ResourceVersion
			if !reflect.DeepEqual(obj.Spec.Ports, found.Spec.Ports) ||
				!reflect.DeepEqual(obj.Spec.Selector, found.Spec.Selector) ||
				!reflect.DeepEqual(obj.Spec.SessionAffinity, found.Spec.SessionAffinity) ||
				!reflect.DeepEqual(obj.Spec.Type, found.Spec.Type) {
				r.logger.Info("Updating!")
				if err := r.client.Update(context.TODO(), obj); err != nil {
					return err
				}
			}
		}
	}
	return nil

}

func (r *Service) UpdateStatus() error {
	return nil
}

func (r *Service) Type() string {
	return "Service"
}

func (r *Service) generate(app *v1.PlusApp) (*corev1.Service, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.plus.GetName(),
			Namespace: r.plus.GetNamespace(),
			Labels:    r.plus.GenerateLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeClusterIP,
			Selector:        r.plus.GenerateLabels(),
			Ports:           r.buildPorts(app),
			SessionAffinity: "None",
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(r.plus, service, r.scheme); err != nil {
		r.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return service, nil
}

// Check if the Service already exists
func (r *Service) exist(app *v1.PlusApp) (bool, *corev1.Service, error) {
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.plus.GetName(), Namespace: r.plus.GetNamespace()}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		r.logger.Error(err, "Found error")
		return true, found, err
	}
	return true, found, nil
}

func (r *Service) buildPorts(app *v1.PlusApp) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, 1)
	ports = append(ports, corev1.ServicePort{
		Name:       fmt.Sprintf("%s-%d", app.Protocol, app.Port),
		Protocol:   corev1.ProtocolTCP,
		Port:       app.Port,
		TargetPort: intstr.FromInt(int(app.Port)),
	})

	if r.plus.Spec.Type == v1.PlusTypeGateway || r.plus.Spec.Type == v1.PlusTypeSvc {
		ports = append(ports, corev1.ServicePort{
			Name:       "pprof",
			Protocol:   corev1.ProtocolTCP,
			Port:       9000,
			TargetPort: intstr.FromInt(9000),
		})
	}
	return ports
}
