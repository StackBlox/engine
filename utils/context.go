package utils

import (
	"fmt"

	"Backend/models"
	"github.com/gin-gonic/gin"
)

func GetProjectFromContext(ctx *gin.Context) (*models.Project, bool) {
	val, exists := ctx.Get(projectContextName)
	return val.(*models.Project), exists
}

func GetDatabaseFromContextParams(c *gin.Context) (*models.Database, error) {
	project, exists := GetProjectFromContext(c)
	if !exists {
		return nil, fmt.Errorf("project not exists in the context")
	}

	dbId := c.Param("databaseId")

	var dbEntity *models.Database
	err := DB.
		Where("id = ? AND project_id = ?", dbId, project.ID.String()).
		Order("created_at desc").
		Preload("Project").
		First(&dbEntity).
		Error

	return dbEntity, err
}

// GetFunctionFromContextParams retrieves the function entity from the database based on the given function tag.
func GetFunctionFromContextParams(c *gin.Context) (*models.Function, error) {
	project, exists := GetProjectFromContext(c)
	if !exists {
		return nil, fmt.Errorf("project not exists in the context")
	}

	funcId := c.Param("functionId")

	var functionEntity *models.Function
	err := DB.
		Where("function_id = ? AND project_id = ?", funcId, project.ID.String()).
		Order("created_at desc").
		Preload("Project").
		First(&functionEntity).
		Error

	return functionEntity, err
}
