package micro

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/fullstorydev/grpcui/standalone"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/reflection"

	"micro/tracer"
)

// MicroService is a All-in-one container for hosting grpc and/or http server,
// along with multiple predefined utils.
type MicroService struct {
	conn                      *grpc.ClientConn
	grpcOptions               []grpc.ServerOption
	unaryInterceptors         []grpc.UnaryServerInterceptor
	streamInterceptors        []grpc.StreamServerInterceptor
	grpcPreprocess            []func(srv *grpc.Server)
	grpcFlag                  bool
	grpcIncomingHeaderMapping map[string]string
	grpcOutgoingHeaderMapping map[string]string
	grpcConnCallbacks         []func(conn *grpc.ClientConn)
	httpMux                   *http.ServeMux
	httpCORS                  bool
	withGRPCUI                bool
	withHealthCheck           bool
	aggresiveGC               bool
	ctx                       context.Context
	workers                   []func(context.Context) error
	logger                    Logger

	gatewayAPIPrefix []string
	gatewayHandlers  []gatewayFunc
}

// NewMicroService return an object to setup micro service in cloud.
func NewMicroService(opts ...MicroServiceOption) *MicroService {
	ms := &MicroService{
		httpMux: http.NewServeMux(),
		ctx:     context.Background(),
		grpcIncomingHeaderMapping: map[string]string{
			"appkey":        "Appkey",
			"access-token":  "Access-Token",
			"uber-trace-id": "Uber-Trace-Id",
		},
		grpcOutgoingHeaderMapping: map[string]string{
			"uber-trace-id": "Uber-Trace-Id",
		},
	}

	// use golden-cloud/logger as default logger library
	/*
		if ms.logger == nil {
			ms.logger = logger.GetLogger()
		}
	*/

	for _, opt := range opts {
		opt(ms)
	}

	return ms
}

// MicroServiceOption offer config tuning for micro service.
type MicroServiceOption func(ms *MicroService)

type gatewayFunc func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error

// WithServerOptions sets the GRPC server options.
// Common used to set grpc message size, like
// `service.WithServerOptions(grpc.MaxRecvMsgSize(1024*1024*16))`
func WithServerOptions(options ...grpc.ServerOption) MicroServiceOption {
	return func(ms *MicroService) {
		ms.grpcOptions = append(
			ms.grpcOptions,
			options...,
		)
	}
}

// WithGrpcUnaryServerInterceptors support user to set user-definition lower-level gRPC interceptors directly by themselves.
func WithGrpcUnaryServerInterceptors(unaryServerInterceptors ...grpc.UnaryServerInterceptor) MicroServiceOption {
	return func(ms *MicroService) {
		ms.unaryInterceptors = append(ms.unaryInterceptors, unaryServerInterceptors...)
	}
}

func WithTracer() MicroServiceOption {
	return func(ms *MicroService) {
		ms.unaryInterceptors = append(
			ms.unaryInterceptors,
			grpc_opentracing.UnaryServerInterceptor(grpc_opentracing.WithTracer(tracer.GetTracer())),
		)

		ms.streamInterceptors = append(
			ms.streamInterceptors,
			grpc_opentracing.StreamServerInterceptor(grpc_opentracing.WithTracer(tracer.GetTracer())),
		)
	}
}

// WithWorker adds dependency worker with this service. If the worker returns,
// the micro service will be terminated.
func WithWorker(worker func(context.Context) error) MicroServiceOption {
	return func(ms *MicroService) {
		ms.logger.Debug("Added worker")
		ms.workers = append(ms.workers, worker)
	}
}

// WithContext sets the context for whole microservice. When ctx get done, the microservice
// will be terminated.
func WithContext(ctx context.Context) MicroServiceOption {
	return func(ms *MicroService) {
		ms.logger.Debug("Added context")
		ms.ctx = ctx
	}
}

// WithGRPC binds a grpc server.
// Example usage:
// ```service.WithhGRPC(func(gs *grpc.Server) {
//    pb.RegisterServiceServer(gs, NewServiceServer())
// }```
func WithGRPC(preprocess func(srv *grpc.Server)) MicroServiceOption {
	return func(ms *MicroService) {
		ms.logger.Debug("Enabled GRPC")
		ms.grpcFlag = true
		ms.grpcPreprocess = append(ms.grpcPreprocess, preprocess)
	}
}

// WithHTTPCORS enables cors for http endpoint. Should not enable in production.
func WithHTTPCORS() MicroServiceOption {
	return func(ms *MicroService) {
		ms.logger.Debug("Enabled HTTP CORS")
		ms.httpCORS = true
	}
}

