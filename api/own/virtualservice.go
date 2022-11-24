package own

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"reflect"

	v1 "clusterplus.io/clusterplus/api/v1"
	"github.com/go-logr/logr"
	istioapiv1 "istio.io/api/networking/v1alpha3"
	istioclientapiv1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type VirtualService struct {
	plus   *v1.Plus
	scheme *runtime.Scheme
	logger logr.Logger
	client client.Client
}

func NewVirtualService(plus *v1.Plus, scheme *runtime.Scheme, client client.Client, logger logr.Logger) *VirtualService {
	d := &VirtualService{
		plus:   plus,
		logger: logger.WithValues("Own", "VirtualService"),
		scheme: scheme,
		client: client}
	return d
}

// Apply this own resource, create or update
func (d *VirtualService) Apply() error {
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
		if !reflect.DeepEqual(obj.Spec.Hosts, found.Spec.Hosts) ||
			!reflect.DeepEqual(obj.Spec.Gateways, found.Spec.Gateways) ||
			!reflect.DeepEqual(obj.Spec.Http, found.Spec.Http) ||
			!reflect.DeepEqual(obj.Spec.Tcp, found.Spec.Tcp) ||
			!reflect.DeepEqual(obj.Spec.Tls, found.Spec.Tls) ||
			!reflect.DeepEqual(obj.Spec.ExportTo, found.Spec.ExportTo) {
			obj.ResourceVersion = found.ResourceVersion
			d.logger.Info("Updating!")
			return d.client.Update(context.TODO(), obj)
		}
		return nil
	}
}

func (d *VirtualService) UpdateStatus() error {
	return nil
}

func (d *VirtualService) Type() string {
	return "VirtualService"
}

func (d *VirtualService) generate() (*istioclientapiv1.VirtualService, error) {
	httpRoutes := make([]*istioapiv1.HTTPRoute, 0, len(d.plus.Spec.Apps)+1)

	for _, app := range d.plus.Spec.Apps {
		httpRoute := &istioapiv1.HTTPRoute{
			Match:      d.generateMatch(app),
			Rewrite:    d.generateRewrite(),
			Route:      d.generateRoute(app),
			Fault:      d.generateFault(),
			Retries:    d.generateRetries(),
			CorsPolicy: d.generateCorsPolicy(),
			Timeout:    d.plus.Spec.Policy.GetTimeout(),
		}
		httpRoutes = append(httpRoutes, httpRoute)
	}

	if d.plus.Spec.Gateway != nil {
		httpRoute := &istioapiv1.HTTPRoute{
			Match:      d.generateDefaultMatches(),
			Rewrite:    d.generateRewrite(),
			Route:      d.generateDefaultRoute(),
			Fault:      d.generateFault(),
			Retries:    d.generateRetries(),
			CorsPolicy: d.generateCorsPolicy(),
			Timeout:    d.plus.Spec.Policy.GetTimeout(),
		}
		httpRoutes = append(httpRoutes, httpRoute)
	}

	vs := &istioclientapiv1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.plus.GetName(),
			Namespace: d.plus.GetNamespace(),
			Labels:    d.plus.GenerateLabels(),
		},
		Spec: istioapiv1.VirtualService{
			Hosts:    d.generateHost(),
			Gateways: d.generateGateway(),
			Http:     httpRoutes,
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(d.plus, vs, d.scheme); err != nil {
		d.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return vs, nil
}

func (d *VirtualService) exist() (bool, *istioclientapiv1.VirtualService, error) {
	found := &istioclientapiv1.VirtualService{}
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

func (d *VirtualService) generateHost() []string {
	if d.plus.Spec.Gateway == nil {
		return []string{fmt.Sprintf("%s.%s.svc.cluster.local", d.plus.GetName(), d.plus.GetNamespace())}
	}
	return d.plus.Spec.Gateway.Hosts

}

func (d *VirtualService) generateGateway() []string {
	if d.plus.Spec.Gateway == nil {
		return []string{"mesh"}
	}
	return []string{"istio-system/gateway"}
}

func (d *VirtualService) generateMatch(app *v1.PlusApp) []*istioapiv1.HTTPMatchRequest {
	if d.plus.Spec.Gateway == nil {
		return []*istioapiv1.HTTPMatchRequest{
			{
				SourceNamespace: d.plus.GetNamespace(),
				SourceLabels:    d.plus.GenerateAppLabels(app),
			},
		}
	}

	var headers = map[string]*istioapiv1.StringMatch{
		"VERSION": {
			MatchType: &istioapiv1.StringMatch_Exact{
				Exact: app.Name,
			},
		},
	}

	matches := make([]*istioapiv1.HTTPMatchRequest, 0, 10)
	matches = append(matches, []*istioapiv1.HTTPMatchRequest{
		{
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("/%s/%s/", app.Name, d.plus.GetName()),
				},
			},
		},
		{
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("/%s/%s", app.Name, d.plus.GetName()),
				},
			},
		},
		{
			Headers: headers,
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("/%s/", d.plus.GetName()),
				},
			},
		},
		{
			Headers: headers,
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("/%s", d.plus.GetName()),
				},
			},
		},
	}...)

	if d.plus.Spec.Gateway.URLPrefix != "" {
		if d.plus.Spec.Gateway.URLPrefix == "/" {
			matches = append(matches, []*istioapiv1.HTTPMatchRequest{
				{
					Headers: headers,
					Uri: &istioapiv1.StringMatch{
						MatchType: &istioapiv1.StringMatch_Prefix{
							Prefix: "/",
						},
					},
				},
				{
					Uri: &istioapiv1.StringMatch{
						MatchType: &istioapiv1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("/%s/", app.Name),
						},
					},
				},
				{
					Uri: &istioapiv1.StringMatch{
						MatchType: &istioapiv1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("/%s", app.Name),
						},
					},
				},
			}...)
		} else {
			matches = append(matches, []*istioapiv1.HTTPMatchRequest{
				{
					Headers: headers,
					Uri: &istioapiv1.StringMatch{
						MatchType: &istioapiv1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("/%s/", d.plus.Spec.Gateway.URLPrefix),
						},
					},
				},
				{
					Headers: headers,
					Uri: &istioapiv1.StringMatch{
						MatchType: &istioapiv1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("/%s", d.plus.Spec.Gateway.URLPrefix),
						},
					},
				},
				{
					Uri: &istioapiv1.StringMatch{
						MatchType: &istioapiv1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("/%s/%s/", app.Name, d.plus.Spec.Gateway.URLPrefix),
						},
					},
				},
				{
					Uri: &istioapiv1.StringMatch{
						MatchType: &istioapiv1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("/%s/%s", app.Name, d.plus.Spec.Gateway.URLPrefix),
						},
					},
				},
			}...)
		}
	}
	return matches
}

