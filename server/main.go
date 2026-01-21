package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bh90210/super/server/api"
	"github.com/bh90210/super/server/dupload"
	"github.com/bh90210/super/server/library"
	dgo "github.com/dgraph-io/dgo/v250"
	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	relationtuples "github.com/ory/keto/proto/ory/keto/relation_tuples/v1alpha2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
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
	ketoReadAddr := flag.String("keto-read-addr", "keto:4466", "Keto read gRPC server address")
	ketoWriteAddr := flag.String("keto-write-addr", "keto:4467", "Keto write gRPC server address")
	ketoTLS := flag.Bool("keto-tls", false, "Use TLS for Keto gRPC connection")
	ketoCA := flag.String("keto-ca", "", "Keto CA certificate file path (if using TLS)")
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
		slog.Error("failed to read minio password file", slog.String("error", err.Error()))
		return
	}

	mnpass := strings.TrimSpace(string(data))

	// Initialize minio client object.
	minioClient, err := minio.New(*minioEndpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(*minioUser, mnpass, ""),
		Secure: *miniossl,
	})
	if err != nil {
		slog.Error("failed to create minio client", slog.String("error", err.Error()))
		return
	}

	slog.Info("Connected to Minio", "endpoint", *minioEndpoint, "user", *minioUser, "ssl", *miniossl, "client", minioClient.EndpointURL().Scheme)

	// Keto client setup.
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // or TLS creds
		grpc.WithBlock(),
	}

	readConn, err := grpc.NewClient(*ketoReadAddr, dialOpts...)
	if err != nil {
		slog.Error("failed to connect to keto read server", slog.String("error", err.Error()))
		return
	}
	defer readConn.Close()

	writeConn, err := grpc.NewClient(*ketoWriteAddr, dialOpts...)
	if err != nil {
		slog.Error("failed to connect to keto write server", slog.String("error", err.Error()))
		return
	}
	defer writeConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	readHC := grpc_health_v1.NewHealthClient(readConn)
	if resp, err := readHC.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: ""}); err != nil {
		slog.Warn("keto read health check failed", "error", err.Error())
	} else {
		slog.Info("keto read health check ok", "status", resp.Status.String())
	}

	writeHC := grpc_health_v1.NewHealthClient(writeConn)
	if resp, err := writeHC.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: ""}); err != nil {
		slog.Warn("keto write health check failed", "error", err.Error())
	} else {
		slog.Info("keto write health check ok", "status", resp.Status.String())
	}

	slog.Info("Connected to Keto",
		"readAddress", *ketoReadAddr,
		"writeAddress", *ketoWriteAddr,
		"tls", *ketoTLS,
		"ketoCA", *ketoCA,
	)

	rt := relationtuples.NewReadServiceClient(readConn)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()

	tuplesResp, err := rt.ListRelationTuples(ctx2, &relationtuples.ListRelationTuplesRequest{})
	if err != nil {
		slog.Warn("keto ListRelationTuples failed", "error", err.Error())
	} else {
		slog.Info("keto ListRelationTuples ok", "response", tuplesResp)
	}

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

	// Create SSL credentials.
	creds, err := credentials.NewServerTLSFromFile(*cert, *key)
	if err != nil {
		log.Fatalf("failed to create credentials: %v", err)
	}

	// Use Credentials in gRPC server options.
	serverOption := grpc.Creds(creds)
	grpcServer := grpc.NewServer(serverOption)

	api.RegisterLibraryServer(grpcServer, libraryService)
	api.RegisterDuploadServer(grpcServer, duploadService)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
