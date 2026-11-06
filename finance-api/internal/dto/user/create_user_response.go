package user_dto

import (
	"finance-api/internal/models"

	"github.com/google/uuid"
)

type CreateUserResponse struct {
	ID    uuid.UUID   `json:"id"`
	Email string      `json:"email"`
	Role  models.Role `json:"role"`
}
