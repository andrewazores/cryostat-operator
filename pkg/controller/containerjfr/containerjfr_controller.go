package containerjfr

import (
	"context"
	"fmt"

	openshiftv1 "github.com/openshift/api/route/v1"
	routeClient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	rhjmcv1alpha1 "github.com/rh-jmc-team/container-jfr-operator/pkg/apis/rhjmc/v1alpha1"
	resources "github.com/rh-jmc-team/container-jfr-operator/pkg/controller/containerjfr/resource_definitions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_containerjfr")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ContainerJFR Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	rc := routeClient.NewForConfigOrDie(mgr.GetConfig())
	return &ReconcileContainerJFR{client: mgr.GetClient(), routeClient: *rc, scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("containerjfr-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ContainerJFR
	err = c.Watch(&source.Kind{Type: &rhjmcv1alpha1.ContainerJFR{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ContainerJFR
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &rhjmcv1alpha1.ContainerJFR{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileContainerJFR implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileContainerJFR{}

// ReconcileContainerJFR reconciles a ContainerJFR object
type ReconcileContainerJFR struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client      client.Client
	routeClient routeClient.RouteV1Client
	scheme      *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ContainerJFR object and makes changes based on the state read
// and what is in the ContainerJFR.Spec
func (r *ReconcileContainerJFR) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ContainerJFR")

	// Fetch the ContainerJFR instance
	instance := &rhjmcv1alpha1.ContainerJFR{}
	err := r.client.Get(context.Background(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("ContainerJFR instance not found")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Error reading ContainerJFR instance")
		return reconcile.Result{}, err
	}

	pvc := resources.NewPersistentVolumeClaimForCR(instance)
	if err := controllerutil.SetControllerReference(instance, pvc, r.scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err = r.createObjectIfNotExists(context.Background(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, &corev1.PersistentVolumeClaim{}, pvc); err != nil {
		return reconcile.Result{}, err
	}

	serviceSpecs := &resources.ServiceSpecs{}
	url, err := r.createService(context.Background(), instance, resources.NewGrafanaService(instance))
	if err != nil {
		return reconcile.Result{}, err
	}
	serviceSpecs.GrafanaAddress = fmt.Sprintf("http://%s", url)

	url, err = r.createService(context.Background(), instance, resources.NewJfrDatasourceService(instance))
	if err != nil {
		return reconcile.Result{}, err
	}
	serviceSpecs.DatasourceAddress = fmt.Sprintf("http://%s", url)

	url, err = r.createService(context.Background(), instance, resources.NewExporterService(instance))
	if err != nil {
		return reconcile.Result{}, err
	}
	serviceSpecs.CoreAddress = url

	url, err = r.createService(context.Background(), instance, resources.NewCommandChannelService(instance))
	if err != nil {
		return reconcile.Result{}, err
	}
	serviceSpecs.CommandAddress = url

	pod := resources.NewPodForCR(instance, serviceSpecs)
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err = r.createObjectIfNotExists(context.Background(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &corev1.Pod{}, pod); err != nil {
		reqLogger.Error(err, "Could not create pod")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileContainerJFR) createService(ctx context.Context, controller *rhjmcv1alpha1.ContainerJFR, svc *corev1.Service) (string, error) {
	if err := controllerutil.SetControllerReference(controller, svc, r.scheme); err != nil {
		return "", err
	}
	if err := r.createObjectIfNotExists(context.Background(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &corev1.Service{}, svc); err != nil {
		return "", err
	}

	return r.createRouteForService(controller, svc)
}

func (r *ReconcileContainerJFR) createRouteForService(controller *rhjmcv1alpha1.ContainerJFR, svc *corev1.Service) (string, error) {
	logger := log.WithValues("Request.Namespace", svc.Namespace, "Name", svc.Name, "Kind", fmt.Sprintf("%T", &openshiftv1.Route{}))
	route := &openshiftv1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
		},
		Spec: openshiftv1.RouteSpec{
			To: openshiftv1.RouteTargetReference{
				Kind: "Service",
				Name: svc.Name,
			},
		},
	}
	if err := controllerutil.SetControllerReference(controller, route, r.scheme); err != nil {
		return "", err
	}

	rc := r.routeClient.Routes(svc.Namespace)
	found, err := rc.Get(svc.Name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Not found")
		if created, err := rc.Create(route); err != nil {
			logger.Error(err, "Could not be created")
			return "", err
		} else {
			logger.Info("Created")
			found = created
		}
	} else if err != nil {
		logger.Error(err, "Could not be read")
		return "", err
	}

	logger.Info("Route created", "Service.Status", fmt.Sprintf("%#v", found.Status))
	if len(found.Status.Ingress) < 1 {
		return "", errors.NewTooManyRequestsError("Ingress configuration not yet available")
	}

	return found.Status.Ingress[0].Host, nil
}

func (r *ReconcileContainerJFR) createObjectIfNotExists(ctx context.Context, ns types.NamespacedName, found runtime.Object, toCreate runtime.Object) error {
	logger := log.WithValues("Request.Namespace", ns.Namespace, "Name", ns.Name, "Kind", fmt.Sprintf("%T", toCreate))
	err := r.client.Get(ctx, ns, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Not found")
		if err := r.client.Create(ctx, toCreate); err != nil {
			logger.Error(err, "Could not be created")
			return err
		} else {
			logger.Info("Created")
			found = toCreate
		}
	} else if err != nil {
		logger.Error(err, "Could not be read")
		return err
	}
	logger.Info("Already exists")
	return nil
}