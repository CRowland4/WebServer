package httpStructs

type LoginRequest struct {
	Password string `json:"password"`
	Email    string `json:"email"`
}

type UsersRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
