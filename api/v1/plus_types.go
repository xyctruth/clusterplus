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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	PlusTypeGateway = "gateway"
	PlusTypeSvc     = "svc"
	PlusTypeWeb     = "web"
)

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

func (in *Plus) GetName() string {
	return in.Name
}

func (in *Plus) GetNamespace() string {
	return in.Namespace
}

func (in *Plus) GetAppName(app *PlusApp) string {
	return in.Name + "-" + app.Name
}

func (in *Plus) GenerateLabels() map[string]string {
	labels := make(map[string]string)
	for k, v := range in.Labels {
		labels[k] = v
	}
	labels["plus"] = in.Name
	return labels
}

func (in *Plus) GenerateVersionLabels(app *PlusApp) map[string]string {
	return map[string]string{"version": app.Name}
}

func (in *Plus) GenerateAppLabels(app *PlusApp) map[string]string {
	var labels = in.GenerateLabels()
	labels["version"] = app.Name
	return labels
}

func (in *Plus) GenerateStatusDesc() {
	in.Status.Desc = PlusDesc{}
	for k, v := range in.Status.AvailableReplicas {
		in.Status.Desc.AvailableReplicas = in.Status.Desc.AvailableReplicas + fmt.Sprintf("%s:%d ", k, v)
	}
	for _, v := range in.Spec.Apps {
		in.Status.Desc.Replicas = in.Status.Desc.Replicas + fmt.Sprintf("%s:%d-%d ", v.Name, v.MinReplicas, v.MaxReplicas)
		imagesPath := strings.Split(v.Image, ":")
		if imagesPath != nil && len(imagesPath) > 0 {
			in.Status.Desc.Images = in.Status.Desc.Images + fmt.Sprintf("%s:%s ", v.Name, imagesPath[len(imagesPath)-1])
		}
	}

	if in.Spec.Gateway != nil {
		for k, v := range in.Spec.Gateway.Weights {
			in.Status.Desc.Weights = in.Status.Desc.Weights + fmt.Sprintf("%s:%d ", k, v)
		}

	}

	in.Status.Desc.AvailableReplicas = fmt.Sprintf("[%s]", in.Status.Desc.AvailableReplicas)
	in.Status.Desc.Replicas = fmt.Sprintf("[%s]", in.Status.Desc.Replicas)
	in.Status.Desc.Images = fmt.Sprintf("[%s]", in.Status.Desc.Images)
	in.Status.Desc.Weights = fmt.Sprintf("[%s]", in.Status.Desc.Weights)
}

func (in *Plus) Validate() error {
	fldPath := field.NewPath("spec")
	if e := in.Spec.Policy; e != nil {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}

	if e := in.Spec.Gateway; e != nil {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}

	for _, e := range in.Spec.Apps {
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
