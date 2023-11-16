package gatewayhandler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/corvinFn/micro/tracer"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
)

type HttpServer struct {
	R *runtime.ServeMux
}

type RegisterFunc func(ctx context.Context, mux *runtime.ServeMux, addr string, opts []grpc.DialOption) error

func (s *HttpServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	defer TimeFunc("Path:", req.URL.Path)()
	span := tracer.GatewayStart(req, req.RequestURI)
	defer span.Finish()
	s.R.ServeHTTP(rw, req)
}

var xHeaderMapping = map[string]string{
	"appkey":        "Appkey",
	"access-token":  "Access-Token",
	"uber-trace-id": "Uber-Trace-Id",
}

func HttpHeaderMatcher(headerName string) (mdName string, ok bool) {
	mdName = xHeaderMapping[strings.ToLower(headerName)]
	if mdName != "" {
		return mdName, true
	}
	return "", false
}

func GatewayHandler(port int, registerEndPointFuncs []RegisterFunc) http.Handler {
	addr := fmt.Sprintf("localhost:%d", port)

	runtime.WithErrorHandler(HTTPError)

	ctx := context.Background()
	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(HttpHeaderMatcher),
		runtime.WithMarshalerOption(
			runtime.MIMEWildcard,
			&runtime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					UseProtoNames:   true,
					UseEnumNumbers:  true,
					EmitUnpopulated: true,
				},
			},
		),
	)
	opts := []grpc.DialOption{grpc.WithInsecure()}

	for _, f := range registerEndPointFuncs {
		err := f(ctx, mux, addr, opts)
		if err != nil {
			panic(err)
		}
	}

	s := HttpServer{
		R: mux,
	}

	return &s
}

// usage: defer TimeFunc("Hello world")()
func TimeFunc(v ...interface{}) func() {
	start := time.Now()
	return func() {
		fmt.Println(append(v, "|", time.Since(start)))
	}
}
