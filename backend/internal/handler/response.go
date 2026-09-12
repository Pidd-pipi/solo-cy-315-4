package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gbschedule/gbschedule/internal/constants"
	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/service"
)

// OK writes a unified success response.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, dto.Response{Code: constants.CodeOK, Message: constants.MsgOK, Data: data})
}

// Created writes a unified created response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, dto.Response{Code: constants.CodeOK, Message: constants.MsgOK, Data: data})
}

// Error converts a service error to a unified JSON response.
func Error(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, dto.Response{Code: constants.CodeNotFound, Message: constants.MsgNotFound, Data: nil})
	case errors.Is(err, service.ErrInvalid):
		c.JSON(http.StatusBadRequest, dto.Response{Code: constants.CodeBadRequest, Message: constants.MsgBadRequest, Data: nil})
	case errors.Is(err, service.ErrConflict):
		c.JSON(http.StatusConflict, dto.Response{Code: constants.CodeConflict, Message: constants.MsgConflict, Data: nil})
	default:
		c.JSON(http.StatusInternalServerError, dto.Response{Code: constants.CodeInternal, Message: constants.MsgInternal, Data: nil})
	}
}

// BadRequest writes a 400 validation-style response.
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, dto.Response{Code: constants.CodeBadRequest, Message: message, Data: nil})
}

// parseID parses a uint path parameter.
func parseID(c *gin.Context, key string) (uint, bool) {
	raw := c.Param(key)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		BadRequest(c, "invalid path parameter "+key)
		return 0, false
	}
	return uint(id), true
}
