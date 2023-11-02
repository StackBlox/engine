package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DbType string

const (
	Postgres DbType = "postgres"
)

type Database struct {
	ID           uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4()" json:"id"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"createdAt" containerEnv:"include"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updatedAt" containerEnv:"include"`
	DeletedAt    gorm.DeletedAt `json:"deletedAt"`
	Name         string         `gorm:"not null;type:varchar(255);primaryKey;uniqueIndex:project_db_name_index" json:"name" containerEnv:"include"`
	Type         DbType         `gorm:"not null;type:varchar(255)" json:"type" containerEnv:"include"`
	Username     string         `gorm:"not null;type:varchar(255)" json:"username" containerEnv:"include"`
	Password     string         `gorm:"not null;type:varchar(255)" json:"password" containerEnv:"include"`
	ContainerId  string         `gorm:"not null;type:varchar(255)" json:"containerId" containerEnv:"include;containerId"`
	ProjectId    uuid.UUID      `gorm:"not null;type:varchar(255);primaryKey;foreignKey:ID;uniqueIndex:project_db_name_index" json:"projectId"`
	VolumeName   string         `gorm:"not null;type:varchar(255);" json:"volumeName"`
	ImageVersion string         `gorm:"type:varchar(255);" json:"imageVersion"`

	Project Project
}

type CreateDatabaseDTO struct {
	Type    DbType `json:"type" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Version string `json:"version" binding:"required"`
}
