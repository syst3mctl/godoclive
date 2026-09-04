// Package main is a fixture for validator-tag constraints and constant
// enumerations. It deliberately depends on nothing: validator rules live in
// struct tags, which are plain strings, so the rules can be stated without
// importing the package that enforces them.
package main

import (
	"encoding/json"
	"net/http"
)

// Status is the publication state of an article.
type Status string

// The states an article moves through, in order.
const (
	StatusDraft     Status = "draft"
	StatusReview    Status = "in_review"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

// Priority ranks an article in the editorial queue.
type Priority int

const (
	PriorityLow Priority = iota + 1
	PriorityNormal
	PriorityHigh
)

// CreateArticleRequest is the body accepted when filing a new article.
type CreateArticleRequest struct {
	// Title is shown in listings.
	Title string `json:"title" validate:"required,min=3,max=120"`
	// Slug is the URL segment for the article.
	Slug string `json:"slug" validate:"required,lowercase,max=64"`
	// AuthorEmail receives editorial notifications.
	AuthorEmail string `json:"author_email" validate:"required,email"`
	// Homepage is the author's own site.
	Homepage string `json:"homepage" validate:"omitempty,url"`
	// Visibility restricts who may read the article.
	Visibility string `json:"visibility" validate:"required,oneof=public unlisted private"`
	// Status is the editorial state; the values come from the Status type.
	Status Status `json:"status"`
	// Priority orders the editorial queue.
	Priority Priority `json:"priority"`
	// WordCount must fall inside the house style guide's range.
	WordCount int `json:"word_count" validate:"gte=300,lte=5000"`
	// Score is strictly between the two bounds.
	Score float64 `json:"score" validate:"gt=0,lt=10"`
	// Tags label the article. At least one, at most five.
	Tags []string `json:"tags" validate:"required,min=1,max=5"`
	// CountryCode is exactly two letters.
	CountryCode string `json:"country_code" validate:"len=2,alpha"`
	// ReviewerID identifies the assigned editor.
	ReviewerID string `json:"reviewer_id" validate:"omitempty,uuid4"`
}

// Article is a stored article.
type Article struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status Status `json:"status"`
}

// CreateArticle files a new article for editorial review.
//
// The body is validated against the house style guide before anything is
// written.
func CreateArticle(w http.ResponseWriter, r *http.Request) {
	var req CreateArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(Article{})
}

// ListArticles returns the articles visible to the caller.
//
// The session cookie decides which drafts are included; without one only
// published articles are returned.
func ListArticles(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie("session_id"); err != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Article{})
		return
	}
	if _, err := r.Cookie("csrf_token"); err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]Article{})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /articles", CreateArticle)
	mux.HandleFunc("GET /articles", ListArticles)
	http.ListenAndServe(":8080", mux)
}
