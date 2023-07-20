package own

import (
	v1 "clusterplus.io/clusterplus/api/v1"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
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
func (r *VirtualService) Apply() error {
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
		if !reflect.DeepEqual(obj.Spec.Hosts, found.Spec.Hosts) ||
			!reflect.DeepEqual(obj.Spec.Gateways, found.Spec.Gateways) ||
			!reflect.DeepEqual(obj.Spec.Http, found.Spec.Http) ||
			!reflect.DeepEqual(obj.Spec.Tcp, found.Spec.Tcp) ||
			!reflect.DeepEqual(obj.Spec.Tls, found.Spec.Tls) ||
			!reflect.DeepEqual(obj.Spec.ExportTo, found.Spec.ExportTo) {
			obj.ResourceVersion = found.ResourceVersion
			r.logger.Info("Updating!")
			return r.client.Update(context.TODO(), obj)
		}
		return nil
	}
}

func (r *VirtualService) UpdateStatus() error {
	return nil
}

func (r *VirtualService) Type() string {
	return "VirtualService"
}

func (r *VirtualService) generate() (*istioclientapiv1.VirtualService, error) {
	httpRoutes := make([]*istioapiv1.HTTPRoute, 0, len(r.plus.Spec.Apps)+1)

	for _, app := range r.plus.Spec.Apps {
		httpRoute := &istioapiv1.HTTPRoute{
			Match:      r.generateMatch(app),
			Rewrite:    r.generateRewrite(),
			Route:      r.generateRoute(app),
			Fault:      r.generateFault(),
			Retries:    r.generateRetries(),
			CorsPolicy: r.generateCorsPolicy(),
		}
		if r.plus.Spec.Policy != nil {
			httpRoute.Timeout = r.plus.Spec.Policy.GetTimeout()
		}
		httpRoutes = append(httpRoutes, httpRoute)
	}

	if r.plus.Spec.Gateway != nil {
		httpRoute := &istioapiv1.HTTPRoute{
			Match:      r.generateDefaultMatches(),
			Rewrite:    r.generateRewrite(),
			Route:      r.generateDefaultRoute(),
			Fault:      r.generateFault(),
			Retries:    r.generateRetries(),
			CorsPolicy: r.generateCorsPolicy(),
		}
		if r.plus.Spec.Policy != nil {
			httpRoute.Timeout = r.plus.Spec.Policy.GetTimeout()
		}
		httpRoutes = append(httpRoutes, httpRoute)
	}

	vs := &istioclientapiv1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.plus.GetName(),
			Namespace: r.plus.GetNamespace(),
			Labels:    r.plus.GenerateLabels(),
		},
		Spec: istioapiv1.VirtualService{
			Hosts:    r.generateHost(),
			Gateways: r.generateGateway(),
			Http:     httpRoutes,
		},
	}

	// 绑定关系，删除instance会删除底下所有资源
	if err := controllerutil.SetControllerReference(r.plus, vs, r.scheme); err != nil {
		r.logger.Error(err, "Set controllerReference failed")
		return nil, err
	}
	return vs, nil
}

