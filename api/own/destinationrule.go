package own

import (
	v1 "clusterplus.io/clusterplus/api/v1"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	istioapiv1 "istio.io/api/networking/v1alpha3"
	istioclientapiv1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type DestinationRule struct {
	plus   *v1.Plus
	scheme *runtime.Scheme
	logger logr.Logger
	client client.Client
}

func NewDestinationRule(plus *v1.Plus, scheme *runtime.Scheme, client client.Client, logger logr.Logger) *DestinationRule {
	d := &DestinationRule{
		plus:   plus,
		logger: logger.WithValues("Own", "DestinationRule"),
		scheme: scheme,
		client: client}
	return d
}

// apply this own resource, create or update
func (d *DestinationRule) Apply() error {
	obj, err := d.generate()
	if err != nil {
		return err
	}

	exist, found, err := d.exist()
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
		if !reflect.DeepEqual(obj.Spec.Host, found.Spec.Host) ||
			!reflect.DeepEqual(obj.Spec.TrafficPolicy, found.Spec.TrafficPolicy) ||
			!reflect.DeepEqual(obj.Spec.Subsets, found.Spec.Subsets) ||
			!reflect.DeepEqual(obj.Spec.ExportTo, found.Spec.ExportTo) ||
			!reflect.DeepEqual(obj.Spec.WorkloadSelector, found.Spec.WorkloadSelector) {
			d.logger.Info("Updating!")
			if err := d.client.Update(context.TODO(), obj); err != nil {
				return err
			}
		}
	}
	return nil

}

func (d *DestinationRule) UpdateStatus() error {
	return nil
}

func (d *DestinationRule) Type() string {
	return "DestinationRule"
}

func (d *DestinationRule) generate() (*istioclientapiv1.DestinationRule, error) {
	subsets := make([]*istioapiv1.Subset, 0, len(d.plus.Spec.Apps))
	for _, app := range d.plus.Spec.Apps {
		subsets = append(subsets, &istioapiv1.Subset{
			Name:          d.plus.GetAppName(app),
			Labels:        d.plus.GenerateAppLabels(app),
			TrafficPolicy: nil,
		})
	}
	ds := &istioclientapiv1.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.plus.GetName(),
			Namespace: d.plus.GetNamespace(),
		},
		Spec: istioapiv1.DestinationRule{
			Host: fmt.Sprintf("%s.%s.svc.cluster.local", d.plus.GetName(), d.plus.GetNamespace()),
			TrafficPolicy: &istioapiv1.TrafficPolicy{
				LoadBalancer:     d.generateLoadBalancerSettings(),
				ConnectionPool:   d.generateConnectionPoolSettings(),
				OutlierDetection: d.generateOutlierDetection(),
			},
			Subsets: subsets,
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(d.plus, ds, d.scheme); err != nil {
		d.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return ds, nil
}

func (d *DestinationRule) generateLoadBalancerSettings() *istioapiv1.LoadBalancerSettings {
	return &istioapiv1.LoadBalancerSettings{
		LbPolicy: &istioapiv1.LoadBalancerSettings_Simple{Simple: istioapiv1.LoadBalancerSettings_ROUND_ROBIN},
	}
}

func (d *DestinationRule) generateConnectionPoolSettings() *istioapiv1.ConnectionPoolSettings {
	if d.plus.Spec.Policy == nil {
		return nil
	}

	return &istioapiv1.ConnectionPoolSettings{
		//Tcp:  &istioapiv1.ConnectionPoolSettings_TCPSettings{},
		Http: &istioapiv1.ConnectionPoolSettings_HTTPSettings{
			Http1MaxPendingRequests:  d.plus.Spec.Policy.MaxRequest,
			Http2MaxRequests:         d.plus.Spec.Policy.MaxRequest,
			MaxRequestsPerConnection: 0,
		},
	}
}

func (d *DestinationRule) generateOutlierDetection() *istioapiv1.OutlierDetection {
	if d.plus.Spec.Policy == nil || d.plus.Spec.Policy.OutlierDetection == nil {
		return nil
	}

	return &istioapiv1.OutlierDetection{
		Consecutive_5XxErrors: d.plus.Spec.Policy.OutlierDetection.GetConsecutiveErrors(),
		Interval:              d.plus.Spec.Policy.OutlierDetection.GetInterval(),
		BaseEjectionTime:      d.plus.Spec.Policy.OutlierDetection.GetEjectionTime(),
		MaxEjectionPercent:    d.plus.Spec.Policy.OutlierDetection.MaxEjectionPercent,
		MinHealthPercent:      d.plus.Spec.Policy.OutlierDetection.MinHealthPercent,
	}
}

func (d *DestinationRule) exist() (bool, *istioclientapiv1.DestinationRule, error) {

	found := &istioclientapiv1.DestinationRule{}
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
