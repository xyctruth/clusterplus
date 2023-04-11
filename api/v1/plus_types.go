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
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PlusSpec defines the desired state of Plus
type PlusSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Gateway 描述需要提供域名对外提供访问的程序
	Gateway *PlusGateway `json:"gateway,omitempty"`
	// Policy 描述网络策略
	Policy *PlusPolicy `json:"policy,omitempty"`
	// Apps 描述具体部署的程序，可以有多个版本
	Apps []*PlusApp `json:"apps,omitempty"`
}

// PlusStatus defines the observed state of Plus
type PlusStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	AvailableReplicas map[string]int32 `json:"availableReplicas,omitempty"`
	Success           bool             `json:"success,omitempty"`
	Desc              PlusDesc         `json:"desc,omitempty"`
}

type PlusDesc struct {
	Replicas   string `json:"replicas,omitempty"`
	Images     string `json:"images,omitempty"`
	Weights    string `json:"weights,omitempty"`
	PrefixPath string `json:"prefixPath,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Hosts",type="string",JSONPath=".spec.gateway.hosts",description="Hosts"
// +kubebuilder:printcolumn:name="PrefixPath",type="string",JSONPath=".status.desc.prefixPath",description="Visit prefix path"
// +kubebuilder:printcolumn:name="Weights",type="string",JSONPath=".status.desc.weights",description="Weights"
// +kubebuilder:printcolumn:name="Images",type="string",JSONPath=".status.desc.images",description="The Docker Image"
// +kubebuilder:printcolumn:name="Replicas",type="string",JSONPath=".status.desc.replicas",description="Replicas"
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

func (r *Plus) GetAppImage(app *PlusApp) string {
	return strings.ReplaceAll(app.Image, "_"+app.Version, "")
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

func (r *Plus) GenerateAppTemplateLabels(app *PlusApp) map[string]string {
	var labels = r.GenerateLabels()
	labels["version"] = app.Version
	for k, v := range app.TemplateLabels {
		labels[k] = v
	}
	return labels
}

func (r *Plus) GenerateStatusDesc() {
	r.Status.Desc = PlusDesc{}
	for _, v := range r.Spec.Apps {
		key := v.Version
		r.Status.Desc.Replicas = r.Status.Desc.Replicas +
			fmt.Sprintf("%s:%d-%d(%d) ", v.Version, v.MinReplicas, v.MaxReplicas, r.Status.AvailableReplicas[key])
		imagesPath := strings.Split(v.Image, ":")
		if imagesPath != nil && len(imagesPath) > 0 {
			r.Status.Desc.Images = r.Status.Desc.Images + fmt.Sprintf("%s:%s ", v.Version, imagesPath[len(imagesPath)-1])
		}
		if r.Spec.Gateway != nil {
			if w, ok := r.Spec.Gateway.Weights[key]; ok {
				r.Status.Desc.Weights = r.Status.Desc.Weights + fmt.Sprintf("%s:%d ", key, w)
			}
		}
	}
	if len(r.Status.Desc.Replicas) > 0 {
		r.Status.Desc.Replicas = strings.TrimSuffix(r.Status.Desc.Replicas, " ")
	}
	if len(r.Status.Desc.Images) > 0 {
		r.Status.Desc.Images = strings.TrimSuffix(r.Status.Desc.Images, " ")
	}
	if len(r.Status.Desc.Weights) > 0 {
		r.Status.Desc.Weights = strings.TrimSuffix(r.Status.Desc.Weights, " ")
	}

	r.Status.Desc.PrefixPath = r.GeneratePrefixPath()
}

func (r *Plus) GeneratePrefixPath() string {
	prefixPath := ""
	if r.Spec.Gateway == nil {
		return ""
	}
	if r.Spec.Gateway.PathPrefix == nil {
		prefixPath = fmt.Sprintf("/%s", r.GetName())
		return prefixPath
	}

	if *r.Spec.Gateway.PathPrefix == "" {
		return ""
	}

	if strings.HasPrefix(*r.Spec.Gateway.PathPrefix, "/") {
		return *r.Spec.Gateway.PathPrefix
	} else {
		return fmt.Sprintf("/%s", *r.Spec.Gateway.PathPrefix)
	}
}

func (r *Plus) Validate() error {
	if errs := validation.IsDNS1123Label(r.Name); len(errs) != 0 {
		err := field.Invalid(field.NewPath(r.Name), r.Name, fmt.Sprintf("%v", errs))
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
