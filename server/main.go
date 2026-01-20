package main

import (
	"flag"
	"log"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/bh90210/super/server/api"
	"github.com/bh90210/super/server/dupload"
	"github.com/bh90210/super/server/library"
	dgo "github.com/dgraph-io/dgo/v250"
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
	address := flag.String("address", "0.0.0.0", "Address to listen on")
	dgraphAddr := flag.String("dgraph", "", "Comma-separated list of Dgraph addresses")
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
	// endpoint := "play.min.io"
	// accessKeyID := "Q3AM3UQ867SPQQA43P2F"
	// secretAccessKey := "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG"
	// useSSL := true

	// // Initialize minio client object.
	// minioClient, err := minio.New(endpoint, &minio.Options{
	// 	Creds:  minioCreds.NewStaticV4(accessKeyID, secretAccessKey, ""),
	// 	Secure: useSSL,
	// })
	// if err != nil {
	// 	log.Fatalln(err)
	// }

	// log.Printf("%#v\n", minioClient) // minioClient is now set up

	// Open prometheus metrics endpoint.
	go func() {
		slog.Info("starting metrics server on :2112")

		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()

	// Start backend service.
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
