package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/auth"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/config"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/metrics"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/middleware"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/service"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ContentReader interface {
	ListPosts(context.Context, int, int) ([]model.Post, int64, error)
	SearchPosts(context.Context, string, int, int) ([]model.Post, int64, error)
	GetPost(context.Context, string, bool) (model.Post, error)
	GetPostWithoutView(context.Context, string, bool) (model.Post, error)
	ListCategories(context.Context) ([]model.Category, error)
	ListTags(context.Context) ([]model.Tag, error)
}

type ContentHealthChecker interface {
	Health(context.Context) error
}

type Handler struct {
	cfg     config.Config
	repo    *repository.Repository
	service *service.BlogService
	reader  ContentReader
	logger  *slog.Logger
	metrics *metrics.Registry
}

func New(cfg config.Config, repo *repository.Repository) *Handler {
	return NewWithLogger(cfg, repo, logging.New("blog-api"))
}

func NewWithLogger(cfg config.Config, repo *repository.Repository, logger *slog.Logger) *Handler {
	return NewWithReader(cfg, repo, nil, logger)
}

func NewWithReader(cfg config.Config, repo *repository.Repository, reader ContentReader, logger *slog.Logger) *Handler {
	return NewWithReaderAndMetrics(cfg, repo, reader, logger, metrics.New())
}

func NewWithReaderAndMetrics(cfg config.Config, repo *repository.Repository, reader ContentReader, logger *slog.Logger, registry *metrics.Registry) *Handler {
	blogService := service.NewBlogService(repo)
	if reader == nil {
		reader = blogService
	}
	if registry == nil {
		registry = metrics.New()
	}
	return &Handler{cfg: cfg, repo: repo, service: blogService, reader: reader, logger: logger, metrics: registry}
}

func (h *Handler) Router() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery(), middleware.SecurityHeaders(), middleware.BodyLimit(h.cfg.MaxBodyBytes), middleware.NewRateLimiter(h.cfg.RateLimitRequests, h.cfg.RateLimitWindow).Handler(), logging.HTTPMiddlewareWithMetrics(h.logger, h.metrics))
	router.GET("/healthz", h.health)
	router.GET("/readyz", h.ready)
	router.GET("/metrics", h.metrics.Handler)

	api := router.Group("/api/v1")
	api.POST("/auth/login", h.login)
	api.GET("/posts", h.listPublicPosts)
	api.GET("/search", h.searchPosts)
	api.GET("/categories", h.listCategories)
	api.GET("/tags", h.listTags)
	api.GET("/posts/:identifier", h.getPublicPost)
	api.POST("/posts/:identifier/comments", h.createComment)
	api.GET("/posts/:identifier/comments", h.listComments)

	admin := api.Group("/admin", middleware.RequireAuth(h.cfg.JWTSecret), middleware.RequireRole("admin"))
	admin.GET("/posts", h.listAdminPosts)
	admin.POST("/posts", h.createPost)
	admin.PUT("/posts/:id", h.updatePost)
	admin.DELETE("/posts/:id", h.deletePost)
	admin.POST("/categories", h.createCategory)
	admin.POST("/tags", h.createTag)
	admin.GET("/comments", h.listPendingComments)
	admin.PATCH("/comments/:id", h.approveComment)
	admin.GET("/audit-events", h.listAuditEvents)
	admin.GET("/outbox-events", h.listOutboxEvents)
	return router
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "gin-vue-blog-go"})
}

func (h *Handler) ready(c *gin.Context) {
	checker, remote := h.reader.(ContentHealthChecker)
	if !remote {
		c.JSON(http.StatusOK, gin.H{"status": "ready", "content": "local"})
		return
	}
	if err := checker.Health(c.Request.Context()); err != nil {
		internalErrorStatus(c, http.StatusServiceUnavailable, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "content": "remote"})
}

