package articles

import (
	"github.com/gin-gonic/gin"

	"github.com/syst3mctl/godoclive/testdata/gin-realworld/common"
)

// ArticleModelValidator is the article create/update request schema.
type ArticleModelValidator struct {
	Article struct {
		Title       string   `form:"title" json:"title" binding:"required,min=4"`
		Description string   `form:"description" json:"description" binding:"max=2048"`
		Body        string   `form:"body" json:"body" binding:"required,max=2048"`
		Tags        []string `form:"tagList" json:"tagList"`
	} `json:"article"`
	articleModel ArticleModel `json:"-"`
}

// Bind delegates to the shared binder, passing itself as the destination.
func (s *ArticleModelValidator) Bind(c *gin.Context) error {
	if err := common.Bind(c, s); err != nil {
		return err
	}
	s.articleModel.Title = s.Article.Title
	s.articleModel.Body = s.Article.Body
	return nil
}

// NewArticleModelValidator returns an empty article validator.
func NewArticleModelValidator() ArticleModelValidator {
	return ArticleModelValidator{}
}

// NewArticleModelValidatorFillWith seeds a validator from an existing article.
func NewArticleModelValidatorFillWith(articleModel ArticleModel) ArticleModelValidator {
	v := NewArticleModelValidator()
	v.Article.Title = articleModel.Title
	v.Article.Body = articleModel.Body
	return v
}

// CommentModelValidator is the comment create request schema.
type CommentModelValidator struct {
	Comment struct {
		Body string `form:"body" json:"body" binding:"required"`
	} `json:"comment"`
	commentModel CommentModel `json:"-"`
}

// Bind delegates to the shared binder.
func (s *CommentModelValidator) Bind(c *gin.Context) error {
	if err := common.Bind(c, s); err != nil {
		return err
	}
	s.commentModel.Body = s.Comment.Body
	return nil
}

// NewCommentModelValidator returns an empty comment validator.
func NewCommentModelValidator() CommentModelValidator {
	return CommentModelValidator{}
}
