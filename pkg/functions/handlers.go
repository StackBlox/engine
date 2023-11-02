package functions

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"Backend/models"
	"Backend/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iancoleman/strcase"
	"github.com/lucsky/cuid"
	"gorm.io/gorm/clause"
)

// DeployFunction manages the deployment process for a new function.
// It handles the file upload, saves the function code, extracts the tarball,
// reads function metadata, generates and writes the Dockerfile and entrypoint,
// builds the Docker image, and saves the function metadata to the database.
// On successful deployment, it sends a success response with the function metadata.
func DeployFunction(c *gin.Context) {
	file, err := c.FormFile("function")
	if utils.HandleError(c, http.StatusBadRequest, err, "function upload failed") {
		return
	}

	uploadedTarballPath, err := saveUploadedFile(c, file)
	if utils.HandleError(c, http.StatusBadRequest, err, "unable to save the uploaded file") {
		return
	}

	functionExtractionPath, err := utils.ExtractTarball(uploadedTarballPath)
	if utils.HandleError(c, http.StatusBadRequest, err, "unable to extract the function code") {
		cleanUpDeploy(functionExtractionPath, uploadedTarballPath)
		return
	}

	functionMetadata, err := readFunctionMetadata(functionExtractionPath)
	if utils.HandleError(c, http.StatusBadRequest, err, "failed to read the definition file") {
		cleanUpDeploy(functionExtractionPath, uploadedTarballPath)
		return
	}

	err = fillFunctionMetadata(c, &functionMetadata)
	if utils.HandleError(c, http.StatusInternalServerError, err, "failed to fill function metadata") {
		cleanUpDeploy(functionExtractionPath, uploadedTarballPath)
		return
	}

	err = generateAndWriteDockerfile(functionExtractionPath, functionMetadata)
	if utils.HandleError(c, http.StatusBadRequest, err, "failed to generate/write the dockerfile") {
		cleanUpDeploy(functionExtractionPath, uploadedTarballPath)
		return
	}

	err = generateAndWriteEntrypoint(functionExtractionPath, functionMetadata)
	if utils.HandleError(c, http.StatusBadRequest, err, "failed to generate/write the entrypoint") {
		cleanUpDeploy(functionExtractionPath, uploadedTarballPath)
		return
	}

	err = buildAndSaveDockerImage(c, functionExtractionPath, &functionMetadata)
	if utils.HandleError(c, http.StatusBadRequest, err, "failed to build the image") {
		cleanUpDeploy(functionExtractionPath, uploadedTarballPath)
		return
	}

	cleanUpDeploy(functionExtractionPath, uploadedTarballPath)

	utils.JsonSuccessH(
		c,
		http.StatusOK,
		"function deployed successfully",
		gin.H{
			"metadata": functionMetadata,
		},
	)
}

// ExecuteFunction handles the execution of a specified function.
// It retrieves the function entity, creates and starts a container for the function,
// polls the health check of the container, and forwards the request to the container.
func ExecuteFunction(c *gin.Context) {
	functionEntity, err := utils.GetFunctionFromContextParams(c)
	if utils.HandleError(c, http.StatusBadRequest, err, "failed to find function") {
		return
	}

	imageVersion := getImageVersion(c)

	containerID, dynamicPort, err := createAndStartContainer(c, functionEntity, imageVersion)
	if utils.HandleError(c, http.StatusInternalServerError, err, "failed to create/start container") {
		return
	}

	isHealthy := pollContainerHealthCheck(dynamicPort)
	if !isHealthy {
		utils.JsonError(
			c,
			http.StatusInternalServerError,
			nil,
			"container did not become healthy",
		)
		return
	}

	jsonResponse, containerResp, err := forwardRequestToContainer(c, dynamicPort)
	if utils.HandleError(c, http.StatusInternalServerError, err, "failed to forward request to container") {
		return
	}

	cleanupContainer(containerID)

	sendContainerResponse(c, containerResp, jsonResponse)
}

// cleanUpDeploy removes the generated files and the uploaded tarball
func cleanUpDeploy(extractionPath string, tarballPath string) {
	err := os.RemoveAll(extractionPath)
	if err != nil {
		utils.Logger.Warnf("cannot remove extraction path %s", extractionPath)
	}
	err = os.Remove(tarballPath)
	if err != nil {
		utils.Logger.Warnf("cannot remove tarball %s", tarballPath)
	}
}

// saveUploadedFile handles the saving of uploaded files and returns the functionID and path.
func saveUploadedFile(c *gin.Context, file *multipart.FileHeader) (string, error) {
	functionID := uuid.NewString()
	uploadedTarballPath := fmt.Sprintf(
		"%s/%s.tar.gz",
		os.Getenv("UPLOADS_PATH"),
		functionID,
	)
	err := c.SaveUploadedFile(file, uploadedTarballPath)
	return uploadedTarballPath, err
}

