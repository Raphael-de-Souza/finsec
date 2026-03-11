package main

import (
	"context" 	//graceful shutdown with timeout
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal" //Listen OS 
	"syscall"	//Signals definitions SIGTERM (syscall.SIGTERM). SIGINT (Ctrl+C) (syscall.SIGINT or os.Interrupt)
	"time"

	"github.com/your-org/scores-api/internal/handler"
	"github.com/your-org/scores-api/internal/middleware"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger) //Anyplace can use this logger as default

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := http.NewServeMux() //Native Go router

	securitiesHandler := handler.NewSecuritiesHandler()
	router.HandleFunc("GET /securities", securitiesHandler.ListSecurities)
	router.HandleFunc("GET /securities/{security_id}/scores", securitiesHandler.GetScores)
	router.HandleFunc("GET /health", handler.HealthCheck)

	chain := middleware.Chain( // one wraps the another. Order matters.
		middleware.RequestID,
		middleware.Logger,
		middleware.Recover,
	)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      chain(router),
		ReadTimeout:  10 * time.Second, // Slowloris attack. Client request up to 10s
		WriteTimeout: 10 * time.Second, // Protect agains slow clients. From the request to client receiving response up to 10s
		IdleTimeout:  60 * time.Second, // Close keep-alive inactives connections. Inactive connections up to 60s
	}

	go func() { // goroutine for the webserver.    
		slog.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed { // http.ErrServerClosed is not an error. This happens when we call srv.Shutdown()
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1) //Creates a channel type os.Signal for a maximum 1 message
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM) //signal.Notify registers your program with the Operating System. 
															// When either of these two signals occurs, the OS notifies Go, and Go places this information within the quit channel.
	<-quit //It blocks the main goroutine here and wait for SIGINT (ctrl+c) or SIGTERM (ECS request/docker/kubernetes)

	slog.Info("server shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) //Waits 30s before shutdown
	defer cancel() //Ensute the context is always cleanup even in error case

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
