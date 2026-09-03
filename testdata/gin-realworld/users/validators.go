package users

import (
	"github.com/gin-gonic/gin"

	"github.com/syst3mctl/godoclive/testdata/gin-realworld/common"
)

// UserModelValidator is the registration/update request schema.
type UserModelValidator struct {
	User struct {
		Username string `form:"username" json:"username" binding:"required,min=4,max=255"`
		Email    string `form:"email" json:"email" binding:"required,email"`
		Password string `form:"password" json:"password" binding:"required,min=8,max=255"`
		Bio      string `form:"bio" json:"bio" binding:"max=1024"`
		Image    string `form:"image" json:"image" binding:"omitempty,url"`
	} `json:"user"`
	userModel UserModel `json:"-"`
}

// Bind delegates to the shared binder, passing itself as the destination.
func (s *UserModelValidator) Bind(c *gin.Context) error {
	if err := common.Bind(c, s); err != nil {
		return err
	}
	s.userModel.Username = s.User.Username
	s.userModel.Email = s.User.Email
	return nil
}

// NewUserModelValidator returns an empty validator.
func NewUserModelValidator() UserModelValidator {
	return UserModelValidator{}
}

// LoginValidator is the login request schema.
type LoginValidator struct {
	User struct {
		Email    string `form:"email" json:"email" binding:"required,email"`
		Password string `form:"password" json:"password" binding:"required,min=8,max=255"`
	} `json:"user"`
	userModel UserModel `json:"-"`
}

// Bind delegates to the shared binder.
func (s *LoginValidator) Bind(c *gin.Context) error {
	if err := common.Bind(c, s); err != nil {
		return err
	}
	s.userModel.Email = s.User.Email
	return nil
}

// NewLoginValidator returns an empty login validator.
func NewLoginValidator() LoginValidator {
	return LoginValidator{}
}
