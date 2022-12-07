/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"strings"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PlusSpec defines the desired state of Plus
type PlusSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Plus. Edit plus_types.go to remove/update
	Gateway *PlusGateway `json:"gateway,omitempty"`
	Policy  *PlusPolicy  `json:"policy,omitempty"`
	Apps    []*PlusApp   `json:"apps,omitempty"`
	Type    PlusType     `json:"type,omitempty"`
}

type PlusType string

// PlusStatus defines the observed state of Plus
type PlusStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	AvailableReplicas map[string]int32 `json:"availableReplicas,omitempty"`
	Success           bool             `json:"success,omitempty"`
	Desc              PlusDesc         `json:"desc,omitempty"`
}

type PlusDesc struct {
	AvailableReplicas string `json:"availableReplicas,omitempty"`
	Replicas          string `json:"replicas,omitempty"`
	Images            string `json:"images,omitempty"`
	Weights           string `json:"weights,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Hosts",type="string",JSONPath=".spec.gateway.hosts",description="Hosts"
// +kubebuilder:printcolumn:name="Images",type="string",JSONPath=".status.desc.images",description="The Docker Image"
// +kubebuilder:printcolumn:name="Replicas",type="string",JSONPath=".status.desc.replicas",description="Replicas"
// +kubebuilder:printcolumn:name="AvailableReplicas",type="string",JSONPath=".status.desc.availableReplicas",description="AvailableReplicas"
// +kubebuilder:printcolumn:name="Weights",type="string",JSONPath=".status.desc.weights",description="Weights"
// +kubebuilder:printcolumn:name="Success",type="boolean",JSONPath=".status.success",description="Success"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:subresource:desc

// Plus is the Schema for the pluses API
type Plus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlusSpec   `json:"spec,omitempty"`
	Status PlusStatus `json:"status,omitempty"`
}

func (r *Plus) GetName() string {
	return r.Name
}

func (r *Plus) GetNamespace() string {
	return r.Namespace
}

func (r *Plus) GetAppName(app *PlusApp) string {
	return r.Name + "-" + app.Version
}

func (r *Plus) GenerateLabels() map[string]string {
	labels := make(map[string]string)
	for k, v := range r.Labels {
		labels[k] = v
	}
	labels["plus"] = r.Name
	return labels
}

func (r *Plus) GenerateVersionLabels(app *PlusApp) map[string]string {
	return map[string]string{"version": app.Version}
}

func (r *Plus) GenerateAppLabels(app *PlusApp) map[string]string {
	var labels = r.GenerateLabels()
	labels["version"] = app.Version
	return labels
}

func (r *Plus) GenerateStatusDesc() {
	r.Status.Desc = PlusDesc{}
	for _, v := range r.Spec.Apps {
		r.Status.Desc.AvailableReplicas = r.Status.Desc.AvailableReplicas + fmt.Sprintf("%s:%d ", v.Version, r.Status.AvailableReplicas[v.Version])
		r.Status.Desc.Replicas = r.Status.Desc.Replicas + fmt.Sprintf("%s:%d-%d ", v.Version, v.MinReplicas, v.MaxReplicas)
		imagesPath := strings.Split(v.Image, ":")
		if imagesPath != nil && len(imagesPath) > 0 {
			r.Status.Desc.Images = r.Status.Desc.Images + fmt.Sprintf("%s:%s ", v.Version, imagesPath[len(imagesPath)-1])
		}
	}

	if r.Spec.Gateway != nil {
		for k, v := range r.Spec.Gateway.Weights {
			r.Status.Desc.Weights = r.Status.Desc.Weights + fmt.Sprintf("%s:%d ", k, v)
		}
	}

	r.Status.Desc.AvailableReplicas = fmt.Sprintf("[%s]", r.Status.Desc.AvailableReplicas)
	r.Status.Desc.Replicas = fmt.Sprintf("[%s]", r.Status.Desc.Replicas)
	r.Status.Desc.Images = fmt.Sprintf("[%s]", r.Status.Desc.Images)
	r.Status.Desc.Weights = fmt.Sprintf("[%s]", r.Status.Desc.Weights)
}

func (r *Plus) Validate() error {
	if msgs := validation.IsDNS1123Label(r.Name); len(msgs) != 0 {
		err := field.Invalid(field.NewPath(r.Name), r.Name, fmt.Sprintf("%v", msgs))
		return apierrors.NewInvalid(PlusKind, "name", field.ErrorList{err})
	}

	fldPath := field.NewPath("spec")
	if e := r.Spec.Policy; e != nil {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}

	if e := r.Spec.Gateway; e != nil {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}

	for _, e := range r.Spec.Apps {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}
	return nil
}

//+kubebuilder:object:root=true

// PlusList contains a list of Plus
type PlusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Plus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Plus{}, &PlusList{})
}
