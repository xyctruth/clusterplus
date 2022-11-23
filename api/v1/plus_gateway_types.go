package v1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type PlusGateway struct {
	Hosts     []string         `json:"hosts,omitempty"`
	Cors      *PlusGatewayCors `json:"cors,omitempty"`
	Weights   map[string]int32 `json:"weights,omitempty"`
	URLPrefix string           `json:"urlPrefix,omitempty"`
}

type PlusGatewayCors struct {
	AllowOrigins  []string `json:"allowOrigins,omitempty"`
	AllowMethods  []string `json:"allowMethods,omitempty"`
	AllowHeaders  []string `json:"allowHeaders,omitempty"`
	ExposeHeaders []string `json:"exposeHeaders,omitempty"`
}

type PlusGatewayWeights struct {
	AllowOrigins []string `json:"allowOrigins,omitempty"`
}

func (d *PlusGateway) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("gateway")

	if d.Hosts == nil || len(d.Hosts) == 0 {
		err := field.Invalid(fldPath.Child("hosts"), d.Hosts, "hosts can't be empty")
		return apierrors.NewInvalid(PlusKind, "hosts", field.ErrorList{err})
	}

	if d.Weights == nil || len(d.Weights) == 0 {
		err := field.Invalid(fldPath.Child("weights"), d.Weights, "weights can't be empty")
		return apierrors.NewInvalid(PlusKind, "weights", field.ErrorList{err})
	}

	var sum int32
	for _, w := range d.Weights {
		sum += w
	}

	if sum != 100 {
		err := field.Invalid(fldPath.Child("weights"), d.Weights, fmt.Sprintf("total weights sum must %d != 100", sum))
		return apierrors.NewInvalid(PlusKind, "weights", field.ErrorList{err})
	}

	return nil
}
