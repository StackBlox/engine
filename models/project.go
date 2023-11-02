package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Project struct {
	ID          uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4()" json:"id"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"deletedAt"`
	Name        string         `gorm:"not null;unique" json:"name"`
	NetworkName string         `gorm:"not null;unique" json:"networkName"`
	Functions   []Function     `gorm:"foreignKey:ProjectId;references:ID" json:"functions"`
	Databases   []Database     `gorm:"foreignKey:ProjectId;references:ID" json:"databases"`
}

type CreateProjectDTO struct {
	Name string `json:"name" binding:"required"`
}
