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
			log.Fatal(err)
		}
	}()

	gracefulShutdown(server)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func gracefulShutdown(srv *http.Server) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	sig := <-interrupt // block until interrupt
	log.Println(fmt.Sprintf("Received %s: shutting down gracefully...", sig))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

	os.Exit(0)
}
