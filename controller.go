package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
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
}

// NewController returns a new configMap controller
func NewController(
	kubeclientset kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
	transformer ConfigMapTransformer,
) *Controller {

	controller := &Controller{
		kubeclientset:   kubeclientset,
		informerFactory: informerFactory,
		transformer:     transformer,
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
		log.Print(err)
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
