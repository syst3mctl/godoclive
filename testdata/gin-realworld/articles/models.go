package articles

import "github.com/syst3mctl/godoclive/testdata/gin-realworld/users"

// ArticleModel is the persisted article record.
type ArticleModel struct {
	ID          uint
	Slug        string
	Title       string
	Description string
	Body        string
	Author      users.UserModel
	Tags        []string
}

// CommentModel is the persisted comment record.
type CommentModel struct {
	ID     uint
	Body   string
	Author users.UserModel
}

// FindOneArticle stands in for the database lookup.
func FindOneArticle(slug string) (ArticleModel, error) {
	return ArticleModel{Slug: slug}, nil
}
