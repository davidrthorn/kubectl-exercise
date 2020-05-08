package main

import (
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const controllerAgentName = "configMap-controller"
const watchAnnotation = "x-k8s.io/curl-me-that"
const timeout = 10 * time.Second

const (
	// SuccessSynced is used as part of the Event 'reason'
	SuccessSynced = "Synced"
	// ConfigMapUpdatedSuccessfully is the message used for an Event fired when a
	// ConfigMap is successfully updated
	ConfigMapUpdatedSuccessfully = "ConfigMap updated successfully"
)

// Controller is a controller to auto-populate ConfigMaps with fetched data
type Controller struct {
	kubeclientset   kubernetes.Interface
	configMapLister listers.ConfigMapLister
	workqueue       workqueue.RateLimitingInterface
	recorder        record.EventRecorder
	httpClient      *http.Client
}

// NewController returns a new configMap controller
func NewController(
	kubeclientset kubernetes.Interface,
	configMapInformer informers.ConfigMapInformer,
	httpClient *http.Client,
) *Controller {

	// Create event broadcaster to log events pertaining to this controller
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:   kubeclientset,
		configMapLister: configMapInformer.Lister(),
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configMaps"),
		recorder:        recorder,
		httpClient:      httpClient,
	}

	klog.Info("Setting up event handlers")

	// TODO: Set up an event handler for when configMaps change
	// configMapInformer.Informer().AddEventHandler()

	return controller
}
