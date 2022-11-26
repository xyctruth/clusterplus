package v1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type PlusGateway struct {
	Hosts   []string                     `json:"hosts,omitempty"`
	Cors    *PlusGatewayCors             `json:"cors,omitempty"`
	Weights map[string]int32             `json:"weights,omitempty"`
	Route   map[string]*PlusGatewayRoute `json:"route,omitempty"`
}

type PlusGatewayRoute struct {
	HeadersMatch []map[string]string `json:"headersMatch,omitempty"`
}

type PlusGatewayCors struct {
	AllowOrigins  []string `json:"allowOrigins,omitempty"`
	AllowMethods  []string `json:"allowMethods,omitempty"`
	AllowHeaders  []string `json:"allowHeaders,omitempty"`
	ExposeHeaders []string `json:"exposeHeaders,omitempty"`
}

func (r *PlusGateway) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("gateway")

	if r.Hosts == nil || len(r.Hosts) == 0 {
		err := field.Invalid(fldPath.Child("hosts"), r.Hosts, "hosts can't be empty")
		return apierrors.NewInvalid(PlusKind, "hosts", field.ErrorList{err})
	}

	if r.Weights == nil || len(r.Weights) == 0 {
		err := field.Invalid(fldPath.Child("weights"), r.Weights, "weights can't be empty")
		return apierrors.NewInvalid(PlusKind, "weights", field.ErrorList{err})
	}

	var sum int32
	for _, w := range r.Weights {
		sum += w
	}

	if sum != 100 {
		err := field.Invalid(fldPath.Child("weights"), r.Weights, fmt.Sprintf("total weights sum must %d != 100", sum))
		return apierrors.NewInvalid(PlusKind, "weights", field.ErrorList{err})
	}

	return nil
}
