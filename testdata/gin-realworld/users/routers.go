package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/syst3mctl/godoclive/testdata/gin-realworld/common"
)

// UsersRegister mounts the anonymous user routes on the group it is handed.
func UsersRegister(router *gin.RouterGroup) {
	router.POST("", UsersRegistration)
	router.POST("/", UsersRegistration)
	router.POST("/login", UsersLogin)
}

// UserRegister mounts the authenticated current-user routes.
func UserRegister(router *gin.RouterGroup) {
	router.GET("", UserRetrieve)
	router.GET("/", UserRetrieve)
	router.PUT("", UserUpdate)
	router.PUT("/", UserUpdate)
}

// ProfileRetrieveRegister mounts the publicly readable profile route.
func ProfileRetrieveRegister(router *gin.RouterGroup) {
	router.GET("/:username", ProfileRetrieve)
}

// ProfileRegister mounts the follow/unfollow routes.
func ProfileRegister(router *gin.RouterGroup) {
	router.POST("/:username/follow", ProfileFollow)
	router.DELETE("/:username/follow", ProfileUnfollow)
}

// UsersRegistration creates a user.
func UsersRegistration(c *gin.Context) {
	userModelValidator := NewUserModelValidator()
	if err := userModelValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}
	serializer := UserSerializer{c}
	c.JSON(http.StatusCreated, gin.H{"user": serializer.Response()})
}

// UsersLogin authenticates a user.
func UsersLogin(c *gin.Context) {
	loginValidator := NewLoginValidator()
	if err := loginValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}
	serializer := UserSerializer{c}
	c.JSON(http.StatusOK, gin.H{"user": serializer.Response()})
}

// UserRetrieve returns the authenticated user.
func UserRetrieve(c *gin.Context) {
	serializer := UserSerializer{c}
	c.JSON(http.StatusOK, gin.H{"user": serializer.Response()})
}

// UserUpdate updates the authenticated user.
func UserUpdate(c *gin.Context) {
	userModelValidator := NewUserModelValidator()
	if err := userModelValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}
	serializer := UserSerializer{c}
	c.JSON(http.StatusOK, gin.H{"user": serializer.Response()})
}

// ProfileRetrieve returns one public profile.
func ProfileRetrieve(c *gin.Context) {
	username := c.Param("username")
	userModel, err := FindOneUser(username)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("profile", errors.New("invalid username")))
		return
	}
	serializer := ProfileSerializer{c, userModel}
	c.JSON(http.StatusOK, gin.H{"profile": serializer.Response()})
}

// ProfileFollow follows a profile.
func ProfileFollow(c *gin.Context) {
	username := c.Param("username")
	userModel, err := FindOneUser(username)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("profile", errors.New("invalid username")))
		return
	}
	serializer := ProfileSerializer{c, userModel}
	c.JSON(http.StatusOK, gin.H{"profile": serializer.Response()})
}

// ProfileUnfollow unfollows a profile.
func ProfileUnfollow(c *gin.Context) {
	username := c.Param("username")
	userModel, err := FindOneUser(username)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("profile", errors.New("invalid username")))
		return
	}
	serializer := ProfileSerializer{c, userModel}
	c.JSON(http.StatusOK, gin.H{"profile": serializer.Response()})
}
