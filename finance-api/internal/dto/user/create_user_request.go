package user_dto

type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email,max=320"`
	Password string `json:"password" validate:"required,min=8,max=128"`
	Role     string `json:"role" validate:"omitempty,oneof=regular admin"`
}

// UpdateUserRequest: a new password. Changing your own needs the current one; an admin resetting another user's does not.
type UpdateUserRequest struct {
	Password        string `json:"password" validate:"required,min=8,max=128"`
	CurrentPassword string `json:"current_password"`
}
