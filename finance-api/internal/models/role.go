package models

// Role: what a user may do. Regular users see and change only their own data; admins also manage who may register.
type Role string

const (
	RoleRegular Role = "regular"
	RoleAdmin   Role = "admin"
)

func (r Role) Valid() bool { return r == RoleRegular || r == RoleAdmin }
