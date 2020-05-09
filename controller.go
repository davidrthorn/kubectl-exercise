package main

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

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
	kubeclientset   kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	lister          listers.ConfigMapLister
	transformer     ConfigMapTransformer
	recorder        record.EventRecorder
}

// NewController returns a new configMap controller
func NewController(
	kubeclientset kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
	transformer ConfigMapTransformer,
) *Controller {

	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:   kubeclientset,
		informerFactory: informerFactory,
		transformer:     transformer,
		recorder:        recorder,
	}

	informer := informerFactory.Core().V1().ConfigMaps()
	controller.lister = informer.Lister()

	klog.Info("Setting up event handlers")

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.addHandler,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.addHandler(newObj)
		},
	})

	return controller
}

// Run dispatches workers and listens for shutdown signal
func (c *Controller) Run(ctx context.Context) {
	c.informerFactory.Start(ctx.Done())
}

func (c *Controller) addHandler(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Println("key invalid") // TODO: good error handling
		return
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Printf("invalid resource key: %s", key)
		return
	}

	configMap, err := c.lister.ConfigMaps(namespace).Get(name)
	if err != nil {
		log.Print(err)
		return
	}

	populatedConfigMap, err := c.transformer.Transform(configMap)
	if err != nil {
		c.recorder.Event(configMap, corev1.EventTypeWarning, err.Error(), "testmessage")
		return
	}
	if populatedConfigMap == nil {
		return
	}

	_, err = c.kubeclientset.CoreV1().ConfigMaps(namespace).Update(populatedConfigMap)
	if err != nil {
		log.Println(err)
	}
}
