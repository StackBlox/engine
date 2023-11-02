package utils

import "github.com/gin-gonic/gin"

func JsonError(ctx *gin.Context, code int, err error, help string) {
	ctx.AbortWithStatusJSON(code, gin.H{
		"error": err.Error(),
		"help":  help,
	})
}

func JsonSuccessH(ctx *gin.Context, code int, message string, data any) {
	ctx.JSON(code, gin.H{
		"message": message,
		"data":    data,
	})
}

// HandleError centralizes the error handling logic.
// It is meant to be used in an if statement to break whenever needed
func HandleError(c *gin.Context, statusCode int, err error, message string) bool {
	if err != nil {
		JsonError(c, statusCode, err, message)
		return true
	}
	return false
}
