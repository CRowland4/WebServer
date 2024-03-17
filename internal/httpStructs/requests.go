package httpStructs

type LoginRequest struct {
	Password         string `json:"password"`
	Email            string `json:"email"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type PutUsersRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
