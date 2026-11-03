package model

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"displayName"`
	Role         string    `gorm:"not null;default:reader" json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	Slug      string    `gorm:"uniqueIndex;not null" json:"slug"`
	CreatedAt time.Time `json:"createdAt"`
}

type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	Slug      string    `gorm:"uniqueIndex;not null" json:"slug"`
	CreatedAt time.Time `json:"createdAt"`
}

type Post struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Title       string     `gorm:"not null" json:"title"`
	Slug        string     `gorm:"uniqueIndex;not null" json:"slug"`
	Excerpt     string     `json:"excerpt"`
	Content     string     `gorm:"type:text;not null" json:"content"`
	Status      string     `gorm:"index;not null;default:draft" json:"status"`
	PublishedAt *time.Time `json:"publishedAt"`
	ViewCount   uint       `gorm:"not null;default:0" json:"viewCount"`
	AuthorID    uint       `json:"authorId"`
	Author      User       `json:"author"`
	CategoryID  *uint      `json:"categoryId"`
	Category    *Category  `json:"category"`
	Tags        []Tag      `gorm:"many2many:post_tags;" json:"tags"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PostID    uint      `gorm:"index;not null" json:"postId"`
	Post      Post      `json:"-"`
	Name      string    `gorm:"not null" json:"name"`
	Email     string    `gorm:"not null" json:"-"`
	Body      string    `gorm:"type:text;not null" json:"body"`
	Approved  bool      `gorm:"index;not null;default:false" json:"approved"`
	CreatedAt time.Time `json:"createdAt"`
}

type AuditEvent struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RequestID    string    `gorm:"index;not null" json:"requestId"`
	TraceID      string    `gorm:"index;not null" json:"traceId"`
	Service      string    `gorm:"not null" json:"service"`
	ActorID      uint      `gorm:"index" json:"actorId"`
	Action       string    `gorm:"index;not null" json:"action"`
	ResourceType string    `gorm:"index;not null" json:"resourceType"`
	ResourceID   uint      `json:"resourceId"`
	Metadata     string    `gorm:"type:text" json:"metadata"`
	CreatedAt    time.Time `json:"createdAt"`
}
