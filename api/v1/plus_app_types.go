package v1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type PlusApp struct {
	Name        string                      `json:"name,omitempty"`
	Image       string                      `json:"image,omitempty"`
	Env         []corev1.EnvVar             `json:"env,omitempty"`
	MinReplicas int32                       `json:"minReplicas,omitempty"`
	MaxReplicas int32                       `json:"maxReplicas,omitempty"`
	Resources   corev1.ResourceRequirements `json:"resources,omitempty"`
	Port        int32                       `json:"port,omitempty"`
	Protocol    string                      `json:"protocol,omitempty"`
	RestartMark string                      `json:"restartMark,omitempty"`
}

func (d *PlusApp) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("app")

	if d.Name == "" {
		err := field.Invalid(fldPath.Child("name"), d.Name, "name can't be empty")
		return apierrors.NewInvalid(PlusKind, "name", field.ErrorList{err})
	}

	if d.MinReplicas == 0 {
		err := field.Invalid(fldPath.Child("minReplicas"), d.MinReplicas, "minReplicas must != 0")
		return apierrors.NewInvalid(PlusKind, "minReplicas", field.ErrorList{err})
	}

	if d.MaxReplicas < d.MinReplicas {
		err := field.Invalid(fldPath.Child("maxReplicas"), d.MaxReplicas, fmt.Sprintf("maxReplicas must >= minReplicas(%d)", d.MinReplicas))
		return apierrors.NewInvalid(PlusKind, "maxReplicas", field.ErrorList{err})
	}

	if d.Port < 0 {
		err := field.Invalid(fldPath.Child("port"), d.Port, fmt.Sprintf("port must > 0"))
		return apierrors.NewInvalid(PlusKind, "port", field.ErrorList{err})
	}

	if d.Protocol != "http" && d.Protocol != "grpc" && d.Protocol != "none" {
		err := field.Invalid(fldPath.Child("protocol"), d.Protocol, fmt.Sprintf("protocol must (http or grpc)"))
		return apierrors.NewInvalid(PlusKind, "protocol", field.ErrorList{err})
	}

	return nil
}
