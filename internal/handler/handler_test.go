package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/auth"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/config"
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