func (h *Handler) login(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if !bindJSON(c, &input) {
		return
	}
	user, err := h.repo.FindUserByEmail(c.Request.Context(), input.Email)
	if err != nil || !auth.CheckPassword(user.PasswordHash, input.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	token, err := auth.IssueToken(h.cfg.JWTSecret, user.ID, user.Role, time.Now().UTC())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

func (h *Handler) listPublicPosts(c *gin.Context) {
	page, size := pagination(c)
	posts, total, err := h.reader.ListPosts(c.Request.Context(), page, size)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": posts, "page": page, "pageSize": size, "total": total, "pages": service.PageCount(total, size)})
}

func (h *Handler) searchPosts(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if len([]rune(query)) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q must contain at least 2 characters"})
		return
	}
	page, size := pagination(c)
	posts, total, err := h.reader.SearchPosts(c.Request.Context(), query, page, size)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": posts, "query": query, "page": page, "pageSize": size, "total": total, "pages": service.PageCount(total, size)})
}

func (h *Handler) listAdminPosts(c *gin.Context) {
	page, size := pagination(c)
	posts, total, err := h.service.ListAllPosts(c.Request.Context(), page, size)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": posts, "page": page, "pageSize": size, "total": total, "pages": service.PageCount(total, size)})
}

func (h *Handler) listCategories(c *gin.Context) {
	categories, err := h.reader.ListCategories(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": categories})
}

func (h *Handler) listTags(c *gin.Context) {
	tags, err := h.reader.ListTags(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": tags})
}

type taxonomyInput struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *Handler) createCategory(c *gin.Context) {
	var input taxonomyInput
	if !bindJSON(c, &input) {
		return
	}
	category, err := h.service.CreateCategory(c.Request.Context(), input.Name, input.Slug)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTaxonomy) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		internalError(c, err)
		return
	}
	claims := c.MustGet(middleware.ClaimsKey).(auth.Claims)
	h.recordAudit(c, claims.UserID, "category.created", "category", category.ID, map[string]string{"slug": category.Slug})
	c.JSON(http.StatusCreated, category)
}

func (h *Handler) createTag(c *gin.Context) {
	var input taxonomyInput
	if !bindJSON(c, &input) {
		return
	}
	tag, err := h.service.CreateTag(c.Request.Context(), input.Name, input.Slug)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTaxonomy) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		internalError(c, err)
		return
	}
	claims := c.MustGet(middleware.ClaimsKey).(auth.Claims)
	h.recordAudit(c, claims.UserID, "tag.created", "tag", tag.ID, map[string]string{"slug": tag.Slug})
	c.JSON(http.StatusCreated, tag)
}

func (h *Handler) getPublicPost(c *gin.Context) {
	post, err := h.reader.GetPost(c.Request.Context(), c.Param("identifier"), true)
	if isNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, post)
}

func (h *Handler) createPost(c *gin.Context) {
	var input postInput
	if !bindJSON(c, &input) {
		return
	}
	claims := c.MustGet(middleware.ClaimsKey).(auth.Claims)
	post := input.model()
	post.AuthorID = claims.UserID
	if err := h.service.SavePost(c.Request.Context(), &post); err != nil {
		if errors.Is(err, service.ErrInvalidPost) || errors.Is(err, service.ErrInvalidStatus) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		internalError(c, err)
		return
	}
	h.recordAudit(c, claims.UserID, "post.created", "post", post.ID, map[string]string{"status": post.Status})
	c.JSON(http.StatusCreated, post)
}

func (h *Handler) updatePost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}
	post, err := h.service.GetPost(c.Request.Context(), strconv.FormatUint(id, 10), false)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	var input postInput
	if !bindJSON(c, &input) {
		return
	}
	input.apply(&post)
	if err := h.service.SavePost(c.Request.Context(), &post); err != nil {
		if errors.Is(err, service.ErrInvalidPost) || errors.Is(err, service.ErrInvalidStatus) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		internalError(c, err)
		return
	}
	claims := c.MustGet(middleware.ClaimsKey).(auth.Claims)
	h.recordAudit(c, claims.UserID, "post.updated", "post", post.ID, map[string]string{"status": post.Status})
	c.JSON(http.StatusOK, post)
}

func (h *Handler) deletePost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}
	if err := h.service.DeletePost(c.Request.Context(), uint(id)); errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	} else if err != nil {
		internalError(c, err)
		return
	}
	claims := c.MustGet(middleware.ClaimsKey).(auth.Claims)
	h.recordAudit(c, claims.UserID, "post.deleted", "post", uint(id), nil)
	c.Status(http.StatusNoContent)
}

