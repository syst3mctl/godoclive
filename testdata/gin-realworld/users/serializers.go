package users

import "github.com/gin-gonic/gin"

// ProfileSerializer renders a UserModel as a public profile.
type ProfileSerializer struct {
	C *gin.Context
	UserModel
}

// ProfileResponse is the public profile schema.
type ProfileResponse struct {
	Username  string `json:"username"`
	Bio       string `json:"bio"`
	Image     string `json:"image"`
	Following bool   `json:"following"`
}

// Response builds the profile payload.
func (s *ProfileSerializer) Response() ProfileResponse {
	return ProfileResponse{
		Username: s.Username,
		Bio:      s.Bio,
		Image:    s.Image,
	}
}

// UserSerializer renders the authenticated user.
type UserSerializer struct {
	C *gin.Context
}

// UserResponse is the authenticated-user schema.
type UserResponse struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Bio      string `json:"bio"`
	Image    string `json:"image"`
	Token    string `json:"token"`
}

// Response builds the authenticated-user payload.
func (s *UserSerializer) Response() UserResponse {
	return UserResponse{Username: "user", Email: "user@example.com", Token: "jwt"}
}
