package resp

import "github.com/gin-gonic/gin"

type R struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(200, R{Code: 0, Msg: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus, code int, msg string) {
	c.AbortWithStatusJSON(httpStatus, R{Code: code, Msg: msg})
}

func BadRequest(c *gin.Context, msg string)    { Fail(c, 400, 1002, msg) }
func Unauthorized(c *gin.Context, msg string)  { Fail(c, 401, 1001, msg) }
func Forbidden(c *gin.Context, msg string)     { Fail(c, 403, 1003, msg) }
func NotFound(c *gin.Context, msg string)      { Fail(c, 404, 1004, msg) }
func InternalError(c *gin.Context, msg string) { Fail(c, 500, 5001, msg) }
