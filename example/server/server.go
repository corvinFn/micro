package server

import (
	"context"
	"fmt"

	pb "github.com/corvinFn/micro/example/protocol"
)

type ExampleSvc struct {
}

func NewExampleSvc() (*ExampleSvc, error) {
	return &ExampleSvc{}, nil
}

func (s *ExampleSvc) Ping(ctx context.Context, in *pb.PingReq) (*pb.PingRsp, error) {
	rsp := &pb.PingRsp{}

	if in.GetName() == "" {
		rsp.Msg = "hello, who's that"
	} else {
		rsp.Msg = fmt.Sprintf("hello %s!", in.GetName())
	}

	return rsp, nil
}