func (d *VirtualService) generateDefaultMatches() []*istioapiv1.HTTPMatchRequest {
	if d.plus.Spec.Gateway == nil {
		return nil
	}
	matches := make([]*istioapiv1.HTTPMatchRequest, 0, len(d.plus.Spec.Apps)*2)
	matches = append(matches, []*istioapiv1.HTTPMatchRequest{
		{
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("/%s/", d.plus.GetName()),
				},
			},
		},
		{
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("/%s", d.plus.GetName()),
				},
			},
		},
	}...)

	if d.plus.Spec.Gateway.URLPrefix != "" {
		matches = append(matches, []*istioapiv1.HTTPMatchRequest{
			{
				Uri: &istioapiv1.StringMatch{
					MatchType: &istioapiv1.StringMatch_Prefix{
						Prefix: fmt.Sprintf("%s/", d.plus.Spec.Gateway.URLPrefix),
					},
				},
			},
			{
				Uri: &istioapiv1.StringMatch{
					MatchType: &istioapiv1.StringMatch_Prefix{
						Prefix: fmt.Sprintf("%s", d.plus.Spec.Gateway.URLPrefix),
					},
				},
			},
		}...)
	}

	return matches
}

func (d *VirtualService) generateRewrite() *istioapiv1.HTTPRewrite {
	if d.plus.Spec.Gateway == nil {
		return nil
	}

	return &istioapiv1.HTTPRewrite{
		Uri: "/",
	}
}

func (d *VirtualService) generateRoute(app *v1.PlusApp) []*istioapiv1.HTTPRouteDestination {
	return []*istioapiv1.HTTPRouteDestination{
		{
			Destination: &istioapiv1.Destination{
				Host: fmt.Sprintf("%s.%s.svc.cluster.local", d.plus.GetName(), d.plus.GetNamespace()),
				Port: &istioapiv1.PortSelector{
					Number: uint32(app.Port),
				},
				Subset: d.plus.GetAppName(app),
			},
			Weight: 100,
		},
	}
}

