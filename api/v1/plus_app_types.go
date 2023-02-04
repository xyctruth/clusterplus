package v1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type PlusApp struct {
	// 一个程序多个版本，（用于蓝绿版本,灰度版本等）
	Annotations      map[string]string           `json:"annotations,omitempty"`
	Version          string                      `json:"version,omitempty" json:"version,omitempty"`
	Image            string                      `json:"image,omitempty" json:"image,omitempty"`
	ImagePullSecrets string                      `json:"imagePullSecrets,omitempty" json:"imagePullSecrets,omitempty"`
	Env              []corev1.EnvVar             `json:"env,omitempty" json:"env,omitempty"`
	MinReplicas      int32                       `json:"minReplicas,omitempty" json:"minReplicas,omitempty"`
	MaxReplicas      int32                       `json:"maxReplicas,omitempty" json:"maxReplicas,omitempty"`
	Resources        corev1.ResourceRequirements `json:"resources,omitempty" json:"resources"`
	Port             int32                       `json:"port,omitempty" json:"port,omitempty"`
	Protocol         string                      `json:"protocol,omitempty" json:"protocol,omitempty"`
	RestartMark      string                      `json:"restartMark,omitempty" json:"restartMark,omitempty"`
	ReadinessProbe   *PlusAppProbe               `json:"readinessProbe,omitempty" json:"readinessProbe,omitempty"`
	LivenessProbe    *PlusAppProbe               `json:"livenessProbe,omitempty" json:"livenessProbe,omitempty"`
	HostAliases      []corev1.HostAlias          `json:"hostAliases,omitempty" json:"hostAliases,omitempty"`
}

type PlusAppProbe struct {
	HttpPath            string `json:"httpPath,omitempty"`
	TimeoutSeconds      int32  `json:"timeoutSeconds,omitempty"`
	InitialDelaySeconds int32  `json:"initialDelaySeconds,omitempty"`
}

func (r *PlusApp) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("app")

	if r.Version == "" {
		err := field.Invalid(fldPath.Child("version"), r.Version, "version can't be empty")
		return apierrors.NewInvalid(PlusKind, "version", field.ErrorList{err})
	}

	if r.MinReplicas == 0 {
		err := field.Invalid(fldPath.Child("minReplicas"), r.MinReplicas, "minReplicas must != 0")
		return apierrors.NewInvalid(PlusKind, "minReplicas", field.ErrorList{err})
	}

	if r.MaxReplicas < r.MinReplicas {
		err := field.Invalid(fldPath.Child("maxReplicas"), r.MaxReplicas, fmt.Sprintf("maxReplicas must >= minReplicas(%d)", r.MinReplicas))
		return apierrors.NewInvalid(PlusKind, "maxReplicas", field.ErrorList{err})
	}

	if r.Port < 0 {
		err := field.Invalid(fldPath.Child("port"), r.Port, fmt.Sprintf("port must > 0"))
		return apierrors.NewInvalid(PlusKind, "port", field.ErrorList{err})
	}

	if r.Protocol != "http" && r.Protocol != "grpc" && r.Protocol != "none" {
		err := field.Invalid(fldPath.Child("protocol"), r.Protocol, fmt.Sprintf("protocol must (http or grpc)"))
		return apierrors.NewInvalid(PlusKind, "protocol", field.ErrorList{err})
	}

	return nil
}
