package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/bh90210/super/auto/api"
	"github.com/bh90210/super/auto/backend"
	"github.com/bh90210/super/auto/frontend"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	githubPath  = "/webhooks"
	metricsPath = "/metrics"
)

// func recordMetrics() {
// 	go func() {
// 		for {
// 			opsProcessed.Inc()
// 			time.Sleep(2 * time.Second)
// 		}
// 	}()
// }

// var (
// 	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
// 		Name: "myapp_processed_ops_total",
// 		Help: "The total number of processed events",
// 	})
// )

func main() {
	isServer := flag.Bool("server", true, "Run as server. Default is true.")
	flag.Parse()

	// Intitiate prometheus metrics server.
	go func() {
		// opsQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		// 	Namespace: "our_company",
		// 	Subsystem: "blob_storage",
		// 	Name:      "ops_queued",
		// 	Help:      "Number of blob storage operations waiting to be processed.",
		// })
		// prometheus.MustRegister(opsQueued)

		// // 10 operations queued by the goroutine managing incoming requests.
		// opsQueued.Add(10)
		// // A worker goroutine has picked up a waiting operation.
		// opsQueued.Dec()
		// // And once more...
		// opsQueued.Dec()
		// recordMetrics()

		slog.Info("starting metrics server on :2112")

		http.Handle(metricsPath, promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()

	switch *isServer {
	// If server flags is set to false, run as client. This means connecting to gRPC server and listening for webhook events.
	case false:
		for {
			// Start a new gRPC client to connect to webhook frontend container running server.
			conn, err := grpc.NewClient("localhost:3005",
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				slog.Warn("could not connect to webhook server", slog.String("error", err.Error()))
				time.Sleep(2 * time.Second)
				continue
			}

			ghClient := api.NewGithubClient(conn)

			// Call the webhook method to get a stream of webhook events.
			w, err := ghClient.Webhook(context.Background(), &api.Empty{})
			if err != nil {
				slog.Info("could not call webhook", slog.String("error", err.Error()))
				time.Sleep(2 * time.Second)
				continue
			}

			// Deal with with incoming webhook events. If end of stream is reached, reconnect.
			err = backend.GithubWebhook(w)
			if err != nil {
				slog.Warn("could not handle webhook response", slog.String("error", err.Error()))
				time.Sleep(2 * time.Second)
				continue
			}
		}

	// If server flags is set to true, run as server. This means starting gRPC server and HTTP server.
	// the gRPC server will be used for interapp communication while the HTTP server will listen for GitHub webhooks.
	case true:
		// Start gRPC server meant for interapp communication.
		grpcServer := grpc.NewServer()

		service, err := frontend.NewService()
		if err != nil {
			slog.Error("failed to create webhook service", slog.String("error", err.Error()))
			return
		}

		api.RegisterGithubServer(grpcServer, service)

		// Start listening for incoming gRPC connections in a non blocking way.
		go func() {
			lis, err := net.Listen("tcp", "0.0.0.0:3005")
			if err != nil {
				slog.Error("failed to listen", slog.String("error", err.Error()))
				return
			}

			slog.Info("gRPC server started on :3005")

			if err := grpcServer.Serve(lis); err != nil {
				slog.Error("failed to serve", slog.String("error", err.Error()))
			}
		}()

		// Start HTTP server to listen for GitHub webhooks.
		githubSecretFile := os.Getenv("GITHUB_SECRET")
		// Get the env viariable and cd in the super directory.
		dat, err := os.ReadFile(githubSecretFile)
		if err != nil {
			slog.Error("could not read super path file", slog.String("error", err.Error()))
			return
		}

		githubSecret := string(dat)

		hook, err := github.New(github.Options.Secret(githubSecret))
		if err != nil {
			slog.Error("could not create github webhook", slog.String("error", err.Error()))
			return
		}

		// Use the webhook package's HandleFunc to create an http handler function.
		http.HandleFunc(githubPath, frontend.HandleFunc(hook, service))

		slog.Info("server started on :3000")

		http.ListenAndServe(":3000", nil)
	}
}
