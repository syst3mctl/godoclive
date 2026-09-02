// Package common holds the shared request-binding wrapper and error envelope.
package common

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// CommonError is the RealWorld error envelope: {"errors": {...}}.
type CommonError struct {
	Errors map[string]interface{} `json:"errors"`
}

// NewError wraps a single keyed error.
func NewError(key string, err error) CommonError {
	res := CommonError{Errors: map[string]interface{}{}}
	res.Errors[key] = err.Error()
	return res
}

// NewValidatorError wraps a binding failure.
func NewValidatorError(err error) CommonError {
	res := CommonError{Errors: map[string]interface{}{}}
	res.Errors["body"] = err.Error()
	return res
}

// Bind is the shared binding wrapper every validator delegates to. The
// destination arrives as interface{}, so the request schema is only knowable
// from the caller's argument.
func Bind(c *gin.Context, obj interface{}) error {
	b := binding.Default(c.Request.Method, c.ContentType())
	return c.ShouldBindWith(obj, b)
}
