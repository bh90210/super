package config

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bh90210/super/server/api"
	"github.com/bh90210/super/server/dupload"
	"github.com/bh90210/super/server/library"
	dgo "github.com/dgraph-io/dgo/v250"
	min "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	relationtuples "github.com/ory/keto/proto/ory/keto/relation_tuples/v1alpha2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.yaml.in/yaml/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var clientsRetryPolicy = `{
		"methodConfig": [{
		  "name": [{}],
		  "timeout": "5s",
		  "retryPolicy": {
			  "MaxAttempts": 10,
			  "InitialBackoff": "0.5s",
			  "MaxBackoff": "30s",
			  "BackoffMultiplier": 0.1,
			  "RetryableStatusCodes": [ "UNAVAILABLE" ]
		  }
		}]}`

type Config struct {
	// Server configuration.
	Server *server `yaml:"server"`
	// Dgraph configuration.
	Dgraph *dgraph `yaml:"dgraph"`
	// Minio configuration.
	Minio *minio `yaml:"minio"`
	// Keto configuration.
	Keto *keto `yaml:"keto"`
}

func Init(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		slog.Error("failed to read config file", slog.String("error", err.Error()))
		return err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		slog.Error("failed to unmarshal config", slog.String("error", err.Error()))
		return err
	}

	// Start backend services.
	return start(&config)
}

func start(c *Config) error {
	// Prometheus metrics server.
	go func() {
		for {
			slog.Info("starting metrics server on :" + c.Server.MetricsPort)

			http.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(":"+c.Server.MetricsPort, nil); err != nil {
				slog.Error("failed to start metrics server, retrying in 10 seconds", slog.String("error", err.Error()))
			}

			time.Sleep(10 * time.Second)
		}
	}()

	// Dgraph client.
	dgraphClient, err := c.Dgraph.connect()
	if err != nil {
		slog.Error("dgraph client", slog.String("error", err.Error()))
		return err
	}

	// Minio client.
	minioClient, err := c.Minio.connect()
	if err != nil {
		slog.Error("minio client", slog.String("error", err.Error()))
		return err
	}

	// Keto client.
	ketoRead, ketoWrite, err := c.Keto.connect()
	if err != nil {
		slog.Error("keto client", slog.String("error", err.Error()))
		return err
	}

	_ = dgraphClient
	_ = minioClient
	_ = ketoRead
	_ = ketoWrite

	libraryService, err := library.NewService(c.Server.LibraryPath)
	if err != nil {
		slog.Error("failed to create library service", slog.String("error", err.Error()))
		return err
	}

	duploadService, err := dupload.NewService(c.Server.LibraryPath)
	if err != nil {
		slog.Error("failed to create dupload service", slog.String("error", err.Error()))
		return err
	}

	// Create SSL credentials.
	creds, err := credentials.NewServerTLSFromFile(c.Server.SSLCertPath, c.Server.SSLKeyPath)
	if err != nil {
		slog.Error("failed to create credentials", slog.String("error", err.Error()))
		return err
	}

	// Use Credentials in gRPC server options.
	serverOption := grpc.Creds(creds)
	grpcServer := grpc.NewServer(serverOption)

	api.RegisterLibraryServer(grpcServer, libraryService)
	api.RegisterDuploadServer(grpcServer, duploadService)

	lis, err := net.Listen("tcp", c.Server.ListenAddress+":"+c.Server.ListenPort)
	if err != nil {
		slog.Error("failed to listen", slog.String("error", err.Error()))
		return err
	}

	slog.Info("starting gRPC server on " + c.Server.ListenAddress + ":" + c.Server.ListenPort)

	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("failed to serve", slog.String("error", err.Error()))
		return err
	}

	return nil
}

type server struct {
	LibraryPath   string `yaml:"library_path"`
	SSLCertPath   string `yaml:"ssl_cert_path"`
	SSLKeyPath    string `yaml:"ssl_key_path"`
	ListenPort    string `yaml:"listen_port"`
	MetricsPort   string `yaml:"metrics_port"`
	ListenAddress string `yaml:"listen_address"`
}

type dgraph struct {
	Addresses []string `yaml:"addresses"`
}

func (d *dgraph) connect() (*dgo.Dgraph, error) {
	client, err := dgo.NewRoundRobinClient(d.Addresses,
		// add Dgraph ACL credentials
		// dgo.WithACLCreds("groot", "password"),
		// add insecure transport credentials
		dgo.WithGrpcOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		dgo.WithGrpcOption(grpc.WithDefaultServiceConfig(clientsRetryPolicy)),
	)
	if err != nil {
		slog.Error("failed to create dgraph client", slog.String("error", err.Error()))
		return nil, err
	}

	// Test connection with a simple query.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	txn := client.NewReadOnlyTxn()
	defer txn.Discard(ctx)

	_, err = txn.Query(ctx, `{ q(func: has(dgraph.type)) { uid } }`)
	if err != nil {
		slog.Error("dgraph health check failed", slog.String("error", err.Error()))
		return nil, err
	}

	slog.Info("Connected to Dgraph", "addresses", strings.Join(d.Addresses, ","))

	return client, nil
}

type minio struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
}

func (m *minio) connect() (*min.Client, error) {
	// Connect to minio server.
	data, err := os.ReadFile(m.SecretKey)
	if err != nil {
		slog.Error("failed to read minio password file", slog.String("error", err.Error()))
		return nil, err
	}

	mnpass := strings.TrimSpace(string(data))

	// Initialize minio client object.
	minioClient, err := min.New(m.Endpoint, &min.Options{
		Creds:  miniocreds.NewStaticV4(m.AccessKey, mnpass, ""),
		Secure: m.UseSSL,
	})
	if err != nil {
		slog.Error("failed to create minio client", slog.String("error", err.Error()))
		return nil, err
	}

	// Test connection with a health check.
	_, err = minioClient.HealthCheck(time.Second * 5)
	if err != nil {
		slog.Error("minio health check failed", slog.String("error", err.Error()))
		return nil, err
	}

	slog.Info("Connected to Minio", "endpoint", m.Endpoint, "user", m.AccessKey, "ssl", m.UseSSL, "client", minioClient.EndpointURL().Scheme)

	return minioClient, nil
}

type keto struct {
	ReadAddress  string `yaml:"read_address"`
	WriteAddress string `yaml:"write_address"`
	UseTLS       bool   `yaml:"use_tls"`
	CACertPath   string `yaml:"ca_cert_path"`
}

func (k *keto) connect() (*grpc.ClientConn, *grpc.ClientConn, error) {
	// Keto client setup.
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // or TLS creds
		grpc.WithDefaultServiceConfig(clientsRetryPolicy),
	}

	readConn, err := grpc.NewClient(k.ReadAddress, dialOpts...)
	if err != nil {
		slog.Error("failed to connect to keto read server", slog.String("error", err.Error()))
		return nil, nil, err
	}
	defer readConn.Close()

	writeConn, err := grpc.NewClient(k.WriteAddress, dialOpts...)
	if err != nil {
		slog.Error("failed to connect to keto write server", slog.String("error", err.Error()))
		return nil, nil, err
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
		"readAddress", k.ReadAddress,
		"writeAddress", k.WriteAddress,
		"tls", k.UseTLS,
		"ketoCA", k.CACertPath,
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

	return readConn, writeConn, nil
}
