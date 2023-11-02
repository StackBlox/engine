package utils

import (
	"net/http"

	"Backend/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var projectContextName = "project"

func InjectProjectOrFail() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectNameOrId := c.Param("projectNameOrId")

		var project models.Project

		_, uuidErr := uuid.Parse(projectNameOrId)

		var where *gorm.DB
		if uuidErr == nil {
			where = DB.Where(
				"id = ?",
				projectNameOrId,
			)
		} else {
			where = DB.Where(
				"name = ?",
				projectNameOrId,
			)
		}

		err := where.
			Preload("Functions").
			Preload("Databases").
			First(&project).
			Error

		if err != nil {
			JsonError(
				c,
				http.StatusNotFound,
				err,
				"cannot find the project",
			)
			return
		}

		c.Set(projectContextName, &project)
		c.Next()
	}
}
