package utils

import (
	"encoding/json"
	"os"
	"path/filepath"

	"Backend/models"
)

const (
	JavaScriptNode models.Language = "node-js"
)

func ReadDefinitionFileFromExtractionPath(extractionPath string) (*models.Definition, models.Language, error) {
	definitionRawData, err := os.ReadFile(filepath.Join(extractionPath, "package.json"))
	if err != nil {
		return nil, "", err
	}

	var def *models.Definition
	if err = json.Unmarshal(definitionRawData, &def); err != nil {
		return nil, "", err
	}

	return def, JavaScriptNode, nil
}
