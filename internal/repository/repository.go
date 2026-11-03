package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var ErrNotFound = gorm.ErrRecordNotFound

type Repository struct {
	db *gorm.DB
}

func Open(databaseURL string) (*Repository, error) {
	db, err := gorm.Open(sqlite.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&model.User{}, &model.Category{}, &model.Tag{}, &model.Post{}, &model.Comment{}, &model.AuditEvent{}, &model.OutboxEvent{}); err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

func (r *Repository) DB() *gorm.DB { return r.db }

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", strings.ToLower(strings.TrimSpace(email))).First(&user).Error
	return user, err
}

func (r *Repository) CreateUser(ctx context.Context, user *model.User) error {
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *Repository) ListCategories(ctx context.Context) ([]model.Category, error) {
	var categories []model.Category
	err := r.db.WithContext(ctx).Order("name ASC").Find(&categories).Error
	return categories, err
}

func (r *Repository) CreateCategory(ctx context.Context, category *model.Category) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *Repository) ListTags(ctx context.Context) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.WithContext(ctx).Order("name ASC").Find(&tags).Error
	return tags, err
}

func (r *Repository) CreateTag(ctx context.Context, tag *model.Tag) error {
	return r.db.WithContext(ctx).Create(tag).Error
}

func (r *Repository) ListPosts(ctx context.Context, page, pageSize int, status string) ([]model.Post, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Post{}).Preload("Author").Preload("Category").Preload("Tags")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var posts []model.Post
	err := query.Order("published_at DESC NULLS LAST, created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&posts).Error
	return posts, total, err
}

func (r *Repository) SearchPosts(ctx context.Context, query string, page, pageSize int) ([]model.Post, int64, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []model.Post{}, 0, nil
	}
	base := r.db.WithContext(ctx).Model(&model.Post{}).
		Where("status = ?", "published").
		Where("title LIKE ? OR excerpt LIKE ? OR content LIKE ?", "%"+query+"%", "%"+query+"%", "%"+query+"%").
		Preload("Author").Preload("Category").Preload("Tags")
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var posts []model.Post
	err := base.Order("published_at DESC NULLS LAST, created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&posts).Error
	return posts, total, err
}

func (r *Repository) FindPost(ctx context.Context, idOrSlug string, publicOnly bool) (model.Post, error) {
	query := r.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags")
	if publicOnly {
		query = query.Where("status = ?", "published")
	}
	var post model.Post
	condition := "slug = ?"
	var value any = idOrSlug
	if id := parseID(idOrSlug); id > 0 {
		condition, value = "id = ?", id
	}
	err := query.Where(condition, value).First(&post).Error
	return post, err
}

func (r *Repository) SavePost(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Save(post).Error
}

func (r *Repository) DeletePost(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&model.Post{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListComments(ctx context.Context, postID uint, includePending bool) ([]model.Comment, error) {
	query := r.db.WithContext(ctx).Order("created_at ASC")
	if postID > 0 {
		query = query.Where("post_id = ?", postID)
	}
	if !includePending {
		query = query.Where("approved = ?", true)
	}
	var comments []model.Comment
	return comments, query.Find(&comments).Error
}

func (r *Repository) CreateComment(ctx context.Context, comment *model.Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *Repository) SetCommentApproval(ctx context.Context, id uint, approved bool) error {
	result := r.db.WithContext(ctx).Model(&model.Comment{}).Where("id = ?", id).Update("approved", approved)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CreateAuditEvent(ctx context.Context, event *model.AuditEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *Repository) ListAuditEvents(ctx context.Context, page, pageSize int) ([]model.AuditEvent, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.AuditEvent{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var events []model.AuditEvent
	err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&events).Error
	return events, total, err
}

func (r *Repository) CreateOutboxEvent(ctx context.Context, event *model.OutboxEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *Repository) ListPendingOutboxEvents(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	var events []model.OutboxEvent
	err := r.db.WithContext(ctx).Where("published_at IS NULL").Order("created_at ASC").Limit(limit).Find(&events).Error
	return events, err
}

func (r *Repository) MarkOutboxPublished(ctx context.Context, id uint, publishedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.OutboxEvent{}).Where("id = ? AND published_at IS NULL", id).Update("published_at", publishedAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) SeedAdmin(ctx context.Context, email, passwordHash string) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return r.CreateUser(ctx, &model.User{Email: email, PasswordHash: passwordHash, DisplayName: "Administrator", Role: "admin"})
}

func parseID(value string) uint {
	var id uint
	if _, err := fmt.Sscanf(value, "%d", &id); err != nil {
		return 0
	}
	return id
}