func (h *Handler) createComment(c *gin.Context) {
	post, err := h.reader.GetPostWithoutView(c.Request.Context(), c.Param("identifier"), true)
	if isNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	var input commentInput
	if !bindJSON(c, &input) {
		return
	}
	if err := service.ValidateComment(input.Name, input.Email, input.Body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	comment := model.Comment{PostID: post.ID, Name: strings.TrimSpace(input.Name), Email: strings.TrimSpace(input.Email), Body: strings.TrimSpace(input.Body)}
	if err := h.repo.CreateComment(c.Request.Context(), &comment); err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": comment.ID, "message": "comment submitted for moderation"})
}

func (h *Handler) listComments(c *gin.Context) {
	post, err := h.reader.GetPostWithoutView(c.Request.Context(), c.Param("identifier"), true)
	if isNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	comments, err := h.repo.ListComments(c.Request.Context(), post.ID, false)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": comments})
}

func (h *Handler) listPendingComments(c *gin.Context) {
	comments, err := h.repo.ListComments(c.Request.Context(), 0, true)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": comments})
}

func (h *Handler) approveComment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment id"})
		return
	}
	var input struct {
		Approved bool `json:"approved"`
	}
	if !bindJSON(c, &input) {
		return
	}
	if err := h.repo.SetCommentApproval(c.Request.Context(), uint(id), input.Approved); errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
		return
	} else if err != nil {
		internalError(c, err)
		return
	}
	claims := c.MustGet(middleware.ClaimsKey).(auth.Claims)
	h.recordAudit(c, claims.UserID, "comment.moderated", "comment", uint(id), map[string]string{"approved": strconv.FormatBool(input.Approved)})
	c.JSON(http.StatusOK, gin.H{"approved": input.Approved})
}

func (h *Handler) listAuditEvents(c *gin.Context) {
	page, size := pagination(c)
	events, total, err := h.repo.ListAuditEvents(c.Request.Context(), page, size)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": events, "page": page, "pageSize": size, "total": total, "pages": service.PageCount(total, size)})
}

func (h *Handler) listOutboxEvents(c *gin.Context) {
	events, err := h.repo.ListPendingOutboxEvents(c.Request.Context(), 100)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": events})
}

type postInput struct {
	Title      string `json:"title"`
	Slug       string `json:"slug"`
	Excerpt    string `json:"excerpt"`
	Content    string `json:"content"`
	Status     string `json:"status"`
	CategoryID *uint  `json:"categoryId"`
}

func (p postInput) model() model.Post {
	return model.Post{Title: p.Title, Slug: p.Slug, Excerpt: p.Excerpt, Content: p.Content, Status: p.Status, CategoryID: p.CategoryID}
}

func (p postInput) apply(post *model.Post) {
	post.Title, post.Slug, post.Excerpt, post.Content, post.Status, post.CategoryID = p.Title, p.Slug, p.Excerpt, p.Content, p.Status, p.CategoryID
}

type commentInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Body  string `json:"body"`
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return false
	}
	return true
}

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	return page, size
}

func internalError(c *gin.Context, err error) {
	internalErrorStatus(c, http.StatusInternalServerError, err)
}

func internalErrorStatus(c *gin.Context, statusCode int, err error) {
	c.Error(err)
	c.JSON(statusCode, gin.H{"error": "internal server error"})
}

func isNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound) || status.Code(err) == codes.NotFound
}

func (h *Handler) recordAudit(c *gin.Context, actorID uint, action, resourceType string, resourceID uint, metadata map[string]string) {
	payload, err := json.Marshal(metadata)
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "marshal audit metadata", "error", err, "action", action)
		return
	}
	event := &model.AuditEvent{
		RequestID:    logging.RequestID(c.Request.Context()),
		TraceID:      logging.TraceID(c.Request.Context()),
		Service:      "blog-api",
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     string(payload),
	}
	if err := h.repo.CreateAuditEvent(c.Request.Context(), event); err != nil {
		h.logger.ErrorContext(c.Request.Context(), "write audit event", "error", err, "action", action, "resource_id", resourceID)
		return
	}
	h.metrics.ObserveAudit()
	outbox := &model.OutboxEvent{EventType: action, AggregateType: resourceType, AggregateID: resourceID, Payload: string(payload)}
	if err := h.repo.CreateOutboxEvent(c.Request.Context(), outbox); err != nil {
		h.logger.ErrorContext(c.Request.Context(), "write outbox event", "error", err, "action", action, "resource_id", resourceID)
	}
}
