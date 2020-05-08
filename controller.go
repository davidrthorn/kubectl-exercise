package main

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

// ConfigMapTransformer transforms a configMap and returns the transformed copy
type ConfigMapTransformer interface {
	Transform(*corev1.ConfigMap) (*corev1.ConfigMap, error)
}

const controllerAgentName = "configMap-controller"
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
	kubeclientset kubernetes.Interface
	lister        listers.ConfigMapLister
	workqueue     workqueue.RateLimitingInterface
	recorder      record.EventRecorder
	transformer   ConfigMapTransformer
}

// NewController returns a new configMap controller
func NewController(
	kubeclientset kubernetes.Interface,
	informer informers.ConfigMapInformer,
	transformer ConfigMapTransformer,
) *Controller {

	// Create event broadcaster to log events pertaining to this controller
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset: kubeclientset,
		lister:        informer.Lister(),
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configMaps"),
		recorder:      recorder,
		transformer:   transformer,
	}

	klog.Info("Setting up event handlers")

	// TODO: Set up an event handler for when configMaps change
	// configMapInformer.Informer().AddEventHandler()

	return controller
}

// Run dispatches workers and listens for shutdown signal
func (c *Controller) Run(threads int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("Starting configMap controller")
	klog.Info("Starting workers")
	for i := 0; i < threads; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)

		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.updateHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
	}
	return true
}

func (c *Controller) updateHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	configMap, err := c.lister.ConfigMaps(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("configMap '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	populatedConfigMap, err := c.transformer.Transform(configMap)
	if err != nil {
		return fmt.Errorf("could not populate config map: %s", err)
	}
	if populatedConfigMap == nil {
		return nil // configMap was not watched
	}

	c.kubeclientset.CoreV1().ConfigMaps(namespace).Update(populatedConfigMap)

	c.recorder.Event(configMap, corev1.EventTypeNormal, SuccessSynced, ConfigMapUpdatedSuccessfully)
	return nil
}