// generateDefaultRoute 生成默认路由，按照网关的配置流量比例
func (d *VirtualService) generateDefaultRoute() []*istioapiv1.HTTPRouteDestination {
	routeDestinations := make([]*istioapiv1.HTTPRouteDestination, 0, len(d.plus.Spec.Apps))
	for _, app := range d.plus.Spec.Apps {
		routeDestinations = append(routeDestinations, &istioapiv1.HTTPRouteDestination{
			Destination: &istioapiv1.Destination{
				Host: fmt.Sprintf("%s.%s.svc.cluster.local", d.plus.GetName(), d.plus.GetNamespace()),
				Port: &istioapiv1.PortSelector{
					Number: uint32(app.Port),
				},
				Subset: d.plus.GetAppName(app),
			},
			Weight: d.plus.Spec.Gateway.Weights[app.Name],
		},
		)
	}
	return routeDestinations
}

func (d *VirtualService) generateRetries() *istioapiv1.HTTPRetry {
	if d.plus.Spec.Policy.Retries == nil {
		return nil
	}
	return &istioapiv1.HTTPRetry{
		Attempts:      d.plus.Spec.Policy.Retries.Attempts,
		PerTryTimeout: d.plus.Spec.Policy.Retries.GetPerTryTimeout(),
		RetryOn:       d.plus.Spec.Policy.Retries.RetryOn,
	}
}

func (d *VirtualService) generateFault() *istioapiv1.HTTPFaultInjection {
	if d.plus.Spec.Policy == nil || d.plus.Spec.Policy.Fault == nil {
		return nil
	}

	fault := &istioapiv1.HTTPFaultInjection{}

	if d.plus.Spec.Policy.Fault.Delay != nil {
		fault.Delay = &istioapiv1.HTTPFaultInjection_Delay{
			Percentage: d.plus.Spec.Policy.Fault.Delay.GetPercent(),
			HttpDelayType: &istioapiv1.HTTPFaultInjection_Delay_FixedDelay{
				FixedDelay: d.plus.Spec.Policy.Fault.Delay.GetDelay(),
			},
		}
	}

	if d.plus.Spec.Policy.Fault.Abort != nil {
		fault.Abort = &istioapiv1.HTTPFaultInjection_Abort{
			Percentage: d.plus.Spec.Policy.Fault.Abort.GetPercent(),
			ErrorType: &istioapiv1.HTTPFaultInjection_Abort_HttpStatus{
				HttpStatus: d.plus.Spec.Policy.Fault.Abort.HttpStatus,
			},
		}
	}
	return fault
}

func (d *VirtualService) generateCorsPolicy() *istioapiv1.CorsPolicy {
	if d.plus.Spec.Gateway == nil || d.plus.Spec.Gateway.Cors == nil {
		return nil
	}

	allowOrigins := make([]*istioapiv1.StringMatch, 0, len(d.plus.Spec.Gateway.Cors.AllowOrigins))

	for _, origin := range d.plus.Spec.Gateway.Cors.AllowOrigins {
		allowOrigins = append(allowOrigins, &istioapiv1.StringMatch{MatchType: &istioapiv1.StringMatch_Exact{Exact: origin}})
	}

	if len(d.plus.Spec.Gateway.Cors.AllowMethods) == 0 {
		d.plus.Spec.Gateway.Cors.AllowMethods = []string{"POST", "GET", "OPTIONS", "DELETE", "PUT"}
	}

	if len(d.plus.Spec.Gateway.Cors.AllowHeaders) == 0 {
		d.plus.Spec.Gateway.Cors.AllowHeaders = []string{"Origin", "x-token"}
	}

	if len(d.plus.Spec.Gateway.Cors.ExposeHeaders) == 0 {
		d.plus.Spec.Gateway.Cors.ExposeHeaders = nil
	}

	return &istioapiv1.CorsPolicy{
		AllowOrigins:     allowOrigins,
		AllowHeaders:     d.plus.Spec.Gateway.Cors.AllowHeaders,
		AllowMethods:     d.plus.Spec.Gateway.Cors.AllowMethods,
		ExposeHeaders:    d.plus.Spec.Gateway.Cors.ExposeHeaders,
		MaxAge:           &duration.Duration{Seconds: 60 * 60 * 24},
		AllowCredentials: &wrappers.BoolValue{Value: true},
	}
}
