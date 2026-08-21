package repository

import "github.com/adity1raut/job-scheduler/internal/models"

// UserRepository handles persistence for users.
type UserRepository interface {
	Create(user *models.User) error
	GetByEmail(email string) (*models.User, error)
	GetByID(id string) (*models.User, error)
}
