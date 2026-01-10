package main

import (
	"flag"
	"log"
	"log/slog"
	"net"

	"github.com/bh90210/super/api"
	"github.com/bh90210/super/server/dupload"
	"github.com/bh90210/super/server/library"
	"google.golang.org/grpc"
)

func main() {
	path := flag.String("path", "", "Library path")
	flag.Parse()

	lis, err := net.Listen("tcp", "0.0.0.0:8080")
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

	// creds, err := credentials.NewClientTLSFromFile("roots.pem", "")
	grpcServer := grpc.NewServer()
	api.RegisterLibraryServer(grpcServer, libraryService)
	api.RegisterDuploadServer(grpcServer, duploadService)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
