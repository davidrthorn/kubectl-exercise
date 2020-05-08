package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
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

// TODO: this has a lot of responsibility
func (c *Controller) updateHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	configMap, err := c.configMapLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("configMap '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	annotations := configMap.GetAnnotations()
	watchValue, ok := annotations[watchAnnotation]
	if !ok {
		return nil // no watched annotation found
	}

	dataKey, URL, err := c.GetDataKeyValuePair(watchValue)
	if err != nil {
		// we'll never be able to decode, so we don't want to requeue
		utilruntime.HandleError(fmt.Errorf("could not decode annotation: %s", err))
		return nil
	}

	fetchedValue, err := c.fetchSimpleBody(URL)
	if err != nil {
		return fmt.Errorf("could not fetch data for annotation URL: %s", err)
	}

	configMapCopy := configMap.DeepCopy()
	configMapCopy.Data[dataKey] = fetchedValue

	c.kubeclientset.CoreV1().ConfigMaps(namespace).Update(configMapCopy)

	c.recorder.Event(configMap, corev1.EventTypeNormal, SuccessSynced, ConfigMapUpdatedSuccessfully)
	return nil
}

// TODO: this could be extracted to a separate annotation handler type
// GetDataKeyValuePair returns the data key and data value for a watched annotation
func (c *Controller) GetDataKeyValuePair(watchValue string) (string, string, error) {
	spl := strings.Split(watchValue, "=")
	if len(spl) != 2 {
		return "", "", fmt.Errorf("watch values should be strings of the form 'key=value'. Value is '%s'", watchValue)
	}
	return spl[0], spl[1], nil
}

// TODO: this could be extracted to a separate annotation handler type
func (c *Controller) fetchSimpleBody(URL string) (string, error) {
	res, err := http.Get(URL)
	if err != nil {
		return "", fmt.Errorf("request failed: %s", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("url responded with status: %s", res.StatusCode)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("could not read body: %s", err)
	}

	return string(body), nil
}
