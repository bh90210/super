package main

import (
	"flag"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bh90210/super/server/api"
	"github.com/bh90210/super/server/dupload"
	"github.com/bh90210/super/server/library"
	dgo "github.com/dgraph-io/dgo/v250"
	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	path := flag.String("path", "", "Library path")
	cert := flag.String("cert", "", "Certificate path")
	key := flag.String("key", "", "Key path")
	port := flag.Int("port", 8888, "Port to listen on")
	metricsPort := flag.Int("metrics-port", 2112, "Port for prometheus metrics")
	address := flag.String("address", "0.0.0.0", "Address to listen on")
	dgraphAddr := flag.String("dgraph", "", "Comma-separated list of Dgraph addresses")
	minioEndpoint := flag.String("minio-endpoint", "", "Minio server endpoint")
	minioUser := flag.String("minio-user", "", "Minio access key ID")
	minioPass := flag.String("minio-pass", "", "Minio secret access key")
	miniossl := flag.Bool("minio-ssl", false, "Use SSL for Minio connection")
	flag.Parse()

	// Connect to dgraph database.
	if *dgraphAddr != "" {
		addresses := []string{}
		for addr := range strings.SplitSeq(*dgraphAddr, ",") {
			addresses = append(addresses, addr)
		}

		client, err := dgo.NewRoundRobinClient(addresses,
			// add Dgraph ACL credentials
			// dgo.WithACLCreds("groot", "password"),
			// add insecure transport credentials
			dgo.WithGrpcOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		)
		if err != nil {
			log.Fatalf("failed to connect to dgraph: %v", err)
		}

		defer client.Close()
	}

	// Connect to minio server.
	data, err := os.ReadFile(*minioPass)
	if err != nil {
		log.Fatalf("failed to read minio password file: %v", err)
	}

	mnpass := strings.TrimSpace(string(data))

	// Initialize minio client object.
	minioClient, err := minio.New(*minioEndpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(*minioUser, mnpass, ""),
		Secure: *miniossl,
	})
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("%#v\n", minioClient) // minioClient is now set up

	// Open prometheus metrics endpoint.
	go func() {
		slog.Info("starting metrics server on :" + strconv.Itoa(*metricsPort))

		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":"+strconv.Itoa(*metricsPort), nil)
	}()

	//
	// Start server service.
	//
	lis, err := net.Listen("tcp", *address+":"+strconv.Itoa(*port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	slog.Info("Starting library", "path", *path)

	libraryService, err := library.NewService(*path)
	if err != nil {
		log.Fatalf("failed to create library service: %v", err)
	}

	slog.Info("Library started", "path", *path)

	duploadService, err := dupload.NewService(*path)
	if err != nil {
		log.Fatalf("failed to create dupload service: %v", err)
	}

	// Create credentials
	creds, err := credentials.NewServerTLSFromFile(*cert, *key)
	if err != nil {
		log.Fatalf("failed to create credentials: %v", err)
	}

	// Use Credentials in gRPC server options
	serverOption := grpc.Creds(creds)
	grpcServer := grpc.NewServer(serverOption)

	api.RegisterLibraryServer(grpcServer, libraryService)
	api.RegisterDuploadServer(grpcServer, duploadService)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
