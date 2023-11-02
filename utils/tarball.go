package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/codeclysm/extract"
)

func ExtractTarball(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)

	if err != nil {
		return "", fmt.Errorf("error while reading the file %s: %v", filePath, err)
	}

	ctx := context.Background()
	buffer := bytes.NewReader(data)

	filename := filepath.Base(filePath)
	filename = filename[:len(filename)-len(".tar.gz")]

	path := fmt.Sprintf(
		"%s/%s",
		os.Getenv("EXTRACTIONS_PATH"),
		filename,
	)

	return path, extract.Gz(
		ctx,
		buffer,
		path,
		nil,
	)
}
