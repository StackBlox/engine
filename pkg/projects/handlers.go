package projects

import (
	"fmt"
	"net/http"

	"Backend/models"
	"Backend/pkg/databases"
	"Backend/pkg/functions"
	"Backend/pkg/storages"
	"Backend/utils"
	"github.com/docker/docker/api/types"
	"github.com/gin-gonic/gin"
	"github.com/iancoleman/strcase"
)

func SetupRoutes(r *gin.Engine) {
	projectsGroup := r.Group("/projects/")
	{
		projectGroup := projectsGroup.Group(":projectNameOrId")
		{

			projectGroup.Use(utils.InjectProjectOrFail())

			projectGroup.GET("/", GetProject)
			projectGroup.DELETE("/", DeleteProject)
			projectGroup.PUT("/", UpdateProject)

			functionsGroup := projectGroup.Group("functions")
			{
				functionsGroup.POST("/deploy", functions.DeployFunction)
				functionsGroup.POST("/execute/:functionId", functions.ExecuteFunction)
			}

			databasesGroup := projectGroup.Group("/databases")
			{
				databasesGroup.POST("/provision", databases.ProvisionDatabase)

				databaseGroup := databasesGroup.Group(":databaseId")
				{
					databaseGroup.DELETE("/teardown", databases.TearDownDatabase)
				}
			}

			storagesGroup := projectGroup.Group("/storages")
			{
				storagesGroup.POST("/provision", storages.ProvisionStorage)
			}
		}

		projectsGroup.GET("/", GetProjectsList)
		projectsGroup.POST("/", CreateProject)
	}
}

func GetProjectsList(c *gin.Context) {
	var projects []models.Project

	err := utils.DB.Find(&projects).Error
	if err != nil {
		utils.JsonError(
			c,
			http.StatusInternalServerError,
			err,
			"cannot find projects",
		)
		return
	}

	utils.JsonSuccessH(
		c,
		http.StatusOK,
		fmt.Sprintf("%d projects found", len(projects)),
		projects,
	)
}

func CreateProject(c *gin.Context) {
	var projectDto models.CreateProjectDTO

	err := c.ShouldBindJSON(&projectDto)
	if err != nil {
		utils.JsonError(
			c,
			http.StatusBadRequest,
			err,
			"cannot parse request",
		)
		return
	}

	project := models.Project{
		Name:        strcase.ToKebab(projectDto.Name),
		NetworkName: fmt.Sprintf("net_%s", strcase.ToSnake(projectDto.Name)),
	}

	netResp, err := utils.DockerClient.NetworkCreate(c, project.NetworkName, types.NetworkCreate{})
	if err != nil {
		utils.JsonError(
			c,
			http.StatusInternalServerError,
			err,
			"cannot create network for the project",
		)
		return
	}

	err = utils.DB.Create(&project).Error
	if err != nil {
		_ = utils.DockerClient.NetworkRemove(c, netResp.ID)
		utils.JsonError(
			c,
			http.StatusInternalServerError,
			err,
			"cannot create project",
		)
		return
	}

	utils.JsonSuccessH(
		c,
		http.StatusCreated,
		"project created",
		nil,
	)
}

func GetProject(c *gin.Context) {
	p, exists := utils.GetProjectFromContext(c)

	if !exists {
		utils.JsonError(
			c,
			http.StatusNotFound,
			fmt.Errorf("record not found"),
			"project cannot be found",
		)
		return
	}

	utils.JsonSuccessH(
		c,
		http.StatusOK,
		"record found",
		p,
	)
}

func UpdateProject(c *gin.Context) {
	p, exists := utils.GetProjectFromContext(c)

	if !exists {
		utils.JsonError(
			c,
			http.StatusNotFound,
			fmt.Errorf("record not found"),
			"project cannot be found",
		)
		return
	}

	var projectDto models.CreateProjectDTO

	err := c.ShouldBindJSON(&projectDto)
	if err != nil {
		utils.JsonError(
			c,
			http.StatusBadRequest,
			err,
			"cannot parse request",
		)
		return
	}

	p.Name = strcase.ToKebab(projectDto.Name)

	err = utils.DB.Save(&p).Error
	if err != nil {
		utils.JsonError(
			c,
			http.StatusInternalServerError,
			err,
			"cannot update project",
		)
		return
	}

	utils.JsonSuccessH(
		c,
		http.StatusCreated,
		"project updated",
		p,
	)
}

func DeleteProject(c *gin.Context) {
	p, exists := utils.GetProjectFromContext(c)

	if !exists {
		utils.JsonError(
			c,
			http.StatusInternalServerError,
			fmt.Errorf("record not found"),
			"project cannot be found",
		)
		return
	}

	for _, function := range p.Functions {
		_, err := utils.DockerClient.ImageRemove(c, function.FunctionId, types.ImageRemoveOptions{
			Force:         true,
			PruneChildren: true,
		})

		if err != nil {
			utils.JsonError(
				c,
				http.StatusInternalServerError,
				err,
				"cannot remove docker image",
			)
			return
		}
	}

	for _, database := range p.Databases {
		err := utils.DockerClient.ContainerRemove(c, database.ContainerId, types.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})

		if err != nil {
			utils.JsonError(
				c,
				http.StatusInternalServerError,
				err,
				"cannot remove database container",
			)
			return
		}
	}

	err := utils.DockerClient.NetworkRemove(c, p.NetworkName)
	if err != nil {
		utils.JsonError(
			c,
			http.StatusInternalServerError,
			err,
			"cannot remove project network",
		)
		return
	}

	err = utils.DB.Delete(&p).Error
	if err != nil {
		utils.JsonError(
			c,
			http.StatusInternalServerError,
			err,
			"cannot delete project",
		)
		return
	}

	utils.JsonSuccessH(
		c,
		http.StatusOK,
		"project removed",
		nil,
	)

}
