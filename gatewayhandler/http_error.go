package gatewayhandler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Error struct {
	Message string `json:"message"`
	Code    int32  `json:"code"`
}

const MetadataHeaderPrefix = "Grpc-Metadata-"
const MetadataTrailerPrefix = "Grpc-Trailer-"

type contentTypeMarshaler interface {
	// ContentTypeFromMessage returns the Content-Type this marshaler produces from the provided message
	ContentTypeFromMessage(v interface{}) string
}

func HTTPError(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
	const fallback = `{"error": "failed to marshal error message"}`

	s, ok := status.FromError(err)
	if !ok {
		s = status.New(codes.Unknown, err.Error())
	}

	httpFlag := false
	errMsg := s.Message()
	if pos := strings.LastIndex(errMsg, "-"); pos > -1 {
		strCode := errMsg[pos+1 : len(errMsg)]
		if code, err := strconv.Atoi(strCode); err == nil {
			errMsg = errMsg[0:pos]
			s = status.New(codes.Code(code), errMsg)
			httpFlag = true
		}
	}

	w.Header().Del("Trailer")

	contentType := marshaler.ContentType()
	// Check marshaler on run time in order to keep backwards compatability
	// An interface param needs to be added to the ContentType() function on
	// the Marshal interface to be able to remove this check
	if typeMarshaler, ok := marshaler.(contentTypeMarshaler); ok {
		pb := s.Proto()
		contentType = typeMarshaler.ContentTypeFromMessage(pb)
	}
	w.Header().Set("Content-Type", contentType)

	body := Error{
		Message: s.Message(),
		Code:    int32(s.Code()),
	}

	buf, merr := marshaler.Marshal(body)
	if merr != nil {
		fmt.Printf("Failed to marshal error message %q: %v", body, merr)
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := io.WriteString(w, fallback); err != nil {
			fmt.Printf("Failed to write response: %v", err)
		}
		return
	}

	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		fmt.Println("Failed to extract ServerMetadata from context")
	}

	handleForwardResponseServerMetadata(w, md)
	handleForwardResponseTrailerHeader(w, md)
	st := runtime.HTTPStatusFromCode(s.Code())
	if httpFlag {
		if int(s.Code()) == int(http.StatusUnauthorized) { //用户登录状态返回401
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusOK)
		}

	} else {
		w.WriteHeader(st)
	}

	if _, err := w.Write(buf); err != nil {
		fmt.Printf("Failed to write response: %v", err)
	}

	handleForwardResponseTrailer(w, md)
}

func handleForwardResponseServerMetadata(w http.ResponseWriter, md runtime.ServerMetadata) {
	for k, vs := range md.HeaderMD {
		h := fmt.Sprintf("%s%s", MetadataHeaderPrefix, k)
		for _, v := range vs {
			w.Header().Add(h, v)
		}
	}
}

func handleForwardResponseTrailerHeader(w http.ResponseWriter, md runtime.ServerMetadata) {
	for k := range md.TrailerMD {
		tKey := textproto.CanonicalMIMEHeaderKey(fmt.Sprintf("%s%s", MetadataTrailerPrefix, k))
		w.Header().Add("Trailer", tKey)
	}
}

func handleForwardResponseTrailer(w http.ResponseWriter, md runtime.ServerMetadata) {
	for k, vs := range md.TrailerMD {
		tKey := fmt.Sprintf("%s%s", MetadataTrailerPrefix, k)
		for _, v := range vs {
			w.Header().Add(tKey, v)
		}
	}
}
