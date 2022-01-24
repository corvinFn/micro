package main

import (
	"log"

	"github.com/corvinFn/micro"
	pb "github.com/corvinFn/micro/example/protocol"
	"github.com/corvinFn/micro/example/server"
	"github.com/corvinFn/micro/gatewayhandler"
	"google.golang.org/grpc"
)

const port = 8080

func main() {
	server, err := server.NewExampleSvc()
	if err != nil {
		panic(err)
	}

	registerFucs := []gatewayhandler.RegisterFunc{pb.RegisterExampleSvcHandlerFromEndpoint}
	h := gatewayhandler.GatewayHandler(port, registerFucs)

	ms := micro.NewMicroService(
		micro.WithGRPC(func(grpcServer *grpc.Server) {
			pb.RegisterExampleSvcServer(grpcServer, server)
		}),
		micro.WithHttpHandler("/", h),
		micro.WithPprof(),
		micro.WithHTTPCORS(),
		micro.WithHealthCheck(),
		micro.WithTracer(),
		micro.WithPrometheus(),
		micro.WithGRPCUI(),
		// micro.WithLogger(lg.NewLogger()),
	)

	if err := ms.ListenAndServe(port); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
