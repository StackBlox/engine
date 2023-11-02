package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Language string

type Function struct {
	ID          uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4()" json:"id"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"createdAt" containerEnv:"include"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updatedAt" containerEnv:"include"`
	DeletedAt   gorm.DeletedAt `json:"deletedAt"`
	Name        string         `gorm:"not null" json:"name" containerEnv:"include"`
	Description string         `gorm:"type:varchar(255)" json:"description" containerEnv:"include"`
	FunctionId  string         `gorm:"not null;type:varchar(255);primaryKey" json:"functionId" containerEnv:"include"`
	Version     string         `gorm:"not null;type:varchar(255);primaryKey" json:"version" containerEnv:"include"`
	ProjectId   uuid.UUID      `gorm:"not null;type:varchar(255);primaryKey;foreignKey:ID" json:"projectId"`

	Project  Project
	Language Language
	Main     string
}

type Definition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Main        string `json:"main"`
	//... other fields as needed ...
}
