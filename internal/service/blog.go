package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
)

var ErrInvalidPost = errors.New("title and content are required")
var ErrInvalidStatus = errors.New("status must be draft or published")
var ErrInvalidTaxonomy = errors.New("name is required")
var ErrInvalidDisplayName = errors.New("display name must be between 1 and 120 characters")
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
		if err := s.repo.IncrementPostView(ctx, post.ID); err != nil {
			return model.Post{}, err
		}
		post.ViewCount++
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

func (s *BlogService) ListCategories(ctx context.Context) ([]model.Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *BlogService) CreateCategory(ctx context.Context, name, slug string) (model.Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Category{}, ErrInvalidTaxonomy
	}
	if strings.TrimSpace(slug) == "" {
		slug = Slugify(name)
	}
	category := model.Category{Name: name, Slug: slug}
	return category, s.repo.CreateCategory(ctx, &category)
}

func (s *BlogService) ListTags(ctx context.Context) ([]model.Tag, error) {
	return s.repo.ListTags(ctx)
}

func (s *BlogService) CreateTag(ctx context.Context, name, slug string) (model.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Tag{}, ErrInvalidTaxonomy
	}
	if strings.TrimSpace(slug) == "" {
		slug = Slugify(name)
	}
	tag := model.Tag{Name: name, Slug: slug}
	return tag, s.repo.CreateTag(ctx, &tag)
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

func NormalizeDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 120 {
		return "", ErrInvalidDisplayName
	}
	return value, nil
}
