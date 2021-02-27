package tracer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	_ "github.com/uber/jaeger-client-go/zipkin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func InitTracer(uri string, appName string) {
	fmt.Println("Init tracer: ", uri)
	span, _, err := NewJaegerTracer(appName, uri)
	if err != nil || span == nil {
		fmt.Println("Failed to init tracer: ", err)
	}
}

// MDReaderWriter metadata Reader and Writer
type MDReaderWriter struct {
	metadata.MD
}

type Tracer struct {
	opentracing.Tracer
}

func GetTracer() Tracer {
	return Tracer{opentracing.GlobalTracer()}
}

// grpc start point, defer span.Finish()
func Start(ctx *context.Context, operationName string) opentracing.Span {
	var parentCtx opentracing.SpanContext
	parentSpan := opentracing.SpanFromContext(*ctx)
	if parentSpan != nil {
		parentCtx = parentSpan.Context()
	}
	span := GetTracer().StartSpan(operationName, opentracing.ChildOf(parentCtx), opentracing.Tag{Key: string(ext.Component), Value: "gRPC-Custom"})
	return span
}

// grpc-geteway start point, defer span.Finish()
func GatewayStart(r *http.Request, operationName string) opentracing.Span {
	spanCtx, _ := GetTracer().Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	span := opentracing.StartSpan(
		operationName,
		ext.RPCServerOption(spanCtx),
		opentracing.Tag{Key: string(ext.Component), Value: "gRPC-Gateway-Custom"},
		ext.SpanKindRPCClient,
	)
	injectErr := GetTracer().Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	if injectErr != nil {
		fmt.Println("Couldn't inject headers", injectErr)
	}
	return span
}

// get trace id
func TraceID(ctx *context.Context) (jaeger.TraceID, error) {
	span := opentracing.SpanFromContext(*ctx)
	if span == nil {
		return jaeger.TraceID{}, errors.New("span is not finalized when created")
	}
	if sc, ok := span.Context().(jaeger.SpanContext); ok {
		return sc.TraceID(), nil
	}
	return jaeger.TraceID{}, errors.New("failed to get TraceID")
}

// get span
func GetSpan(ctx *context.Context) opentracing.Span {
	span := opentracing.SpanFromContext(*ctx)
	return span
}

// grpc server
func GrpcServerOption() grpc.ServerOption {
	return ServerOption(GetTracer())
}

// grpc client
func GrpcDialOption() grpc.DialOption {
	return DialOption(GetTracer())
}

// grpc-gateway client
func GrpcGatewayDialOption() grpc.DialOption {
	return grpc.WithUnaryInterceptor(gatewayClientInterceptor(GetTracer()))
}

// grpc-gateway gatewayClientInterceptor
func gatewayClientInterceptor(tracer opentracing.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string,
		req, reply interface{}, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}
		spanContext, err := tracer.Extract(opentracing.TextMap, MDReaderWriter{md})
		span := tracer.StartSpan(
			method,
			ext.RPCServerOption(spanContext),
			opentracing.Tag{Key: string(ext.Component), Value: "gRPC-Gateway"},
			ext.SpanKindRPCClient,
		)
		defer span.Finish()
		mdWriter := MDReaderWriter{md}
		err = tracer.Inject(span.Context(), opentracing.TextMap, mdWriter)
		if err != nil {
			fmt.Println("inject-error", err.Error())
		}
		err = invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			fmt.Println("call-error", err.Error())
		}
		return err
	}
}

// ForeachKey implements ForeachKey of opentracing.TextMapReader
func (c MDReaderWriter) ForeachKey(handler func(key, val string) error) error {
	for k, vs := range c.MD {
		for _, v := range vs {
			if err := handler(k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// Set implements Set() of opentracing.TextMapWriter
func (c MDReaderWriter) Set(key, val string) {
	key = strings.ToLower(key)
	c.MD[key] = append(c.MD[key], val)
}

// NewJaegerTracer NewJaegerTracer for current service
func NewJaegerTracer(serviceName string, JaegerAgentUri string) (tracer opentracing.Tracer, closer io.Closer, err error) {

	// zipkinPropagator := zipkin.NewZipkinB3HTTPHeaderPropagator()
	jcfg := jaegercfg.Configuration{
		Sampler: &jaegercfg.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
			LocalAgentHostPort:  JaegerAgentUri,
		},
	}
	jcfg.ServiceName = serviceName
	tracer, closer, err = jcfg.NewTracer(
		jaegercfg.Logger(jaeger.StdLogger),
		// jaegercfg.Injector(opentracing.HTTPHeaders, zipkinPropagator),
		// jaegercfg.Extractor(opentracing.HTTPHeaders, zipkinPropagator),
		// jaegercfg.ZipkinSharedRPCSpan(true),
	)
	if err != nil {
		return
	}
	opentracing.SetGlobalTracer(tracer)
	return
}

// DialOption grpc client option
func DialOption(tracer opentracing.Tracer) grpc.DialOption {
	return grpc.WithUnaryInterceptor(grpc_opentracing.UnaryClientInterceptor(
		grpc_opentracing.WithTracer(tracer),
	))
}

// ServerOption grpc server option
func ServerOption(tracer opentracing.Tracer) grpc.ServerOption {
	return grpc.UnaryInterceptor(grpc_opentracing.UnaryServerInterceptor(grpc_opentracing.WithTracer(tracer)))
}
