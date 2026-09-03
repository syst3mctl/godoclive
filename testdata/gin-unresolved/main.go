// Command gin-unresolved collects the registration shapes whose contract
// cannot be established statically. It exists so the coverage report is tested
// against work it must NOT claim to have completed: an unreachable registration
// helper, an OpenAPI path collision, a binding wrapper that resolves to an
// interface, and a middleware that is a value rather than a function.
package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// deps carries middleware as data, so the chain cannot be resolved back to a
// function body.
type deps struct {
	Auth gin.HandlerFunc
}

var registry = deps{Auth: func(c *gin.Context) { c.Next() }}

func main() {
	r := gin.Default()
	v1 := r.Group("/api")

	// Two paths that differ only in the name of their parameter: OpenAPI
	// treats these as the same path.
	v1.GET("/items/:id", GetItem)
	v1.GET("/items/:itemID", GetItemAlias)

	admin := v1.Group("/admin", registry.Auth)
	admin.GET("/stats", Stats)

	_ = r.Run(":8080")
}

// OrphanRegister is never called, so the prefix and middleware of the group it
// would receive are unknowable.
func OrphanRegister(router *gin.RouterGroup) {
	router.GET("", ListOrphans)
	router.POST("/:id", CreateOrphan)
}

// GetItem returns one item.
func GetItem(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

// GetItemAlias returns one item under the alias parameter name.
func GetItemAlias(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("itemID")})
}

// Stats returns admin statistics.
func Stats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"count": 0})
}

// ListOrphans lists orphaned records.
func ListOrphans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"orphans": []string{}})
}

// CreateOrphan binds a payload whose type is only known at runtime.
func CreateOrphan(c *gin.Context) {
	dst := payloadFor(c.Param("id"))
	if err := bindDynamic(c, dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true})
}

// bindDynamic binds the request body into a destination chosen by the caller.
func bindDynamic(c *gin.Context, dst interface{}) error {
	return c.ShouldBindJSON(dst)
}

// payloadFor picks a payload type at runtime.
func payloadFor(kind string) interface{} {
	if kind == "user" {
		return &struct {
			Name string `json:"name"`
		}{}
	}
	return map[string]interface{}{}
}