// WithHttpHandler binds a http server implementing http.Handler
func WithHttpHandler(pattern string, handler http.Handler) MicroServiceOption {
	return func(ms *MicroService) {
		if !strings.HasSuffix(pattern, "/") {
			pattern = pattern + "/"
		}
		ms.logger.Debug("Added http handler for", pattern)

		prefix := strings.TrimSuffix(pattern, "/")
		ms.httpMux.Handle(pattern, http.StripPrefix(prefix, handler))
	}
}

// WithPprof register the golang built-in profiling interface to `/debug` http path.
func WithPprof() MicroServiceOption {
	return func(ms *MicroService) {
		ms.logger.Debug("Enabled pprof")
		ms.httpMux.HandleFunc("/debug/pprof/", pprof.Index)
		ms.httpMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		ms.httpMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		ms.httpMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		ms.httpMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		ms.httpMux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		ms.httpMux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		ms.httpMux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		ms.httpMux.Handle("/debug/pprof/block", pprof.Handler("block"))
	}
}

// WithPrometheus registers the service to monitor infrastructure.
func WithPrometheus(handlerFuncs ...http.HandlerFunc) MicroServiceOption {

	addMiddlwares := func(handler http.Handler, middlewares ...http.HandlerFunc) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			for _, middleware := range middlewares {
				middleware(w, r)
			}
			handler.ServeHTTP(w, r)
		}
		return http.HandlerFunc(f)
	}

	return func(ms *MicroService) {
		ms.logger.Debug("Enabled prometheus")
		ms.unaryInterceptors = append(ms.unaryInterceptors, grpc_prometheus.UnaryServerInterceptor)
		ms.streamInterceptors = append(ms.streamInterceptors, grpc_prometheus.StreamServerInterceptor)
		ms.grpcPreprocess = append(ms.grpcPreprocess, func(grpcServer *grpc.Server) {
			grpc_prometheus.EnableClientHandlingTimeHistogram()
			grpc_prometheus.EnableHandlingTimeHistogram()
			grpc_prometheus.Register(grpcServer)
		})

		promHandler := promhttp.Handler()
		promHandler = addMiddlwares(promHandler, handlerFuncs...)
		ms.httpMux.Handle("/metrics", promHandler)

	}
}

// WithGRPCUI registers a grpc ui for easy interact with GRPC server like postmen.
func WithGRPCUI() MicroServiceOption {
	return func(ms *MicroService) {
		ms.logger.Debug("Enabled GRPC UI")
		ms.withGRPCUI = true
	}
}

// WithRestfulGateway binds GRPC gateway handler for all registered grpc service.
func WithRestfulGateway(apiPrefix string, handler gatewayFunc) MicroServiceOption {
	return func(ms *MicroService) {
		ms.logger.Debug("Enabled GRPC HTTP Gateway", apiPrefix)
		prefix := strings.TrimSuffix(apiPrefix, "/")
		ms.gatewayAPIPrefix = append(ms.gatewayAPIPrefix, prefix)
		ms.gatewayHandlers = append(ms.gatewayHandlers, handler)
	}
}

// WithGRPCHeaderMapping redirect http header to GRPC metadata.
func WithGRPCHeaderMapping(m map[string]string) MicroServiceOption {
	return func(ms *MicroService) {
		ms.grpcIncomingHeaderMapping = m
	}
}

// WithGRPCConnCallback is invoked after GRPC server starts serving, and a self-connect
// GRPC connection will be dialed and be returned in the callback.
func WithGRPCConnCallback(cb func(conn *grpc.ClientConn)) MicroServiceOption {
	return func(ms *MicroService) {
		ms.grpcConnCallbacks = append(ms.grpcConnCallbacks, cb)
	}
}

func (ms *MicroService) httpIncomingHeaderMatcher(headerName string) (mdName string, ok bool) {
	if len(ms.grpcIncomingHeaderMapping) == 0 {
		return "", false
	}

	key := strings.ToLower(headerName)
	mdName, exists := ms.grpcIncomingHeaderMapping[key]
	return mdName, exists
}

func (ms *MicroService) httpOutgoingHeaderMatcher(headerName string) (mdName string, ok bool) {
	if len(ms.grpcOutgoingHeaderMapping) == 0 {
		return "", false
	}

	key := strings.ToLower(headerName)
	mdName, exists := ms.grpcOutgoingHeaderMapping[key]
	return mdName, exists
}

// WithAggresiveGC invokes Garbage Collecting periodically, to reduce small chunk of files
// memory footprint reserved by Go.
func WithAggresiveGC() MicroServiceOption {
	return func(ms *MicroService) {
		ms.logger.Debug("Enabled aggressive garbage collector")
		ms.aggresiveGC = true
	}
}

