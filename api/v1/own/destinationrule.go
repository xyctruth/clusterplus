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

// Apply apply this own resource, create or update
func (r *DestinationRule) Apply() error {
	obj, err := r.generate()
	if err != nil {
		return err
	}

	exist, found, err := r.exist()
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
		if !reflect.DeepEqual(obj.Spec.Host, found.Spec.Host) ||
			!reflect.DeepEqual(obj.Spec.TrafficPolicy, found.Spec.TrafficPolicy) ||
			!reflect.DeepEqual(obj.Spec.Subsets, found.Spec.Subsets) ||
			!reflect.DeepEqual(obj.Spec.ExportTo, found.Spec.ExportTo) ||
			!reflect.DeepEqual(obj.Spec.WorkloadSelector, found.Spec.WorkloadSelector) {
			r.logger.Info("Updating!")
			if err := r.client.Update(context.TODO(), obj); err != nil {
				return err
			}
		}
	}
	return nil

}

func (r *DestinationRule) UpdateStatus() error {
	return nil
}

func (r *DestinationRule) Type() string {
	return "DestinationRule"
}

func (r *DestinationRule) generate() (*istioclientapiv1.DestinationRule, error) {
	subsets := make([]*istioapiv1.Subset, 0, len(r.plus.Spec.Apps))
	for _, app := range r.plus.Spec.Apps {
		subsets = append(subsets, &istioapiv1.Subset{
			Name:          r.plus.GetAppName(app),
			Labels:        r.plus.GenerateAppLabels(app),
			TrafficPolicy: nil,
		})
	}
	ds := &istioclientapiv1.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.plus.GetName(),
			Namespace: r.plus.GetNamespace(),
		},
		Spec: istioapiv1.DestinationRule{
			Host: fmt.Sprintf("%s.%s.svc.cluster.local", r.plus.GetName(), r.plus.GetNamespace()),
			TrafficPolicy: &istioapiv1.TrafficPolicy{
				LoadBalancer:     r.generateLoadBalancerSettings(),
				ConnectionPool:   r.generateConnectionPoolSettings(),
				OutlierDetection: r.generateOutlierDetection(),
			},
			Subsets: subsets,
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(r.plus, ds, r.scheme); err != nil {
		r.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return ds, nil
}

func (r *DestinationRule) generateLoadBalancerSettings() *istioapiv1.LoadBalancerSettings {
	return &istioapiv1.LoadBalancerSettings{
		LbPolicy: &istioapiv1.LoadBalancerSettings_Simple{Simple: istioapiv1.LoadBalancerSettings_ROUND_ROBIN},
	}
}

func (r *DestinationRule) generateConnectionPoolSettings() *istioapiv1.ConnectionPoolSettings {
	if r.plus.Spec.Policy == nil {
		return nil
	}

	return &istioapiv1.ConnectionPoolSettings{
		//Tcp:  &istioapiv1.ConnectionPoolSettings_TCPSettings{},
		Http: &istioapiv1.ConnectionPoolSettings_HTTPSettings{
			Http1MaxPendingRequests:  r.plus.Spec.Policy.MaxRequest,
			Http2MaxRequests:         r.plus.Spec.Policy.MaxRequest,
			MaxRequestsPerConnection: 0,
		},
	}
}

func (r *DestinationRule) generateOutlierDetection() *istioapiv1.OutlierDetection {
	if r.plus.Spec.Policy == nil || r.plus.Spec.Policy.OutlierDetection == nil {
		return nil
	}

	return &istioapiv1.OutlierDetection{
		Consecutive_5XxErrors: r.plus.Spec.Policy.OutlierDetection.GetConsecutiveErrors(),
		Interval:              r.plus.Spec.Policy.OutlierDetection.GetInterval(),
		BaseEjectionTime:      r.plus.Spec.Policy.OutlierDetection.GetEjectionTime(),
		MaxEjectionPercent:    r.plus.Spec.Policy.OutlierDetection.MaxEjectionPercent,
		MinHealthPercent:      r.plus.Spec.Policy.OutlierDetection.MinHealthPercent,
	}
}

func (r *DestinationRule) exist() (bool, *istioclientapiv1.DestinationRule, error) {

	found := &istioclientapiv1.DestinationRule{}
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
