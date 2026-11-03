package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

type IdentityProvider interface {
	Login(context.Context, string, string) (model.User, string, error)
	Validate(context.Context, string) (uint, string, error)
}

type CommentProvider interface {
	List(context.Context, uint, bool) ([]model.Comment, error)
	Create(context.Context, uint, string, string, string) (uint, string, error)
	Moderate(context.Context, uint, bool) error
}

type localCommentProvider struct {
	repo *repository.Repository
}

func (p localCommentProvider) List(ctx context.Context, postID uint, includePending bool) ([]model.Comment, error) {
	return p.repo.ListComments(ctx, postID, includePending)
}

func (p localCommentProvider) Create(ctx context.Context, postID uint, name, email, body string) (uint, string, error) {
	if err := service.ValidateComment(name, email, body); err != nil {
		return 0, "", status.Error(codes.InvalidArgument, err.Error())
	}
	comment := &model.Comment{PostID: postID, Name: strings.TrimSpace(name), Email: strings.TrimSpace(email), Body: strings.TrimSpace(body)}
	if err := p.repo.CreateComment(ctx, comment); err != nil {
		return 0, "", err
	}
	return comment.ID, "comment submitted for moderation", nil
}

func (p localCommentProvider) Moderate(ctx context.Context, id uint, approved bool) error {
	return p.repo.SetCommentApproval(ctx, id, approved)
}

type localIdentityProvider struct {
	repo      *repository.Repository
	jwtSecret string
}

func (p localIdentityProvider) Login(ctx context.Context, email, password string) (model.User, string, error) {
	user, err := p.repo.FindUserByEmail(ctx, email)
	if err != nil || !auth.CheckPassword(user.PasswordHash, password) {
		return model.User{}, "", status.Error(codes.Unauthenticated, "invalid credentials")
	}
	token, err := auth.IssueToken(p.jwtSecret, user.ID, user.Role, time.Now().UTC())
	if err != nil {
		return model.User{}, "", err
	}
	return user, token, nil
}

func (p localIdentityProvider) Validate(_ context.Context, token string) (uint, string, error) {
	claims, err := auth.ParseToken(p.jwtSecret, token)
	if err != nil {
		return 0, "", err
	}
	return claims.UserID, claims.Role, nil
}

type Handler struct {
	cfg      config.Config
	repo     *repository.Repository
	service  *service.BlogService
	reader   ContentReader
	identity IdentityProvider
	comments CommentProvider
	logger   *slog.Logger
	metrics  *metrics.Registry
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
	return NewWithProviders(cfg, repo, reader, localIdentityProvider{repo: repo, jwtSecret: cfg.JWTSecret}, logger, registry)
}

func NewWithProviders(cfg config.Config, repo *repository.Repository, reader ContentReader, identity IdentityProvider, logger *slog.Logger, registry *metrics.Registry) *Handler {
	return NewWithComments(cfg, repo, reader, identity, nil, logger, registry)
}

func NewWithComments(cfg config.Config, repo *repository.Repository, reader ContentReader, identity IdentityProvider, comments CommentProvider, logger *slog.Logger, registry *metrics.Registry) *Handler {
	blogService := service.NewBlogService(repo)
	if reader == nil {
		reader = blogService
	}
	if registry == nil {
		registry = metrics.New()
	}
	if identity == nil {
		identity = localIdentityProvider{repo: repo, jwtSecret: cfg.JWTSecret}
	}
	if comments == nil {
		comments = localCommentProvider{repo: repo}
	}
	return &Handler{cfg: cfg, repo: repo, service: blogService, reader: reader, identity: identity, comments: comments, logger: logger, metrics: registry}
}

