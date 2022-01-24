package tracer

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const mdGinOutgoingKey = "mdGinOutgoingKey"

// Gin start point, defer span.Finish()
func GinStart(c *gin.Context, operationName string) opentracing.Span {
	var err error
	var spanCtx opentracing.SpanContext
	md, _ := c.Get(mdGinOutgoingKey)
	if md != nil {
		spanCtx, _ = GetTracer().Extract(opentracing.TextMap, MDReaderWriter{md.(metadata.MD)})
	} else {
		spanCtx, _ = GetTracer().Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(c.Request.Header))
	}

	span := opentracing.StartSpan(
		operationName,
		ext.RPCServerOption(spanCtx),
		opentracing.Tag{Key: string(ext.Component), Value: "Gin-Http"},
		ext.SpanKindRPCClient,
	)

	carrier := opentracing.TextMapCarrier{}
	err = GetTracer().Inject(span.Context(), opentracing.TextMap, carrier)
	if err != nil {
		fmt.Println("inject-error", err.Error())
	}
	c.Set(mdGinOutgoingKey, metadata.New(carrier))
	return span
}

// Gin client
func GinGrpcDialOption() grpc.DialOption {
	return grpc.WithUnaryInterceptor(ginClientInterceptor(GetTracer()))
}

// Gin ginClientInterceptor
func ginClientInterceptor(tracer opentracing.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string,
		req, reply interface{}, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// md := ctx.Value(mdGinOutgoingKey).(metadata.MD)
		md := metadata.New(nil)
		spanCtx, _ := tracer.Extract(opentracing.TextMap, MDReaderWriter{md})
		span := tracer.StartSpan(
			method,
			ext.RPCServerOption(spanCtx),
			opentracing.Tag{Key: string(ext.Component), Value: "Gin-gRPC"},
			ext.SpanKindRPCClient,
		)
		defer span.Finish()
		mdWriter := MDReaderWriter{md}
		err := tracer.Inject(span.Context(), opentracing.TextMap, mdWriter)
		if err != nil {
			fmt.Println("inject-error", err.Error())
		}
		parentCtx := metadata.NewOutgoingContext(ctx, md)
		err = invoker(parentCtx, method, req, reply, cc, opts...)
		if err != nil {
			fmt.Println("call-error", err.Error())
		}
		return err
	}
}
