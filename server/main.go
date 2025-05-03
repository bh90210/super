package main

import (
	"flag"
	"log"
	"net"

	"github.com/bh90210/super/api"
	"github.com/bh90210/super/server/library"
	"google.golang.org/grpc"
)

func main() {
	path := flag.String("path", "", "Library path")
	flag.Parse()

	lis, err := net.Listen("tcp", "0.0.0.0:80")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(1024*1024*1024),
		grpc.MaxSendMsgSize(2024*1024*1024),
	)
	api.RegisterLibraryServer(grpcServer, &library.Service{
		LibraryPath: *path,
	})

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
