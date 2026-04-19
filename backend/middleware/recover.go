package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("panic: %v\n%s\n", err, debug.Stack())
				c.AbortWithStatusJSON(500, gin.H{
					"code": 5001,
					"msg":  "internal server error",
				})
			}
		}()
		c.Next()
	}
}
