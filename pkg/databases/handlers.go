package databases

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"Backend/models"
	"Backend/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/gin-gonic/gin"
	"github.com/iancoleman/strcase"
	"github.com/lucsky/cuid"
)

func ProvisionDatabase(c *gin.Context) {
	var createDbRequest models.CreateDatabaseDTO

	err := c.ShouldBindJSON(&createDbRequest)
	if utils.HandleError(c, http.StatusBadRequest, err, "cannot parse input") {
		return
	}

	project, _ := utils.GetProjectFromContext(c)

	dbName, username, password, envVars, err := generateEnvVars(createDbRequest)
	if utils.HandleError(c, http.StatusBadRequest, err, "cannot generate env vars") {
		return
	}

	var foundedDbsCount int64

	utils.DB.
		Model(&models.Database{}).
		Where(
			"name = ? AND project_id = ?",
			dbName,
			project.ID.String(),
		).
		Count(&foundedDbsCount)

	if foundedDbsCount > 0 {
		utils.JsonError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("record exists"),
			"database with the same name exists for the project",
		)
		return
	}

	image, err := determineDatabaseImage(createDbRequest)
	if utils.HandleError(c, http.StatusBadRequest, err, "database image determination failed") {
		return
	}

	err = pullDatabaseDockerImage(image)
	if utils.HandleError(c, http.StatusInternalServerError, err, "cannot pull database image") {
		return
	}

	containerId, volumeName, err := createAndStartContainer(
		createDbRequest,
		image,
		envVars,
		project,
	)

	if utils.HandleError(c, http.StatusInternalServerError, err, "cannot start database container") {
		return
	}

	dbEntity := models.Database{
		Name:         dbName,
		Type:         createDbRequest.Type,
		ProjectId:    project.ID,
		ContainerId:  containerId,
		Password:     password,
		Username:     username,
		VolumeName:   volumeName,
		ImageVersion: volumeName,
	}

	err = utils.DB.Create(&dbEntity).Error
	if utils.HandleError(c, http.StatusInternalServerError, err, "failed to create database entity") {
		_, _ = utils.StopAndRemoveContainer(containerId)
		return
	}

	utils.JsonSuccessH(c, http.StatusOK, "database provisioned successfully", gin.H{
		"id":   dbEntity.ID,
		"name": dbEntity.Name,
	})
}

func TearDownDatabase(c *gin.Context) {
	db, err := utils.GetDatabaseFromContextParams(c)
	if err != nil {
		utils.JsonError(
			c,
			http.StatusBadRequest,
			err,
			"cannot find the database",
		)
		return
	}

	// TODO: implement the teardown process

	utils.JsonSuccessH(
		c,
		http.StatusOK,
		"entity found",
		db,
	)
}

func generateEnvVars(dto models.CreateDatabaseDTO) (string, string, string, []string, error) {
	dbName := strcase.ToSnake(dto.Name)

	username, err := utils.GenerateSecureUsername("u", 9)
	if err != nil {
		return "", "", "", nil, err
	}
	username = strings.ToLower(username)

	password, err := utils.GeneratePassword(16)
	if err != nil {
		return "", "", "", nil, err
	}

	var envVars []string

	switch dto.Type {
	case models.Postgres:
		envVars = []string{
			fmt.Sprintf("POSTGRES_DB=%s", dbName),
			fmt.Sprintf("POSTGRES_USER=%s", username),
			fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
		}
	default:
		return "", "", "", nil, fmt.Errorf("unrecognized database type: %s", dto.Type)
	}

	return dbName, username, password, envVars, nil
}

func determineDatabaseImage(dto models.CreateDatabaseDTO) (string, error) {
	version := "latest"

	if dto.Version != "" {
		version = dto.Version
	}

	switch dto.Type {
	case models.Postgres:
		return fmt.Sprintf("postgres:%s", version), nil
	default:
		return "", fmt.Errorf("cannot determine database type %s", dto.Type)
	}
}

func pullDatabaseDockerImage(image string) error {
	ctx := context.Background()
	resp, err := utils.DockerClient.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	defer resp.Close()

	_, err = io.Copy(io.Discard, resp)
	return err
}

func createAndStartContainer(dto models.CreateDatabaseDTO, image string, envVars []string, project *models.Project) (string, string, error) {
	ctx := context.Background()

	networkInspect, err := utils.DockerClient.NetworkInspect(ctx, project.NetworkName, types.NetworkInspectOptions{})
	if err != nil {
		return "", "", err
	}

	containerId := fmt.Sprintf(
		"db_%s",
		cuid.New(),
	)

	volumeName := fmt.Sprintf("vol_%s", containerId)
	vol, err := utils.DockerClient.VolumeCreate(ctx, volume.CreateOptions{
		Driver: "local",
		Name:   volumeName,
	})
	if err != nil {
		return "", "", nil
	}

	var target string
	switch dto.Type {
	case models.Postgres:
		target = "/var/lib/postgresql/data"
	default:
		return "", "", fmt.Errorf("unknown database %s to make volume target path", dto.Type)
	}

	mounts := []mount.Mount{
		{
			Type:     mount.TypeVolume,
			Source:   vol.Name,
			Target:   target,
			ReadOnly: false,
		},
	}

	resp, err := utils.DockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: image,
			Env:   envVars,
		},
		&container.HostConfig{
			Mounts:      mounts,
			NetworkMode: container.NetworkMode(networkInspect.Name),
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
		},
		nil,
		nil,
		containerId,
	)

	if err != nil {
		return "", "", err
	}

	if err = utils.DockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", "", err
	}

	return containerId, volumeName, nil
}
