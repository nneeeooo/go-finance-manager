package user_dto

import "github.com/google/uuid"

type CreateUserResponse struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}