func (r *VirtualService) exist() (bool, *istioclientapiv1.VirtualService, error) {
	found := &istioclientapiv1.VirtualService{}
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

func (r *VirtualService) generateHost() []string {
	if r.plus.Spec.Gateway == nil {
		return []string{fmt.Sprintf("%s.%s.svc.cluster.local", r.plus.GetName(), r.plus.GetNamespace())}
	}
	return r.plus.Spec.Gateway.Hosts

}

func (r *VirtualService) generateGateway() []string {
	if r.plus.Spec.Gateway == nil {
		return []string{"mesh"}
	}
	return []string{"istio-system/gateway"}
}

func (r *VirtualService) generatePrefixPath() string {
	return r.plus.GeneratePrefixPath()
}

func (r *VirtualService) generateMatch(app *v1.PlusApp) []*istioapiv1.HTTPMatchRequest {
	matches := make([]*istioapiv1.HTTPMatchRequest, 0, 10)

	if r.plus.Spec.Gateway == nil {
		if app.Version == "blue" || app.Version == "green" {
			matches = append(matches, &istioapiv1.HTTPMatchRequest{
				SourceNamespace: r.plus.GetNamespace(),
				SourceLabels:    r.plus.GenerateVersionLabels(app),
			})
		}
		return matches
	}

	// 匹配自定义路由
	route := r.plus.Spec.Gateway.Route[app.Version]
	if route != nil {
		for _, match := range route.HeadersMatch {
			headers := make(map[string]*istioapiv1.StringMatch)
			for k, v := range match {
				headers[k] = &istioapiv1.StringMatch{
					MatchType: &istioapiv1.StringMatch_Exact{
						Exact: v,
					},
				}
			}
			matches = append(matches, []*istioapiv1.HTTPMatchRequest{
				{
					Headers: headers,
					Uri: &istioapiv1.StringMatch{
						MatchType: &istioapiv1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("%s/", r.generatePrefixPath()),
						},
					},
				},
				{
					Headers: headers,
					Uri: &istioapiv1.StringMatch{
						MatchType: &istioapiv1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("%s", r.generatePrefixPath()),
						},
					},
				},
			}...)

		}
	}

	var defaultHeaders = map[string]*istioapiv1.StringMatch{
		"VERSION": {
			MatchType: &istioapiv1.StringMatch_Exact{
				Exact: app.Version,
			},
		},
	}
	// 匹配默认版本请求头
	matches = append(matches, []*istioapiv1.HTTPMatchRequest{
		{
			Headers: defaultHeaders,
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("%s/", r.generatePrefixPath()),
				},
			},
		},
		{
			Headers: defaultHeaders,
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("%s", r.generatePrefixPath()),
				},
			},
		},
	}...)

	return matches
}

func (r *VirtualService) generateDefaultMatches() []*istioapiv1.HTTPMatchRequest {
	if r.plus.Spec.Gateway == nil {
		return nil
	}
	matches := make([]*istioapiv1.HTTPMatchRequest, 0, len(r.plus.Spec.Apps)*2)
	matches = append(matches, []*istioapiv1.HTTPMatchRequest{
		{
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("%s/", r.generatePrefixPath()),
				},
			},
		},
		{
			Uri: &istioapiv1.StringMatch{
				MatchType: &istioapiv1.StringMatch_Prefix{
					Prefix: fmt.Sprintf("%s", r.generatePrefixPath()),
				},
			},
		},
	}...)
	return matches
}

func (r *VirtualService) generateRewrite() *istioapiv1.HTTPRewrite {
	if r.plus.Spec.Gateway == nil {
		return nil
	}

	return &istioapiv1.HTTPRewrite{
		Uri: "/",
	}
}

func (r *VirtualService) generateRoute(app *v1.PlusApp) []*istioapiv1.HTTPRouteDestination {
	return []*istioapiv1.HTTPRouteDestination{
		{
			Destination: &istioapiv1.Destination{
				Host: fmt.Sprintf("%s.%s.svc.cluster.local", r.plus.GetName(), r.plus.GetNamespace()),
				Port: &istioapiv1.PortSelector{
					Number: uint32(app.Port),
				},
				Subset: r.plus.GetAppName(app),
			},
			Weight: 100,
		},
	}
}

// generateDefaultRoute 生成默认路由，按照网关的配置流量比例
func (r *VirtualService) generateDefaultRoute() []*istioapiv1.HTTPRouteDestination {
	routeDestinations := make([]*istioapiv1.HTTPRouteDestination, 0, len(r.plus.Spec.Apps))
	for _, app := range r.plus.Spec.Apps {
		routeDestinations = append(routeDestinations, &istioapiv1.HTTPRouteDestination{
			Destination: &istioapiv1.Destination{
				Host: fmt.Sprintf("%s.%s.svc.cluster.local", r.plus.GetName(), r.plus.GetNamespace()),
				Port: &istioapiv1.PortSelector{
					Number: uint32(app.Port),
				},
				Subset: r.plus.GetAppName(app),
			},
			Weight: r.plus.Spec.Gateway.Weights[app.Version],
		},
		)
	}
	return routeDestinations
}

