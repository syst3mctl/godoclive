// Command gin-unresolved collects registration shapes whose contract cannot be
// established statically. It exists so coverage reporting is tested against
// work the analyzer must NOT claim to have completed.
package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	v1 := r.Group("/api")

	v1.GET("/items/:id", GetItem)

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

// ListOrphans lists orphaned records.
func ListOrphans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"orphans": []string{}})
}

// CreateOrphan creates an orphaned record.
func CreateOrphan(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"id": c.Param("id")})
}
