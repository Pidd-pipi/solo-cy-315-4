package constants

// Unified response codes.
const (
	CodeOK           = 0
	CodeBadRequest   = 40000
	CodeUnauthorized = 40100
	CodeNotFound     = 40400
	CodeConflict     = 40900
	CodeInternal     = 50000
)

// Default messages for unified response codes.
const (
	MsgOK         = "ok"
	MsgBadRequest = "invalid request"
	MsgNotFound   = "resource not found"
	MsgConflict   = "resource conflict"
	MsgInternal   = "internal server error"
)
