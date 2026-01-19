package main

import (
	"crypto/tls"
	"flag"
	"log"
	"log/slog"
	"net"
	"os"

	"github.com/bh90210/super/server/api"
	"github.com/bh90210/super/server/dupload"
	"github.com/bh90210/super/server/library"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	path := flag.String("path", "", "Library path")
	flag.Parse()

	lis, err := net.Listen("tcp", "0.0.0.0:8888")
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

	// Read cert and key file
	backendCert, err := os.ReadFile("/server.pem")
	if err != nil {
		log.Fatalf("failed to read certificate file: %v", err)
	}

	backendKey, err := os.ReadFile("/server-key.pem")
	if err != nil {
		log.Fatalf("failed to read key file: %v", err)
	}

	// Generate Certificate struct
	cert, err := tls.X509KeyPair(backendCert, backendKey)
	if err != nil {
		log.Fatalf("failed to parse certificate: %v", err)
	}

	// Create credentials
	creds := credentials.NewServerTLSFromCert(&cert)

	// Use Credentials in gRPC server options
	serverOption := grpc.Creds(creds)
	var s *grpc.Server = grpc.NewServer(serverOption)
	defer s.Stop()

	grpcServer := grpc.NewServer()
	api.RegisterLibraryServer(grpcServer, libraryService)
	api.RegisterDuploadServer(grpcServer, duploadService)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
