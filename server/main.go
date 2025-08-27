package main

import (
	"github.com/dedinirtadinata/docxtool/docgenpb"
	"github.com/dedinirtadinata/docxtool/middleware"
	"github.com/dedinirtadinata/docxtool/server/service"
	"github.com/dedinirtadinata/docxtool/workerpool"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"golang.org/x/time/rate"
	"log"
	"net"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
)

func main() {
	// init logger
	service.InitLogger()

	// start /metrics on :9090
	service.RegisterMetrics(":9090")

	wp := workerpool.NewWorkerPool(5)
	var limiter = rate.NewLimiter(2, 5) // 2 req/sec, burst 5

	// create gRPC server with chained interceptors:
	// order: auth -> logging -> prometheus
	unaryChain := grpc_middleware.ChainUnaryServer(
		service.UnaryAuthInterceptor,
		service.UnaryLoggingInterceptor,
		grpc_prometheus.UnaryServerInterceptor,
		middleware.RateLimiter(limiter),
	)

	streamChain := grpc_middleware.ChainStreamServer(
		service.StreamAuthInterceptor,
		service.StreamLoggingInterceptor,
		grpc_prometheus.StreamServerInterceptor,
	)

	// create server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(unaryChain),
		grpc.StreamInterceptor(streamChain),
	)

	// register service and prometheus
	svc := service.NewDocService(wp)
	docgenpb.RegisterDocServiceServer(grpcServer, svc)
	grpc_prometheus.Register(grpcServer)          // register metrics
	grpc_prometheus.EnableHandlingTimeHistogram() // optional

	// reflection for debug
	// reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":5051")
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	log.Println("gRPC server listening :5051, metrics at :9090/metrics")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve failed: %v", err)
	}
}
