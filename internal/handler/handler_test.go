package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/auth"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/config"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/gin-gonic/gin"
)

func TestPublicAndAdminPostFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{JWTSecret: "test-secret", AdminEmail: "admin@example.com"}
	if err := repo.SeedAdmin(context.Background(), cfg.AdminEmail, auth.HashPassword("admin123")); err != nil {
		t.Fatal(err)
	}
	admin, _ := repo.FindUserByEmail(context.Background(), cfg.AdminEmail)
	draft := model.Post{Title: "Draft", Slug: "draft", Content: "draft body", Status: "draft", AuthorID: admin.ID}
	if err := repo.SavePost(context.Background(), &draft); err != nil {
		t.Fatal(err)
	}
	router := New(cfg, repo).Router()

	response := request(t, router, http.MethodGet, "/api/v1/posts", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"total":0`)) {
		t.Fatalf("draft leaked from public list: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/categories", nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("category listing failed: %d %s", response.Code, response.Body.String())
	}

	response = request(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": cfg.AdminEmail, "password": "admin123"}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", response.Code, response.Body.String())
	}
	var login struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	response = request(t, router, http.MethodPost, "/api/v1/admin/posts", map[string]string{"title": "Published", "content": "hello", "status": "published"}, login.Token)
	if response.Code != http.StatusCreated {
		t.Fatalf("post creation failed: %d %s", response.Code, response.Body.String())
	}
	var createdPost struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &createdPost); err != nil {
		t.Fatal(err)
	}
	postResponse := request(t, router, http.MethodGet, "/api/v1/posts/published", nil, "")
	if postResponse.Code != http.StatusOK || postResponse.Header().Get("ETag") == "" {
		t.Fatalf("post ETag missing: %d %s", postResponse.Code, postResponse.Body.String())
	}
	conditional := httptest.NewRequest(http.MethodGet, "/api/v1/posts/published", nil)
	conditional.Header.Set("If-None-Match", postResponse.Header().Get("ETag"))
	conditionalResponse := httptest.NewRecorder()
	router.ServeHTTP(conditionalResponse, conditional)
	if conditionalResponse.Code != http.StatusNotModified {
		t.Fatalf("conditional post request was not cached: %d", conditionalResponse.Code)
	}
	response = request(t, router, http.MethodGet, "/api/v1/admin/posts/"+strconv.FormatUint(uint64(createdPost.ID), 10)+"/revisions", nil, login.Token)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"version":1`)) {
		t.Fatalf("post revision missing: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/admin/posts/"+strconv.FormatUint(uint64(createdPost.ID), 10)+"/analytics", nil, login.Token)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"views":1`)) {
		t.Fatalf("post analytics missing: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodPost, "/api/v1/admin/categories", map[string]string{"name": "Go"}, login.Token)
	if response.Code != http.StatusCreated || !bytes.Contains(response.Body.Bytes(), []byte(`"slug":"go"`)) {
		t.Fatalf("category creation failed: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodPost, "/api/v1/admin/tags", map[string]string{"name": "Gin"}, login.Token)
	if response.Code != http.StatusCreated || !bytes.Contains(response.Body.Bytes(), []byte(`"slug":"gin"`)) {
		t.Fatalf("tag creation failed: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/admin/audit-events", nil, login.Token)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"total":3`)) {
		t.Fatalf("audit events missing: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/admin/outbox-events", nil, login.Token)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"items"`)) {
		t.Fatalf("outbox events missing: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/posts", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"total":1`)) {
		t.Fatalf("published post missing: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/search?q=Published", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"total":1`)) {
		t.Fatalf("published post missing from search: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/search?q=x", nil, "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("short search query should be rejected: %d", response.Code)
	}
	response = request(t, router, http.MethodPost, "/api/v1/admin/posts", map[string]string{"title": "Unauthorized", "content": "no"}, "bad-token")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", response.Code)
	}
}

type fakeContentReader struct{}

type fakeIdentityProvider struct{}

func (fakeIdentityProvider) Login(context.Context, string, string) (model.User, string, error) {
	return model.User{ID: 9, Email: "remote@example.com", Role: "admin"}, "remote-token", nil
}

func (fakeIdentityProvider) Health(context.Context) error { return nil }

func (fakeIdentityProvider) Validate(context.Context, string) (uint, string, error) {
	return 9, "admin", nil
}

type fakeCommentProvider struct {
	created bool
}

type fakeTaxonomyProvider struct{}

func (fakeTaxonomyProvider) ListCategories(context.Context) ([]model.Category, error) {
	return []model.Category{{Name: "Remote category", Slug: "remote-category"}}, nil
}

func (fakeTaxonomyProvider) ListTags(context.Context) ([]model.Tag, error) {
	return []model.Tag{{Name: "Remote tag", Slug: "remote-tag"}}, nil
}

func (fakeTaxonomyProvider) Health(context.Context) error { return nil }

type fakeSearchProvider struct{}

func (fakeSearchProvider) SearchPosts(context.Context, string, int, int) ([]model.Post, int64, error) {
	return []model.Post{{Title: "Remote search result", Slug: "remote-search"}}, 1, nil
}

func (fakeSearchProvider) Health(context.Context) error { return nil }

type fakeAuditProvider struct {
	recorded bool
}

type fakeAdminPostProvider struct {
	saved bool
}

func (p *fakeAdminPostProvider) List(context.Context, int, int) ([]model.Post, int64, error) {
	return []model.Post{{ID: 44, Title: "Remote admin post", Slug: "remote-admin-post", Content: "body", Status: "draft"}}, 1, nil
}

func (p *fakeAdminPostProvider) Get(context.Context, string) (model.Post, error) {
	return model.Post{ID: 44, Title: "Remote admin post", Slug: "remote-admin-post", Content: "body", Status: "draft"}, nil
}

func (p *fakeAdminPostProvider) Save(_ context.Context, post *model.Post) error {
	p.saved = true
	if post.ID == 0 {
		post.ID = 44
	}
	return nil
}

func (p *fakeAdminPostProvider) Delete(context.Context, uint) error { return nil }

func (p *fakeAdminPostProvider) ListRevisions(context.Context, uint) ([]model.PostRevision, error) {
	return []model.PostRevision{{PostID: 44, Version: 1}}, nil
}

func (p *fakeAdminPostProvider) Health(context.Context) error { return nil }

func (p *fakeAuditProvider) Record(context.Context, *model.AuditEvent, *model.OutboxEvent) error {
	p.recorded = true
	return nil
}

func (p *fakeAuditProvider) List(context.Context, int, int) ([]model.AuditEvent, int64, error) {
	return []model.AuditEvent{{Action: "remote.audit"}}, 1, nil
}

func (p *fakeAuditProvider) ListPendingOutbox(context.Context, int) ([]model.OutboxEvent, error) {
	return []model.OutboxEvent{{EventType: "remote.audit"}}, nil
}

func (p *fakeAuditProvider) Health(context.Context) error { return nil }

func (p *fakeCommentProvider) List(context.Context, uint, bool) ([]model.Comment, error) {
	return []model.Comment{{ID: 42, Name: "Remote reader", Approved: true}}, nil
}

func (p *fakeCommentProvider) Create(context.Context, uint, string, string, string) (uint, string, error) {
	p.created = true
	return 42, "remote comment accepted", nil
}

func (p *fakeCommentProvider) Moderate(context.Context, uint, bool) error { return nil }

func (p *fakeCommentProvider) Health(context.Context) error { return nil }

func (fakeContentReader) ListPosts(context.Context, int, int) ([]model.Post, int64, error) {
	return []model.Post{{Title: "Remote post"}}, 1, nil
}

func (fakeContentReader) SearchPosts(context.Context, string, int, int) ([]model.Post, int64, error) {
	return []model.Post{{Title: "Remote search result"}}, 1, nil
}

func (fakeContentReader) GetPost(context.Context, string, bool) (model.Post, error) {
	return model.Post{Title: "Remote detail"}, nil
}

func (fakeContentReader) GetPostWithoutView(context.Context, string, bool) (model.Post, error) {
	return model.Post{ID: 1, Title: "Remote detail"}, nil
}

func (fakeContentReader) ListCategories(context.Context) ([]model.Category, error) {
	return []model.Category{{Name: "Remote category"}}, nil
}

func (fakeContentReader) ListTags(context.Context) ([]model.Tag, error) {
	return []model.Tag{{Name: "Remote tag"}}, nil
}

func (fakeContentReader) Health(context.Context) error { return nil }

func TestRemoteContentReaderFeedsPublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{JWTSecret: "test-secret"}
	router := NewWithReader(cfg, repo, fakeContentReader{}, logging.NewText("gateway-test")).Router()
	response := request(t, router, http.MethodGet, "/api/v1/posts", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"title":"Remote post"`)) {
		t.Fatalf("gateway did not use remote reader: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/readyz", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"content":"remote"`)) {
		t.Fatalf("gateway readiness did not check remote reader: %d %s", response.Code, response.Body.String())
	}
}

func TestPublicPaginationMetadataUsesNormalizedValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{JWTSecret: "test-secret"}
	router := New(cfg, repo).Router()
	response := request(t, router, http.MethodGet, "/api/v1/posts?page=0&pageSize=1000", nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("post listing failed: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Page != 1 || body.PageSize != 10 {
		t.Fatalf("pagination metadata was not normalized: %#v", body)
	}
}

func TestRemoteIdentityProviderFeedsLoginRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{JWTSecret: "test-secret"}
	router := NewWithProviders(cfg, repo, fakeContentReader{}, fakeIdentityProvider{}, logging.NewText("identity-gateway-test"), nil).Router()
	response := request(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "remote@example.com", "password": "secret"}, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"token":"remote-token"`)) {
		t.Fatalf("gateway did not use remote identity: %d %s", response.Code, response.Body.String())
	}
}

func TestRemoteCommentProviderFeedsCommentRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	comments := &fakeCommentProvider{}
	cfg := config.Config{JWTSecret: "test-secret"}
	router := NewWithComments(cfg, repo, fakeContentReader{}, fakeIdentityProvider{}, comments, logging.NewText("comments-gateway-test"), nil).Router()
	response := request(t, router, http.MethodPost, "/api/v1/posts/remote/comments", map[string]string{"name": "Reader", "email": "reader@example.com", "body": "Hello"}, "")
	if response.Code != http.StatusCreated || !comments.created || !bytes.Contains(response.Body.Bytes(), []byte(`"id":42`)) {
		t.Fatalf("gateway did not use remote comments: %d %s", response.Code, response.Body.String())
	}
}

func TestRemoteTaxonomyProviderFeedsPublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{JWTSecret: "test-secret"}
	router := NewWithDependencies(cfg, repo, fakeContentReader{}, fakeIdentityProvider{}, nil, nil, fakeTaxonomyProvider{}, logging.NewText("taxonomy-gateway-test"), nil).Router()
	response := request(t, router, http.MethodGet, "/api/v1/categories", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"slug":"remote-category"`)) {
		t.Fatalf("gateway did not use remote taxonomy categories: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/tags", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"slug":"remote-tag"`)) {
		t.Fatalf("gateway did not use remote taxonomy tags: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/readyz", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"taxonomy":"remote"`)) {
		t.Fatalf("gateway readiness did not check remote taxonomy: %d %s", response.Code, response.Body.String())
	}
}

func TestRemoteSearchProviderFeedsSearchRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{JWTSecret: "test-secret"}
	router := NewWithServices(cfg, repo, fakeContentReader{}, fakeIdentityProvider{}, nil, nil, nil, fakeSearchProvider{}, logging.NewText("search-gateway-test"), nil).Router()
	response := request(t, router, http.MethodGet, "/api/v1/search?q=distributed", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"slug":"remote-search"`)) {
		t.Fatalf("gateway did not use remote search: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/readyz", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"search":"remote"`)) {
		t.Fatalf("gateway readiness did not check remote search: %d %s", response.Code, response.Body.String())
	}
}

func TestRemoteAuditProviderFeedsAdminRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	audit := &fakeAuditProvider{}
	cfg := config.Config{JWTSecret: "test-secret"}
	router := NewWithServicesAndAudit(cfg, repo, fakeContentReader{}, fakeIdentityProvider{}, nil, nil, nil, nil, audit, logging.NewText("audit-gateway-test"), nil).Router()
	response := request(t, router, http.MethodGet, "/api/v1/admin/audit-events", nil, "remote-token")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"action":"remote.audit"`)) {
		t.Fatalf("gateway did not use remote audit list: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/admin/outbox-events", nil, "remote-token")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"eventType":"remote.audit"`)) {
		t.Fatalf("gateway did not use remote outbox list: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodPost, "/api/v1/admin/posts", map[string]string{"title": "Remote audit", "content": "body"}, "remote-token")
	if response.Code != http.StatusCreated || !audit.recorded {
		t.Fatalf("gateway did not use remote audit record: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/readyz", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"audit":"remote"`)) {
		t.Fatalf("gateway readiness did not check remote audit: %d %s", response.Code, response.Body.String())
	}
}

func TestRemoteAdminPostProviderFeedsAdminRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	posts := &fakeAdminPostProvider{}
	cfg := config.Config{JWTSecret: "test-secret"}
	router := NewWithAllDependencies(cfg, repo, fakeContentReader{}, fakeIdentityProvider{}, nil, nil, nil, nil, nil, posts, logging.NewText("posts-gateway-test"), nil).Router()
	response := request(t, router, http.MethodGet, "/api/v1/admin/posts", nil, "remote-token")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"title":"Remote admin post"`)) {
		t.Fatalf("gateway did not use remote admin post list: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodPost, "/api/v1/admin/posts", map[string]string{"title": "Created remotely", "content": "body"}, "remote-token")
	if response.Code != http.StatusCreated || !posts.saved {
		t.Fatalf("gateway did not use remote admin post save: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/api/v1/admin/posts/44/revisions", nil, "remote-token")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"version":1`)) {
		t.Fatalf("gateway did not use remote revisions: %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/readyz", nil, "")
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"posts":"remote"`)) {
		t.Fatalf("gateway readiness did not check remote posts: %d %s", response.Code, response.Body.String())
	}
}

func request(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}