// readFunctionMetadata reads and returns the function metadata.
func readFunctionMetadata(functionExtractionPath string) (models.Function, error) {
	def, lang, err := utils.ReadDefinitionFileFromExtractionPath(functionExtractionPath)
	if err != nil {
		return models.Function{}, err
	}
	return models.Function{
		Name:        def.Name,
		Description: def.Description,
		FunctionId:  strcase.ToKebab(def.Name),
		Version:     def.Version,
		Language:    lang,
		Main:        def.Main,
		//... other metadata ...
	}, nil
}

// generateAndWriteDockerfile generates and writes the Dockerfile.
func generateAndWriteDockerfile(functionExtractionPath string, functionMetadata models.Function) error {
	dockerfileContent, err := utils.GenerateDockerfileContent(functionMetadata)
	if err != nil {
		return err
	}
	dockerfilePath := filepath.Join(functionExtractionPath, "Dockerfile")
	return os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644)
}

// generateAndWriteEntrypoint generates and writes the entrypoint.
func generateAndWriteEntrypoint(functionExtractionPath string, functionMetadata models.Function) error {
	entrypointContent, entrypointName, err := utils.GenerateEntrypointContent(functionMetadata)
	if err != nil {
		return err
	}
	entrypointPath := filepath.Join(functionExtractionPath, entrypointName)
	return os.WriteFile(entrypointPath, []byte(entrypointContent), 0644)
}

// buildAndSaveDockerImage builds and saves the Docker image.
func buildAndSaveDockerImage(c *gin.Context, functionExtractionPath string, functionMetadata *models.Function) error {
	tar, err := archive.TarWithOptions(functionExtractionPath, &archive.TarOptions{})
	if err != nil {
		return err
	}
	buildOpts := getDockerBuildOptions(functionMetadata)
	buildResponse, err := utils.DockerClient.ImageBuild(c, tar, buildOpts)
	if err != nil {
		return err
	}
	defer buildResponse.Body.Close()
	return saveMetadataToDB(functionMetadata)
}

// fillFunctionMetadata fills the extra metadata required for the function entity
func fillFunctionMetadata(c *gin.Context, model *models.Function) error {
	p, exists := utils.GetProjectFromContext(c)

	if !exists {
		return fmt.Errorf("project cannot be found from context")
	}

	model.ProjectId = p.ID

	return nil
}

// getDockerBuildOptions creates and returns Docker build options.
func getDockerBuildOptions(functionMetadata *models.Function) types.ImageBuildOptions {
	return types.ImageBuildOptions{
		SuppressOutput: true,
		Remove:         true,
		ForceRemove:    true,
		Tags: []string{
			fmt.Sprintf("%s:%s", functionMetadata.FunctionId, functionMetadata.Version),
			fmt.Sprintf("%s:latest", functionMetadata.FunctionId),
		},
	}
}

// saveMetadataToDB saves the function metadata to the database.
func saveMetadataToDB(functionMetadata *models.Function) error {
	err := utils.DB.
		Clauses(
			clause.OnConflict{
				Columns: []clause.Column{
					{Name: "function_id"},
					{Name: "version"},
					{Name: "project_id"},
				},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"updated_at": "NOW()",
				}),
			},
		).
		Create(&functionMetadata).
		Error

	if err != nil {
		return err
	}

	return utils.DB.First(&functionMetadata).Error
}

// getImageVersion extracts the image version from the request header.
// If not provided, defaults to "latest".
func getImageVersion(c *gin.Context) string {
	imageVersion := c.Request.Header.Get("x-image-version")
	if imageVersion == "" {
		imageVersion = "latest"
	}
	return imageVersion
}

// createAndStartContainer creates and starts a Docker container for the given function entity.
// It returns the container ID and the dynamic port on which the container is exposed.
func createAndStartContainer(c *gin.Context, functionEntity *models.Function, imageVersion string) (string, string, error) {
	var envs []string

	// Fetch the project from the context.
	project, _ := utils.GetProjectFromContext(c)

	// Generate environment variables for functions in the project.
	envs = generateEnvironmentVariables(project.Functions, "FUNC")

	// Generate environment variables for databases in the project.
	envs = append(envs, generateEnvironmentVariables(project.Databases, "DB")...)

	// Create the Docker container with the specified configurations.
	containerID, err := createContainer(c, functionEntity, imageVersion, envs)
	if err != nil {
		return "", "", err
	}

	// Start the Docker container.
	if err := utils.DockerClient.ContainerStart(c, containerID, types.ContainerStartOptions{}); err != nil {
		return "", "", err
	}

	// Fetch the dynamic port on which the container is exposed.
	dynamicPort, err := fetchContainerDynamicPort(c, containerID)
	if err != nil {
		return "", "", err
	}

	return containerID, dynamicPort, nil
}

