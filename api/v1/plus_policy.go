package v1

import (
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/protobuf/types/known/durationpb"
	"time"

	protobuftypes "github.com/gogo/protobuf/types"
	istioapiv1 "istio.io/api/networking/v1alpha3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type PlusPolicy struct {
	MaxRequest       int32                       `json:"maxRequests,omitempty"`
	Timeout          string                      `json:"timeout,omitempty"` //总的超时时间
	Retries          *PlusPolicyRetries          `json:"retries,omitempty"`
	Fault            *PlusPolicyFault            `json:"fault,omitempty"`
	OutlierDetection *PlusPolicyOutlierDetection `json:"outlierDetection,omitempty"`
}

type PlusPolicyRetries struct {
	Attempts      int32  `json:"attempts,omitempty"`      //重试次数
	PerTryTimeout string `json:"perTryTimeout,omitempty"` //每次重试的超时时间
	RetryOn       string `json:"retryOn,omitempty"`       //重试发生于什么错误
}

type PlusPolicyOutlierDetection struct {
	ConsecutiveErrors  uint32 `json:"consecutiveErrors,omitempty"` //熔断错误数量
	Interval           string `json:"interval,omitempty"`          //检查间隔
	EjectionTime       string `json:"ejectionTime,omitempty"`      //驱逐时间
	MaxEjectionPercent int32  `json:"ejectionPercent,omitempty"`   //服务的可驱逐故障实例的最大比例
	MinHealthPercent   int32  `json:"minHealthPercent,omitempty"`  //最小健康比例，健康的实例数量低于这个比例,异常检查功能
}

type PlusPolicyFault struct {
	Delay *PlusPolicyFaultDelay `json:"delay,omitempty"`
	Abort *PlusPolicyFaultAbort `json:"abort,omitempty"`
}

type PlusPolicyFaultDelay struct {
	Percent *int32 `json:"percent,omitempty"`
	Delay   string `json:"delay,omitempty"`
}

type PlusPolicyFaultAbort struct {
	Percent    *int32 `json:"percent,omitempty"`
	HttpStatus int32  `json:"httpStatus,omitempty"`
}

func (d *PlusPolicy) GetTimeout() *duration.Duration {
	time, err := time.ParseDuration(d.Timeout)
	if err != nil {
		return nil
	}
	dd := protobuftypes.DurationProto(time)
	return &duration.Duration{
		Seconds: dd.Seconds,
		Nanos:   dd.Nanos,
	}
}

func (d *PlusPolicyRetries) GetPerTryTimeout() *duration.Duration {
	time, err := time.ParseDuration(d.PerTryTimeout)
	if err != nil {
		return nil
	}
	dd := protobuftypes.DurationProto(time)
	return &duration.Duration{
		Seconds: dd.Seconds,
		Nanos:   dd.Nanos,
	}
}

func (d *PlusPolicyOutlierDetection) GetConsecutiveErrors() *wrappers.UInt32Value {
	return &wrappers.UInt32Value{Value: d.ConsecutiveErrors}
}

func (d *PlusPolicyOutlierDetection) GetInterval() *duration.Duration {
	time, err := time.ParseDuration(d.Interval)
	if err != nil {
		return nil
	}
	dd := protobuftypes.DurationProto(time)
	return &durationpb.Duration{
		Seconds: dd.Seconds,
		Nanos:   dd.Nanos,
	}
}

func (d *PlusPolicyOutlierDetection) GetEjectionTime() *durationpb.Duration {
	time, err := time.ParseDuration(d.EjectionTime)
	if err != nil {
		return nil
	}
	dd := protobuftypes.DurationProto(time)
	return &duration.Duration{
		Seconds: dd.Seconds,
		Nanos:   dd.Nanos,
	}
}

func (d *PlusPolicyFaultDelay) GetDelay() *durationpb.Duration {
	time, err := time.ParseDuration(d.Delay)
	if err != nil {
		return nil
	}
	dd := protobuftypes.DurationProto(time)
	return &duration.Duration{
		Seconds: dd.Seconds,
		Nanos:   dd.Nanos,
	}
}

func (d *PlusPolicyFaultDelay) GetPercent() *istioapiv1.Percent {
	var value float64
	if d.Percent != nil {
		value = float64(*d.Percent)
	}
	return &istioapiv1.Percent{
		Value: value,
	}
}

func (d *PlusPolicyFaultAbort) GetPercent() *istioapiv1.Percent {
	var value float64
	if d.Percent != nil {
		value = float64(*d.Percent)
	}
	return &istioapiv1.Percent{
		Value: value,
	}
}

func (d *PlusPolicy) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("policy")

	if _, err := time.ParseDuration(d.Timeout); err != nil {
		err := field.Invalid(fldPath.Child("timeout"), d.Timeout, err.Error())
		return apierrors.NewInvalid(PlusKind, "timeout", field.ErrorList{err})
	}

	if d.MaxRequest <= 0 {
		err := field.Invalid(fldPath.Child("maxRequest"), d.MaxRequest, "maxRequest must > 0")
		return apierrors.NewInvalid(PlusKind, "maxRequest", field.ErrorList{err})
	}

	if e := d.Fault; e != nil {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}

	if e := d.Retries; e != nil {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}

	if e := d.OutlierDetection; e != nil {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}

	return nil
}

func (d *PlusPolicyRetries) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("retries")

	if _, err := time.ParseDuration(d.PerTryTimeout); err != nil {
		err := field.Invalid(fldPath.Child("perTryTimeout"), d.PerTryTimeout, err.Error())
		return apierrors.NewInvalid(PlusKind, "perTryTimeout", field.ErrorList{err})
	}

	if d.Attempts <= 0 {
		err := field.Invalid(fldPath.Child("attempts"), d.Attempts, "attempts must > 0")
		return apierrors.NewInvalid(PlusKind, "attempts", field.ErrorList{err})
	}

	return nil
}

func (d *PlusPolicyFault) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("fault")
	if e := d.Delay; e != nil {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}

	if e := d.Abort; e != nil {
		if err := e.Validate(fldPath); err != nil {
			return err
		}
	}

	return nil
}

func (d *PlusPolicyFaultDelay) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("delay")
	if _, err := time.ParseDuration(d.Delay); err != nil {
		err := field.Invalid(fldPath.Child("delay"), d.Delay, err.Error())
		return apierrors.NewInvalid(PlusKind, "delay", field.ErrorList{err})
	}
	return nil
}

func (d *PlusPolicyFaultAbort) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("abort")
	return nil
}

func (d *PlusPolicyOutlierDetection) Validate(fldPath *field.Path) error {
	fldPath = fldPath.Child("outlierDetection")

	if _, err := time.ParseDuration(d.Interval); err != nil {
		err := field.Invalid(fldPath.Child("interval"), d.Interval, err.Error())
		return apierrors.NewInvalid(PlusKind, "interval", field.ErrorList{err})
	}

	if _, err := time.ParseDuration(d.EjectionTime); err != nil {
		err := field.Invalid(fldPath.Child("ejectionTime"), d.EjectionTime, err.Error())
		return apierrors.NewInvalid(PlusKind, "ejectionTime", field.ErrorList{err})
	}

	if d.ConsecutiveErrors <= 0 {
		err := field.Invalid(fldPath.Child("consecutiveErrors"), d.ConsecutiveErrors, "consecutiveErrors must > 0")
		return apierrors.NewInvalid(PlusKind, "consecutiveErrors", field.ErrorList{err})
	}

	return nil
}
