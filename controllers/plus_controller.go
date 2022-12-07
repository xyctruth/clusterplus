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

package controllers

import (
	plusappsv1 "clusterplus.io/clusterplus/api/v1"
	own2 "clusterplus.io/clusterplus/api/v1/own"
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	istioclientapiv1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PlusReconciler reconciles a Plus object
type PlusReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.clusterplus.io,resources=pluses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.clusterplus.io,resources=pluses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.clusterplus.io,resources=pluses/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=*,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Plus object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *PlusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("controllers").WithName("Plus").WithValues("plus", req.NamespacedName)
	//if !plusappsv1.CONFIG.FilterRequest(req.Version) {
	//	log.WithValues("config", plusappsv1.CONFIG).WithValues("req", req.Version).Info("Reconcile cancel,this req filter is false ")
	//	return ctrl.Result{}, nil
	//}

	// panic recovery
	defer func() {
		if rec := recover(); rec != nil {
			switch x := rec.(type) {
			case error:
				log.Error(x, "Reconcile error")
			case string:
				log.Error(errors.New(x), "Reconcile error")
			}
		}
	}()

	var found plusappsv1.Plus

	if err := r.Get(ctx, req.NamespacedName, &found); err != nil {
		// 在我们删除一个不存在的对象的时，我们会遇到not-found errors这样的报错
		// 我们将暂时忽略，因为不能通过重新加入队列的方式来修复这些错误
		//（我们需要等待新的通知），而且我们可以根据删除的请求来获取它们
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, client.IgnoreNotFound(err)
		} else {
			log.Error(err, "unable to fetch Plus")
			return ctrl.Result{}, err
		}
	}

	if res, err, isFinalizer := r.Finalizer(ctx, log, &found); isFinalizer {
		return res, err
	}

	instance := found.DeepCopy()
	instance.Status.Success = true

	// 创建或更新操作
	resources, err := r.getOwnResources(instance, log)
	if err != nil {
		log.Error(err, "getOwnResource error")
		return ctrl.Result{}, err
	}

	// 判断各 resource 是否存在，不存在则创建，存在则判断spec是否有变化，有变化则更新
	for _, ownResource := range resources {
		if err = ownResource.Apply(); err != nil {
			r.Recorder.Event(instance, "Normal", "ApplyError", fmt.Sprintf("%s  error : %s", ownResource.Type(), err.Error()))
			log.Error(err, fmt.Sprintf("Apply Error %s", ownResource.Type()))
			instance.Status.Success = false
		}

		if err = ownResource.UpdateStatus(); err != nil {
			r.Recorder.Event(instance, "Normal", "UpdateStatusError", fmt.Sprintf("%s  error : %s", ownResource.Type(), err.Error()))
			log.Error(err, "Update Status Error")
			instance.Status.Success = false
		}
	}

	instance.GenerateStatusDesc()
	if !reflect.DeepEqual(instance.Status, found.Status) {
		if err := r.Status().Update(context.Background(), instance); err != nil {
			if apierrors.IsConflict(err) {
				log.Info("Update Status Conflict Requeue")
				return ctrl.Result{Requeue: true}, nil
			}
			r.Recorder.Event(instance, "Normal", "UpdateStatusError", fmt.Sprintf(" error : %s", err.Error()))
			return ctrl.Result{}, err
		}

		fmt.Println(instance.Status)
		fmt.Println(found.Status)
		log.Info("Successfully Update Status")
	}

	log.Info("Successfully Reconciled")
	return ctrl.Result{}, nil

}

func (r *PlusReconciler) Finalizer(ctx context.Context, log logr.Logger, instance *plusappsv1.Plus) (res ctrl.Result, err error, isContinue bool) {
	// 2. 删除操作
	// 如果资源对象被直接删除，就无法再读取任何被删除对象的信息，这就会导致后续的清理工作因为信息不足无法进行，Finalizer字段设计来处理这种情况：
	// 2.1 当资源对象 Finalizer字段不为空时，delete操作就会变成update操作，即为对象加上deletionTimestamp时间戳
	// 2.2 当 当前时间在deletionTimestamp时间之后，且Finalizer已清空(视为清理后续的任务已处理完成)的情况下，就会gc此对象了
	myFinalizerName := "storage.finalizers.tutorial.kubebuilder.io"
	//orphanFinalizerName := "orphan"

	// 2.1 检查 DeletionTimestamp 以确定对象是否在删除中
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// 如果当前对象没有 finalizer， 说明其没有处于正被删除的状态。
		// 接着让我们添加 finalizer 并更新对象，相当于注册我们的 finalizer。
		if !containsString(instance.ObjectMeta.Finalizers, myFinalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, myFinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				log.Error(err, "Add Finalizers error", instance.Namespace, instance.Name)
				return ctrl.Result{}, err, true
			}
		}
	} else {
		// 2.2  DeletionTimestamp不为空，说明对象已经开始进入删除状态了，执行自己的删除步骤后续的逻辑，并清除掉自己的finalizer字段，等待自动gc
		if containsString(instance.ObjectMeta.Finalizers, myFinalizerName) {

			// 在删除owner resource之前，先执行自定义的预删除步骤，例如删除owner resource
			if err := r.PreDelete(instance); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err, true
			}

			// 移出掉自定义的Finalizers，这样当Finalizers为空时，gc就会正式开始了
			instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, myFinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err, true
			}
		}

		// 当它们被删除的时候停止 reconciliation
		return ctrl.Result{}, nil, true
	}
	return ctrl.Result{}, nil, false
}

// 根据Unit.Spec生成其所有的own resource
func (r *PlusReconciler) getOwnResources(instance *plusappsv1.Plus, log logr.Logger) ([]IResource, error) {
	var resources []IResource

	resources = append(resources, own2.NewDeployment(instance, r.Scheme, r.Client, log))
	resources = append(resources, own2.NewService(instance, r.Scheme, r.Client, log))
	resources = append(resources, own2.NewDestinationRule(instance, r.Scheme, r.Client, log))
	resources = append(resources, own2.NewVirtualService(instance, r.Scheme, r.Client, log))
	resources = append(resources, own2.NewAutoScaling(instance, r.Scheme, r.Client, log))

	return resources, nil
}

func (r *PlusReconciler) PreDelete(instance *plusappsv1.Plus) error {
	// 特别说明，own resource加上了ControllerReference之后，owner resource gc删除前，会先自动删除它的所有
	// own resources，因此绑定ControllerReference后无需再特别处理删除own resource。

	// 这里留空出来，是为了如果有自定义的pre delete逻辑的需要，可在这里实现。
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&plusappsv1.Plus{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&autoscalingv1.HorizontalPodAutoscaler{}).
		Owns(&istioclientapiv1.VirtualService{}).
		Owns(&istioclientapiv1.DestinationRule{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		//WithEventFilter(&EventFilter{}).
		Complete(r)
}