// generateEnvironmentVariables creates environment variables for given entities using the specified prefix.
func generateEnvironmentVariables(entities interface{}, prefix string) []string {
	var envs []string

	slice := reflect.ValueOf(entities)
	if slice.Kind() != reflect.Slice {
		return nil // or panic, based on how you want to handle this situation
	}

	for i := 0; i < slice.Len(); i++ {
		entity := slice.Index(i).Interface()
		idField := reflect.ValueOf(entity).FieldByName("Name")
		if !idField.IsValid() {
			continue // If the "Name" field doesn't exist, skip to the next entity
		}

		id := strcase.ToScreamingSnake(idField.String())

		v := reflect.ValueOf(entity)
		t := v.Type()

		for j := 0; j < v.NumField(); j++ {
			field := t.Field(j)

			tags, ok := field.Tag.Lookup("containerEnv")
			if !ok {
				continue
			}

			tagsSeparated := strings.Split(tags, ";")

			if slices.Contains(tagsSeparated, "include") {
				fieldName := t.Field(j).Name

				envKey := fmt.Sprintf("%s_%s_%s", prefix, id, strcase.ToScreamingSnake(fieldName))

				var fieldValue any

				// this is meant to be done sometime later. the question is
				// do we want to inspect the container and give the user
				// the id of the container? although we already know the id of it?
				if slices.Contains(tagsSeparated, "containerId") {
					fieldValue = v.Field(j).Interface()
				} else {
					fieldValue = v.Field(j).Interface()
				}

				envs = append(envs, fmt.Sprintf("%s=%v", envKey, fieldValue))
			}
		}
	}

	return envs
}

// createContainer initializes a new Docker container with the provided configurations.
func createContainer(c *gin.Context, functionEntity *models.Function, imageVersion string, envs []string) (string, error) {
	resp, err := utils.DockerClient.ContainerCreate(
		c,
		&container.Config{
			Image: fmt.Sprintf("%s:%s", functionEntity.FunctionId, imageVersion),
			ExposedPorts: nat.PortSet{
				"8080": {},
			},
			Env: envs,
		},
		&container.HostConfig{
			PortBindings: nat.PortMap{
				"8080": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: ""}},
			},
			NetworkMode: container.NetworkMode(functionEntity.Project.NetworkName),
		},
		nil,
		nil,
		fmt.Sprintf(
			"func_%s",
			cuid.New(),
		),
	)
	return resp.ID, err
}

// fetchContainerDynamicPort retrieves the dynamic port of the specified Docker container.
func fetchContainerDynamicPort(c *gin.Context, containerID string) (string, error) {
	inspect, err := utils.DockerClient.ContainerInspect(c, containerID)
	if err != nil {
		return "", err
	}
	return inspect.NetworkSettings.Ports["8080/tcp"][0].HostPort, nil
}

// pollContainerHealthCheck checks the health status of the container by polling its health check endpoint.
// It returns true if the container is healthy, otherwise false.
func pollContainerHealthCheck(dynamicPort string) bool {
	retryIntervals := []time.Duration{
		250 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		3 * time.Second,
	}
	healthCheckURL := fmt.Sprintf("http://localhost:%s/health", dynamicPort)

	client := &http.Client{}
	for _, interval := range retryIntervals {
		time.Sleep(interval) // Wait for the specified interval before checking health.
		resp, err := client.Get(healthCheckURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true
		}
	}
	return false
}

// forwardRequestToContainer sends the client's request to the container and retrieves its response.
// It returns the JSON response, the HTTP response, and any error that occurs.
func forwardRequestToContainer(c *gin.Context, dynamicPort string) (interface{}, *http.Response, error) {
	containerURL := fmt.Sprintf("http://localhost:%s", dynamicPort)
	containerReq, err := http.NewRequest(c.Request.Method, containerURL, c.Request.Body)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range c.Request.Header {
		containerReq.Header.Set(k, strings.Join(v, ","))
	}

	client := &http.Client{}
	containerResp, err := client.Do(containerReq)
	if err != nil {
		return nil, nil, err
	}

	containerResponseBody, err := io.ReadAll(containerResp.Body)
	if err != nil {
		return nil, nil, err
	}
	containerResp.Body.Close()

	var jsonResponse interface{}
	if len(containerResponseBody) > 0 {
		err = json.Unmarshal(containerResponseBody, &jsonResponse)
		if err != nil {
			utils.Logger.Warnf("failed to unmarshal container response: %s", err)
			jsonResponse = string(containerResponseBody)
		}
	} else {
		jsonResponse = ""
	}

	return jsonResponse, containerResp, nil
}

// cleanupContainer stops and removes the specified Docker container.
// This is done asynchronously to not delay the response.
func cleanupContainer(containerID string) {
	go func() {
		errMessage, err := utils.StopAndRemoveContainer(containerID)
		if err != nil {
			utils.Logger.Errorf("%s: %v", errMessage, err)
		}
	}()
}

// sendContainerResponse sends the container's response back to the client.
// It sets the necessary headers and JSON response.
func sendContainerResponse(c *gin.Context, containerResp *http.Response, jsonResponse interface{}) {
	for k, v := range containerResp.Header {
		c.Header(k, strings.Join(v, ","))
	}

	c.JSON(
		containerResp.StatusCode,
		jsonResponse,
	)
}