// generateRetries 生成重试策略
func (r *VirtualService) generateRetries() *istioapiv1.HTTPRetry {
	if r.plus.Spec.Policy == nil || r.plus.Spec.Policy.Retries == nil {
		return nil
	}
	return &istioapiv1.HTTPRetry{
		Attempts:      r.plus.Spec.Policy.Retries.Attempts,
		PerTryTimeout: r.plus.Spec.Policy.Retries.GetPerTryTimeout(),
		RetryOn:       r.plus.Spec.Policy.Retries.RetryOn,
	}
}

func (r *VirtualService) generateFault() *istioapiv1.HTTPFaultInjection {
	if r.plus.Spec.Policy == nil || r.plus.Spec.Policy.Fault == nil {
		return nil
	}
	fault := &istioapiv1.HTTPFaultInjection{}
	if r.plus.Spec.Policy.Fault.Delay != nil {
		fault.Delay = &istioapiv1.HTTPFaultInjection_Delay{
			Percentage: r.plus.Spec.Policy.Fault.Delay.GetPercent(),
			HttpDelayType: &istioapiv1.HTTPFaultInjection_Delay_FixedDelay{
				FixedDelay: r.plus.Spec.Policy.Fault.Delay.GetDelay(),
			},
		}
	}
	if r.plus.Spec.Policy.Fault.Abort != nil {
		fault.Abort = &istioapiv1.HTTPFaultInjection_Abort{
			Percentage: r.plus.Spec.Policy.Fault.Abort.GetPercent(),
			ErrorType: &istioapiv1.HTTPFaultInjection_Abort_HttpStatus{
				HttpStatus: r.plus.Spec.Policy.Fault.Abort.HttpStatus,
			},
		}
	}
	return fault
}

func (r *VirtualService) generateCorsPolicy() *istioapiv1.CorsPolicy {
	if r.plus.Spec.Gateway == nil || r.plus.Spec.Gateway.Cors == nil {
		return nil
	}

	allowOrigins := make([]*istioapiv1.StringMatch, 0, len(r.plus.Spec.Gateway.Cors.AllowOrigins))

	for _, origin := range r.plus.Spec.Gateway.Cors.AllowOrigins {
		allowOrigins = append(allowOrigins, &istioapiv1.StringMatch{MatchType: &istioapiv1.StringMatch_Exact{Exact: origin}})
	}

	if len(r.plus.Spec.Gateway.Cors.AllowMethods) == 0 {
		r.plus.Spec.Gateway.Cors.AllowMethods = []string{"POST", "GET", "OPTIONS", "DELETE", "PUT"}
	}

	if len(r.plus.Spec.Gateway.Cors.AllowHeaders) == 0 {
		r.plus.Spec.Gateway.Cors.AllowHeaders = []string{"Origin", "x-token"}
	}

	if len(r.plus.Spec.Gateway.Cors.ExposeHeaders) == 0 {
		r.plus.Spec.Gateway.Cors.ExposeHeaders = nil
	}

	return &istioapiv1.CorsPolicy{
		AllowOrigins:     allowOrigins,
		AllowHeaders:     r.plus.Spec.Gateway.Cors.AllowHeaders,
		AllowMethods:     r.plus.Spec.Gateway.Cors.AllowMethods,
		ExposeHeaders:    r.plus.Spec.Gateway.Cors.ExposeHeaders,
		MaxAge:           &duration.Duration{Seconds: 60 * 60 * 24},
		AllowCredentials: &wrappers.BoolValue{Value: true},
	}
}
