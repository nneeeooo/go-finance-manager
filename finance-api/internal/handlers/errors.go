package handlers

// ErrorResponse: every error reply, {"error": "<message>"}
type ErrorResponse struct {
	Error string `json:"error" example:"Account not found"`
}
