package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
)

var ErrInvalidPost = errors.New("title and content are required")
var ErrInvalidStatus = errors.New("status must be draft or published")
var slugPattern = regexp.MustCompile(`[^\pL\pN]+`)

type BlogService struct {
	repo *repository.Repository
}

func NewBlogService(repo *repository.Repository) *BlogService { return &BlogService{repo: repo} }

func (s *BlogService) ListPosts(ctx context.Context, page, pageSize int) ([]model.Post, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return s.repo.ListPosts(ctx, page, pageSize, "published")
}

func (s *BlogService) ListAllPosts(ctx context.Context, page, pageSize int) ([]model.Post, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return s.repo.ListPosts(ctx, page, pageSize, "")
}

func (s *BlogService) SearchPosts(ctx context.Context, query string, page, pageSize int) ([]model.Post, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return s.repo.SearchPosts(ctx, query, page, pageSize)
}

func (s *BlogService) GetPost(ctx context.Context, idOrSlug string, publicOnly bool) (model.Post, error) {
	post, err := s.repo.FindPost(ctx, idOrSlug, publicOnly)
	if err != nil {
		return model.Post{}, err
	}
	if publicOnly {
		post.ViewCount++
		if err := s.repo.SavePost(ctx, &post); err != nil {
			return model.Post{}, err
		}
	}
	return post, nil
}

func (s *BlogService) SavePost(ctx context.Context, post *model.Post) error {
	post.Title = strings.TrimSpace(post.Title)
	post.Content = strings.TrimSpace(post.Content)
	if post.Title == "" || post.Content == "" {
		return ErrInvalidPost
	}
	if post.Status != "" && post.Status != "draft" && post.Status != "published" {
		return ErrInvalidStatus
	}
	if post.Slug == "" {
		post.Slug = Slugify(post.Title)
	}
	if post.Status == "published" && post.PublishedAt == nil {
		now := time.Now().UTC()
		post.PublishedAt = &now
	}
	if post.Status == "" {
		post.Status = "draft"
	}
	return s.repo.SavePost(ctx, post)
}

func (s *BlogService) GetPostWithoutView(ctx context.Context, idOrSlug string, publicOnly bool) (model.Post, error) {
	return s.repo.FindPost(ctx, idOrSlug, publicOnly)
}

func (s *BlogService) DeletePost(ctx context.Context, id uint) error {
	return s.repo.DeletePost(ctx, id)
}

func Slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('-')
		}
	}
	slug := slugPattern.ReplaceAllString(builder.String(), "-")
	return strings.Trim(slug, "-")
}

func PageCount(total int64, size int) int {
	if size <= 0 {
		return 0
	}
	return int((total + int64(size) - 1) / int64(size))
}

func ValidateComment(name, email, body string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(email) == "" || strings.TrimSpace(body) == "" {
		return fmt.Errorf("name, email, and body are required")
	}
	return nil
}
