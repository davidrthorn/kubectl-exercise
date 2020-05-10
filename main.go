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
	"k8s.io/klog"
)

// ConfigMapTransformer returned a transformed deep copy of a config map
type ConfigMapTransformer interface {
	Transform(*corev1.ConfigMap) (*corev1.ConfigMap, error)
}

func main() {
	ctx := context.Background()

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/readiness", readinessHandler)

	port := 8080
	server := &http.Server{
		Handler: nil,
		Addr:    fmt.Sprintf(":%d", port),
	}

	// start server
	go func() {
		log.Println(fmt.Sprintf("Starting server on port %d", port))
		if err := server.ListenAndServe(); err != nil {
			klog.Fatal("could not start server: " + err.Error())
		}
	}()

	transformer := HTTPDataPopulator{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		KeyToWatch: "x-k8s.io/curl-me-that",
	}
	controller := buildController(os.Getenv("KUBECONFIG"), transformer)

	go controller.Run(ctx)

	// handle interrupt
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	sig := <-interrupt // block until interrupt
	klog.Info(fmt.Sprintf("Received %s: shutting down gracefully...", sig))

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

func buildController(kubeConfPath string, transformer ConfigMapTransformer) *Controller {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfPath)
	if err != nil {
		klog.Fatal("Could not load config: " + err.Error())
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal("Could not create client: " + err.Error())
	}

	factory := informers.NewSharedInformerFactory(client, 0)
	return NewController(client, factory, transformer)
}
