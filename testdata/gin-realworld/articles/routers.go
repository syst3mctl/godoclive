package articles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/syst3mctl/godoclive/testdata/gin-realworld/common"
)

// ArticlesRegister mounts the authenticated article routes.
func ArticlesRegister(router *gin.RouterGroup) {
	router.GET("/feed", ArticleFeed)
	router.POST("", ArticleCreate)
	router.POST("/", ArticleCreate)
	router.PUT("/:slug", ArticleUpdate)
	router.PUT("/:slug/", ArticleUpdate)
	router.DELETE("/:slug", ArticleDelete)
	router.POST("/:slug/favorite", ArticleFavorite)
	router.DELETE("/:slug/favorite", ArticleUnfavorite)
	router.POST("/:slug/comments", ArticleCommentCreate)
	router.DELETE("/:slug/comments/:id", ArticleCommentDelete)
}

// ArticlesAnonymousRegister mounts the publicly readable article routes.
func ArticlesAnonymousRegister(router *gin.RouterGroup) {
	router.GET("", ArticleList)
	router.GET("/", ArticleList)
	router.GET("/:slug", ArticleRetrieve)
	router.GET("/:slug/comments", ArticleCommentList)
}

// TagsAnonymousRegister mounts the tag listing route.
func TagsAnonymousRegister(router *gin.RouterGroup) {
	router.GET("", TagList)
	router.GET("/", TagList)
}

// ArticleCreate creates an article.
func ArticleCreate(c *gin.Context) {
	articleModelValidator := NewArticleModelValidator()
	if err := articleModelValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}
	serializer := ArticleSerializer{c, articleModelValidator.articleModel}
	c.JSON(http.StatusCreated, gin.H{"article": serializer.Response()})
}

// ArticleList lists articles.
func ArticleList(c *gin.Context) {
	tag := c.Query("tag")
	author := c.Query("author")
	limit := c.DefaultQuery("limit", "20")
	offset := c.DefaultQuery("offset", "0")
	_, _, _, _ = tag, author, limit, offset

	serializer := ArticlesSerializer{c, []ArticleModel{}}
	c.JSON(http.StatusOK, gin.H{"articles": serializer.Response(), "articlesCount": 0})
}

// ArticleFeed lists articles from followed authors.
func ArticleFeed(c *gin.Context) {
	limit := c.DefaultQuery("limit", "20")
	offset := c.DefaultQuery("offset", "0")
	_, _ = limit, offset

	serializer := ArticlesSerializer{c, []ArticleModel{}}
	c.JSON(http.StatusOK, gin.H{"articles": serializer.Response(), "articlesCount": 0})
}

// ArticleRetrieve returns one article.
func ArticleRetrieve(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("articles", errors.New("invalid slug")))
		return
	}
	serializer := ArticleSerializer{c, articleModel}
	c.JSON(http.StatusOK, gin.H{"article": serializer.Response()})
}

// ArticleUpdate updates an article.
func ArticleUpdate(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("articles", errors.New("invalid slug")))
		return
	}
	articleModelValidator := NewArticleModelValidatorFillWith(articleModel)
	if err := articleModelValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}
	serializer := ArticleSerializer{c, articleModel}
	c.JSON(http.StatusOK, gin.H{"article": serializer.Response()})
}

// ArticleDelete deletes an article.
func ArticleDelete(c *gin.Context) {
	slug := c.Param("slug")
	if _, err := FindOneArticle(slug); err != nil {
		c.JSON(http.StatusNotFound, common.NewError("articles", errors.New("invalid slug")))
		return
	}
	c.JSON(http.StatusOK, gin.H{"article": "Delete success"})
}

// ArticleFavorite favorites an article.
func ArticleFavorite(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("articles", errors.New("invalid slug")))
		return
	}
	serializer := ArticleSerializer{c, articleModel}
	c.JSON(http.StatusOK, gin.H{"article": serializer.Response()})
}

// ArticleUnfavorite unfavorites an article.
func ArticleUnfavorite(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("articles", errors.New("invalid slug")))
		return
	}
	serializer := ArticleSerializer{c, articleModel}
	c.JSON(http.StatusOK, gin.H{"article": serializer.Response()})
}

// ArticleCommentCreate adds a comment to an article.
func ArticleCommentCreate(c *gin.Context) {
	slug := c.Param("slug")
	if _, err := FindOneArticle(slug); err != nil {
		c.JSON(http.StatusNotFound, common.NewError("comment", errors.New("invalid slug")))
		return
	}
	commentModelValidator := NewCommentModelValidator()
	if err := commentModelValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}
	serializer := CommentSerializer{c, commentModelValidator.commentModel}
	c.JSON(http.StatusCreated, gin.H{"comment": serializer.Response()})
}

// ArticleCommentDelete removes a comment.
func ArticleCommentDelete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusNotFound, common.NewError("comment", errors.New("invalid id")))
		return
	}
	c.JSON(http.StatusOK, gin.H{"comment": "Delete success"})
}

// ArticleCommentList lists an article's comments.
func ArticleCommentList(c *gin.Context) {
	slug := c.Param("slug")
	if _, err := FindOneArticle(slug); err != nil {
		c.JSON(http.StatusNotFound, common.NewError("comments", errors.New("invalid slug")))
		return
	}
	serializer := CommentsSerializer{c, []CommentModel{}}
	c.JSON(http.StatusOK, gin.H{"comments": serializer.Response()})
}

// TagList lists all tags.
func TagList(c *gin.Context) {
	serializer := TagsSerializer{c, []string{}}
	c.JSON(http.StatusOK, gin.H{"tags": serializer.Response()})
}
