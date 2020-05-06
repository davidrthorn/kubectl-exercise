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
)

func main() {
	http.HandleFunc("/health", livenessHandler)
	http.HandleFunc("/readiness", readinessHandler)

	port := 8080
	server := &http.Server{
		Handler: nil,
		Addr:    fmt.Sprintf(":%d", port),
	}

	// Start Server
	go func() {
		log.Println(fmt.Sprintf("Starting server on port %d", port))
		if err := server.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	gracefulShutdown(server)
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func gracefulShutdown(srv *http.Server) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-interrupt // block until interrupt

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("Shutting server down")
	os.Exit(0)
}
