package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ConfigMapTransformer transforms a configMap and returns the transformed copy
type ConfigMapTransformer interface {
	Transform(*corev1.ConfigMap) (*corev1.ConfigMap, error)
}

func main() {

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/readiness", readinessHandler)

	port := 8080
	server := &http.Server{
		Handler: nil,
		Addr:    fmt.Sprintf(":%d", port),
	}

	ctx := context.Background()
	controller, err := build()
	if err != nil {
		log.Fatalln("failed to build controller: " + err.Error())
	}
	go controller.Run(ctx, 2)

	// start server
	go func() {
		log.Println(fmt.Sprintf("Starting server on port %d", port))
		if err := server.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	sig := <-interrupt // block until interrupt
	log.Println(fmt.Sprintf("Received %s: shutting down gracefully...", sig))

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	os.Exit(0)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func build() (*Controller, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config) // FIXME: need to load from config here?
	if err != nil {
		return nil, err
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Core().V1().ConfigMaps()

	transformer := DataPopulator{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		keyToWatch: "x-k8s.io/curl-me-that",
	}

	return NewController(clientset, informer, transformer), nil
}
