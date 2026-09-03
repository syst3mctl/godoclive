package users

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// extractToken pulls the token out of the Authorization header, falling back
// to a query parameter. The auth scheme is only visible one call level below
// the middleware itself.
func extractToken(c *gin.Context) string {
	bearer := c.GetHeader("Authorization")
	if len(bearer) > 6 && strings.ToUpper(bearer[0:6]) == "TOKEN " {
		return bearer[6:]
	}
	return c.Query("access_token")
}

// AuthMiddleware is a factory whose bool argument decides whether a missing or
// invalid token aborts the request. AuthMiddleware(true) makes auth required;
// AuthMiddleware(false) makes it optional.
func AuthMiddleware(auto401 bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			if auto401 {
				c.AbortWithStatus(http.StatusUnauthorized)
			}
			return
		}
		c.Set("my_user_model", UserModel{Username: tokenString})
	}
}
