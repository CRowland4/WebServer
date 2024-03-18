package httpStructs

type LoginResponse struct {
	Email        string `json:"email"`
	ID           int    `json:"id"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

type CreateNewUserResponse struct {
	Email string `json:"email"`
	ID    int    `json:"id"`
}

type UserUpdateResponse struct {
	Email string `json:"email"`
	ID    int    `json:"id"`
}
