package main

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

const controllerAgentName = "configMap-controller"

// Controller is a controller to auto-populate ConfigMaps with fetched data
type Controller struct {
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	informer        v1.ConfigMapInformer
	lister          listers.ConfigMapLister
	transformer     ConfigMapTransformer
	recorder        record.EventRecorder
}

// NewController returns a new configMap controller
func NewController(
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
	transformer ConfigMapTransformer,
) *Controller {

	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	informer := informerFactory.Core().V1().ConfigMaps()

	controller := &Controller{
		client:          client,
		informerFactory: informerFactory,
		informer:        informer,
		transformer:     transformer,
		recorder:        recorder,
	}

	klog.Info("Setting up event handlers")
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.updateConfigMap,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.updateConfigMap(newObj)
		},
	})

	return controller
}

// Run starts the informer and syncs caches
func (c *Controller) Run(ctx context.Context) {
	c.informerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.Informer().HasSynced) {
		klog.Errorf("caches did not sync") // TODO: this should do something more than simply logging
	}
}

func (c *Controller) updateConfigMap(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("couldn't retrieve key from object: %s", err)
		return
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("invalid resource key: %s", err)
		return
	}

	configMap, err := c.informer.Lister().ConfigMaps(namespace).Get(name)
	if err != nil {
		klog.Errorf("couldn't retrieve config map %s/%s: %s", namespace, name, err)
		return
	}

	transformed, err := c.transformer.Transform(configMap)
	if err != nil {
		c.recorder.Event(transformed, corev1.EventTypeWarning, err.Error(), "could not transform")
		return
	}
	if transformed == nil {
		return // no error but no result => did not contain watched annotation
	}

	_, err = c.client.CoreV1().ConfigMaps(namespace).Update(transformed)
	if err != nil {
		c.recorder.Event(transformed, corev1.EventTypeWarning, err.Error(), "could not update")
		return
	}

	time.Sleep(500 * time.Millisecond) // TODO: remove this deplorable hack once proper queueing is implemented
}