// Serve starts the micro service server using given listener.
func (ms *MicroService) Serve(listener net.Listener) error {
	ms.grpcOptions = append(ms.grpcOptions, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(ms.unaryInterceptors...)))
	ms.grpcOptions = append(ms.grpcOptions, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(ms.streamInterceptors...)))

	grpcServer := grpc.NewServer(ms.grpcOptions...)
	for _, fn := range ms.grpcPreprocess {
		fn(grpcServer)
	}
	reflection.Register(grpcServer)

	m := cmux.New(listener)
	grpcListener := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpListener := m.Match(cmux.HTTP1Fast(), cmux.HTTP2())

	if ms.withHealthCheck || len(ms.gatewayHandlers) > 0 || ms.withGRPCUI || len(ms.grpcConnCallbacks) > 0 {
		opts := []grpc.DialOption{
			grpc.WithInsecure(),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16 * 1024 * 1024)),
		}
		_, port, _ := net.SplitHostPort(listener.Addr().String())
		target := fmt.Sprintf("127.0.0.1:%s", port)
		var err error
		ms.conn, err = grpc.DialContext(ms.ctx, target, opts...)
		if err != nil {
			ms.logger.Error("Failed to dial", err)
			return errors.Wrap(err, "Gateway dialing to grpc")
		}
		ms.logger.Debug("Inited self connect")
	}

	for _, cb := range ms.grpcConnCallbacks {
		cb(ms.conn)
	}

	if len(ms.gatewayHandlers) > 0 {
		gwmux := runtime.NewServeMux(
			runtime.WithIncomingHeaderMatcher(ms.httpIncomingHeaderMatcher),
			runtime.WithOutgoingHeaderMatcher(ms.httpOutgoingHeaderMatcher),
		)
		runtime.SetHTTPBodyMarshaler(gwmux)

		for i := 0; i < len(ms.gatewayHandlers); i++ {
			if err := ms.gatewayHandlers[i](ms.ctx, gwmux, ms.conn); err != nil {
				return errors.Wrapf(err, "Register %d gateway handler", i)
			}
			ms.httpMux.Handle(ms.gatewayAPIPrefix[i]+"/", http.StripPrefix(ms.gatewayAPIPrefix[i], gwmux))
		}
	}
	if ms.withGRPCUI {
		go func() {
			target := listener.Addr().String()
			handler, err := standalone.HandlerViaReflection(ms.ctx, ms.conn, target)
			if err != nil {
				ms.logger.Error("Failed to start GRPCUI: ", err.Error())
			} else {
				ms.httpMux.Handle("/debug/", http.StripPrefix("/debug", handler))
			}
		}()
	}

	var httpMux http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.httpMux.ServeHTTP(w, r)
	})
	if ms.httpCORS {
		httpMux = cors.AllowAll().Handler(httpMux)
	}

	terminated := make(chan error)
	go func() {
		terminated <- http.Serve(httpListener, httpMux)
	}()
	go func() {
		terminated <- grpcServer.Serve(grpcListener)
	}()
	go func() {
		terminated <- m.Serve()
	}()
	go func() {
		ch := make(chan os.Signal)
		signal.Notify(ch,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT,
			syscall.SIGKILL,
		)
		sg := <-ch
		if ms.conn != nil {
			ms.conn.Close()
		}
		if ms.grpcFlag {
			ms.logger.Infof("Graceful stoping grpc server, Signal:%s", sg.String())
			grpcServer.GracefulStop()
		}

		terminated <- errors.Errorf("Signal: %s", sg.String())
	}()
	go func() {
		<-ms.ctx.Done()
		terminated <- errors.New("Context get done")
	}()
	for _, w := range ms.workers {
		worker := w
		go func() {
			terminated <- worker(ms.ctx)
		}()
	}
	if ms.aggresiveGC {
		go func() {
			for range time.Tick(time.Second * 10) {
				debug.FreeOSMemory()
			}
		}()
	}

	ms.logger.Info("Listening", listener.Addr().String())
	return <-terminated
}

// ListenAndServe start service server by given `port`,
// If `port==0`, a random available port will be used.
func (ms *MicroService) ListenAndServe(port int) error {
	addr := ""
	if port > 0 {
		addr = fmt.Sprintf(":%d", port)
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return ms.Serve(lis)
}

// WithHealthCheck register the health check handler for k8s
// use "/healthz" compromise to mistake in Kubernetes
func WithHealthCheck() MicroServiceOption {
	return func(ms *MicroService) {
		healthCheck := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			if s := ms.conn.GetState(); s != connectivity.Ready {
				http.Error(w, fmt.Sprintf("grpc server is %s", s), http.StatusBadGateway)
				return
			}
			fmt.Fprintln(w, "ok")
		}

		ms.httpMux.HandleFunc("/healthz", healthCheck)
		ms.withHealthCheck = true
	}
}

type HandlerFuncGenerator func(conn *grpc.ClientConn) http.HandlerFunc

func WithLogger(logger Logger) MicroServiceOption {
	return func(ms *MicroService) {
		ms.logger = logger
	}
}