func (h *Handler) Router() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery(), middleware.SecurityHeaders(), middleware.BodyLimit(h.cfg.MaxBodyBytes), middleware.NewRateLimiterWithTrustedProxies(h.cfg.RateLimitRequests, h.cfg.RateLimitWindow, h.cfg.TrustedProxies).Handler(), logging.HTTPMiddlewareWithMetrics(h.logger, h.metrics))
	router.GET("/healthz", h.health)
	router.GET("/readyz", h.ready)
	router.GET("/metrics", h.metrics.Handler)

	api := router.Group("/api/v1")
	api.Use(middleware.APIVersion("v1"))
	api.POST("/auth/login", h.login)
	api.GET("/posts", h.listPublicPosts)
	api.GET("/search", h.searchPosts)
	api.GET("/categories", h.listCategories)
	api.GET("/tags", h.listTags)
	api.GET("/posts/:identifier", h.getPublicPost)
	api.POST("/posts/:identifier/comments", h.createComment)
	api.GET("/posts/:identifier/comments", h.listComments)

	admin := api.Group("/admin", middleware.RequireTokenValidator(h.identity), middleware.RequireRole("admin"))
	admin.GET("/posts", h.listAdminPosts)
	admin.POST("/posts", h.createPost)
	admin.PUT("/posts/:id", h.updatePost)
	admin.DELETE("/posts/:id", h.deletePost)
	admin.GET("/posts/:id/revisions", h.listPostRevisions)
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
	dependencies := gin.H{"content": "local", "identity": "local", "comments": "local"}
	for name, dependency := range map[string]any{"content": h.reader, "identity": h.identity, "comments": h.comments} {
		checker, remote := dependency.(ContentHealthChecker)
		if !remote {
			continue
		}
		if err := checker.Health(c.Request.Context()); err != nil {
			internalErrorStatus(c, http.StatusServiceUnavailable, err)
			return
		}
		dependencies[name] = "remote"
	}
	dependencies["status"] = "ready"
	c.JSON(http.StatusOK, dependencies)
}

func (h *Handler) login(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if !bindJSON(c, &input) {
		return
	}
	user, token, err := h.identity.Login(c.Request.Context(), input.Email, input.Password)
	if status.Code(err) == codes.Unauthenticated {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
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
	etag := postETag(post)
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
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
	h.recordRevision(c, claims.UserID, post)
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
	h.recordRevision(c, claims.UserID, post)
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

func (h *Handler) listPostRevisions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}
	revisions, err := h.repo.ListPostRevisions(c.Request.Context(), uint(id))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": revisions})
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
	commentID, message, err := h.comments.Create(c.Request.Context(), post.ID, input.Name, input.Email, input.Body)
	if status.Code(err) == codes.InvalidArgument {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": commentID, "message": message})
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
	comments, err := h.comments.List(c.Request.Context(), post.ID, false)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": comments})
}

func (h *Handler) listPendingComments(c *gin.Context) {
	comments, err := h.comments.List(c.Request.Context(), 0, true)
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
	if err := h.comments.Moderate(c.Request.Context(), uint(id), input.Approved); isNotFound(err) {
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
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}
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
	outbox := &model.OutboxEvent{EventType: action, AggregateType: resourceType, AggregateID: resourceID, Payload: string(payload)}
	if err := h.repo.RecordAuditAndOutbox(c.Request.Context(), event, outbox); err != nil {
		h.logger.ErrorContext(c.Request.Context(), "write audit and outbox event", "error", err, "action", action, "resource_id", resourceID)
		return
	}
	h.metrics.ObserveAudit()
}

func (h *Handler) recordRevision(c *gin.Context, editorID uint, post model.Post) {
	revision := &model.PostRevision{PostID: post.ID, Title: post.Title, Excerpt: post.Excerpt, Content: post.Content, Status: post.Status, EditorID: editorID}
	if err := h.repo.CreatePostRevision(c.Request.Context(), revision); err != nil {
		h.logger.ErrorContext(c.Request.Context(), "write post revision", "error", err, "post_id", post.ID)
	}
}

func postETag(post model.Post) string {
	digest := sha256.Sum256([]byte(strconv.FormatUint(uint64(post.ID), 10) + ":" + post.UpdatedAt.UTC().Format(time.RFC3339Nano)))
	return `"` + hex.EncodeToString(digest[:]) + `"`
}
