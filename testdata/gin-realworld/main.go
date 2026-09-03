// Command gin-realworld is a compact, dependency-free reduction of the
// gothinkster/golang-gin-realworld-example-app "Conduit" backend. It keeps
// every route-registration shape of the upstream app — cross-package
// registration helpers, group prefixes accumulated through .Use(), a
// bool-gated auth middleware factory, gin's trailing-slash semantics, the
// validator.Bind → common.Bind → c.ShouldBindWith chain, and gin.H responses
// wrapping serializer return types — while dropping the database and JWT
// layers that contribute nothing to static analysis.
package main

import (
	"github.com/gin-gonic/gin"

	"github.com/syst3mctl/godoclive/testdata/gin-realworld/articles"
	"github.com/syst3mctl/godoclive/testdata/gin-realworld/users"
)

func main() {
	r := gin.Default()

	// Trailing slashes are meaningful: "" and "/" register distinct paths.
	r.RedirectTrailingSlash = false

	v1 := r.Group("/api")
	users.UsersRegister(v1.Group("/users"))

	// Optional auth: the token is read when present, never demanded.
	v1.Use(users.AuthMiddleware(false))
	articles.ArticlesAnonymousRegister(v1.Group("/articles"))
	articles.TagsAnonymousRegister(v1.Group("/tags"))
	users.ProfileRetrieveRegister(v1.Group("/profiles"))

	// Required auth: everything registered from here on 401s without a token.
	v1.Use(users.AuthMiddleware(true))
	users.UserRegister(v1.Group("/user"))
	users.ProfileRegister(v1.Group("/profiles"))
	articles.ArticlesRegister(v1.Group("/articles"))

	testAuth := r.Group("/api/ping")
	testAuth.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	_ = r.Run(":8080")
}
