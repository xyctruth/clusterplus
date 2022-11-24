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

// apply this own resource, create or update
func (d *Service) Apply() error {
	for _, app := range d.plus.Spec.Apps {
		obj, err := d.generate(app)
		if err != nil {
			return err
		}

		exist, found, err := d.exist(app)
		if err != nil {
			return err
		}

		if !exist {
			d.logger.Info("Not found, create it!")
			if err := d.client.Create(context.TODO(), obj); err != nil {
				return err
			}
			return nil
		} else {
			obj.ResourceVersion = found.ResourceVersion
			if !reflect.DeepEqual(obj.Spec.Ports, found.Spec.Ports) ||
				!reflect.DeepEqual(obj.Spec.Selector, found.Spec.Selector) ||
				!reflect.DeepEqual(obj.Spec.SessionAffinity, found.Spec.SessionAffinity) ||
				!reflect.DeepEqual(obj.Spec.Type, found.Spec.Type) {
				d.logger.Info("Updating!")
				if err := d.client.Update(context.TODO(), obj); err != nil {
					return err
				}
			}
		}
	}
	return nil

}

func (d *Service) UpdateStatus() error {
	return nil
}

func (d *Service) Type() string {
	return "Service"
}

func (d *Service) generate(app *v1.PlusApp) (*corev1.Service, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.plus.GetName(),
			Namespace: d.plus.GetNamespace(),
			Labels:    d.plus.GenerateLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeClusterIP,
			Selector:        d.plus.GenerateLabels(),
			Ports:           d.buildPorts(app),
			SessionAffinity: "None",
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(d.plus, service, d.scheme); err != nil {
		d.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return service, nil
}

// Check if the Service already exists
func (d *Service) exist(app *v1.PlusApp) (bool, *corev1.Service, error) {
	found := &corev1.Service{}
	err := d.client.Get(context.TODO(), types.NamespacedName{Name: d.plus.GetName(), Namespace: d.plus.GetNamespace()}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		d.logger.Error(err, "Found error")
		return true, found, err
	}
	return true, found, nil
}

func (d *Service) buildPorts(app *v1.PlusApp) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, 1)
	ports = append(ports, corev1.ServicePort{
		Name:       fmt.Sprintf("%s-%d", app.Protocol, app.Port),
		Protocol:   corev1.ProtocolTCP,
		Port:       app.Port,
		TargetPort: intstr.FromInt(int(app.Port)),
	})

	if d.plus.Spec.Type == v1.PlusTypeGateway || d.plus.Spec.Type == v1.PlusTypeSvc {
		ports = append(ports, corev1.ServicePort{
			Name:       "pprof",
			Protocol:   corev1.ProtocolTCP,
			Port:       9000,
			TargetPort: intstr.FromInt(9000),
		})
	}
	return ports
}
