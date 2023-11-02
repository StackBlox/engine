package utils

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"path/filepath"

	"Backend/models"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

var DockerClient *client.Client

func initDockerClient() {
	var err error
	DockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		Logger.Fatalf("failed to setup docker client: %v", err)
	}
}

func GenerateDockerfileContent(metadata models.Function) (string, error) {
	var baseImage, command, entrypoint string
	var runs []string

	switch metadata.Language {
	case JavaScriptNode:
		baseImage = "node:18"
		command = "node"
		entrypoint = "entrypoint.js"
		runs = []string{"npm install"}

	default:
		return "", fmt.Errorf("unsupported language: %s", metadata.Language)
	}

	data := struct {
		BaseImage  string
		Command    string
		Entrypoint string
		Runs       []string
	}{
		BaseImage:  baseImage,
		Command:    command,
		Entrypoint: entrypoint,
		Runs:       runs,
	}

	var tpl bytes.Buffer

	tmpl, err := template.New("base.tmpl").ParseFiles("templates/dockerfiles/base.tmpl")
	if err != nil {
		return "", err
	}

	err = tmpl.Execute(&tpl, data)
	if err != nil {
		return "", err
	}

	return tpl.String(), nil
}

func GenerateEntrypointContent(metadata models.Function) (string, string, error) {
	var tmplName string

	switch metadata.Language {
	case JavaScriptNode:
		tmplName = "entrypoint.js"
	default:
		return "", "", fmt.Errorf("unsupported language: %s", metadata.Language)
	}

	var tpl bytes.Buffer
	tmplFilename := fmt.Sprintf("%s.tmpl", tmplName)

	tmpl, err := template.
		New(tmplFilename).
		ParseFiles(filepath.Join("templates/entrypoints", tmplFilename))
	if err != nil {
		return "", "", err
	}

	data := struct {
		Main string
	}{
		Main: metadata.Main,
	}

	err = tmpl.Execute(&tpl, data)
	if err != nil {
		return "", "", err
	}

	return tpl.String(), tmplName, nil
}

func StopAndRemoveContainer(containerId string) (string, error) {
	if err := DockerClient.ContainerStop(
		context.Background(),
		containerId,
		container.StopOptions{},
	); err != nil {
		return "failed to stop the container", err
	}

	if err := DockerClient.
		ContainerRemove(
			context.Background(),
			containerId,
			types.ContainerRemoveOptions{
				RemoveVolumes: true,
			},
		); err != nil {
		return "failed to remove the container", err
	}

	return "", nil
}
