package main

import (
	"fmt"

	"Backend/models"
	"Backend/pkg/projects"
	"Backend/utils"
	"github.com/gin-gonic/gin"
	ginlogrus "github.com/toorop/gin-logrus"
)

func main() {

	r := gin.Default()

	r.Use(ginlogrus.Logger(utils.Logger), gin.Recovery())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	projects.SetupRoutes(r)

	err := r.Run(":8080")
	if err != nil {
		_ = fmt.Errorf("error while running the app: %v", err)
	}
}

func init() {
	err := utils.DB.AutoMigrate(
		&models.Project{},
		&models.Function{},
		&models.Database{},
	)
	if err != nil {
		utils.Logger.Fatalf("failed to run the databases migrations: %v", err)
	}
}
