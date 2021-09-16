package gormtracer

import (
	"context"
	"runtime"

	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
	otgorm "github.com/smacker/opentracing-gorm"
)

// GormTracerHandler clone baseDb with span info from baseCtx, and execute callback f with cloned db
func GormTracerHandler(baseCtx context.Context, baseDB *gorm.DB, f func(db *gorm.DB) error) error {
	handlerName := traceCaller(3)
	span, ctx := opentracing.StartSpanFromContext(baseCtx, handlerName)
	db := otgorm.SetSpanToGorm(ctx, baseDB)
	defer span.Finish()

	return f(db)
}

// traceCaller get caller handler name and set into opentracing span
func traceCaller(n int) string {
	pc := make([]uintptr, 1)
	runtime.Callers(n, pc)
	return runtime.FuncForPC(pc[0]).Name()
}
