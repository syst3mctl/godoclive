package articles

import (
	"github.com/gin-gonic/gin"

	"github.com/syst3mctl/godoclive/testdata/gin-realworld/users"
)

// ArticleSerializer renders one article.
type ArticleSerializer struct {
	C *gin.Context
	ArticleModel
}

// ArticleResponse is the article schema.
type ArticleResponse struct {
	Slug        string                `json:"slug"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Body        string                `json:"body"`
	TagList     []string              `json:"tagList"`
	Favorited   bool                  `json:"favorited"`
	Author      users.ProfileResponse `json:"author"`
}

// Response builds the article payload.
func (s *ArticleSerializer) Response() ArticleResponse {
	return ArticleResponse{
		Slug:    s.Slug,
		Title:   s.Title,
		Body:    s.Body,
		TagList: s.Tags,
	}
}

// ArticlesSerializer renders a list of articles.
type ArticlesSerializer struct {
	C        *gin.Context
	Articles []ArticleModel
}

// Response builds the article list payload.
func (s *ArticlesSerializer) Response() []ArticleResponse {
	response := []ArticleResponse{}
	for _, article := range s.Articles {
		serializer := ArticleSerializer{s.C, article}
		response = append(response, serializer.Response())
	}
	return response
}

// CommentSerializer renders one comment.
type CommentSerializer struct {
	C *gin.Context
	CommentModel
}

// CommentResponse is the comment schema.
type CommentResponse struct {
	ID     uint                  `json:"id"`
	Body   string                `json:"body"`
	Author users.ProfileResponse `json:"author"`
}

// Response builds the comment payload.
func (s *CommentSerializer) Response() CommentResponse {
	return CommentResponse{ID: s.ID, Body: s.Body}
}

// CommentsSerializer renders a list of comments.
type CommentsSerializer struct {
	C        *gin.Context
	Comments []CommentModel
}

// Response builds the comment list payload.
func (s *CommentsSerializer) Response() []CommentResponse {
	response := []CommentResponse{}
	for _, comment := range s.Comments {
		serializer := CommentSerializer{s.C, comment}
		response = append(response, serializer.Response())
	}
	return response
}

// TagsSerializer renders the tag list.
type TagsSerializer struct {
	C    *gin.Context
	Tags []string
}

// Response builds the tag list payload.
func (s *TagsSerializer) Response() []string {
	return s.Tags
}
