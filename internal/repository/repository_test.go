package repository

import (
	"context"
	"testing"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
)

func TestRepositoryPostAndCommentLifecycle(t *testing.T) {
	repo, err := Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user := model.User{Email: "author@example.com", DisplayName: "Author", Role: "admin", PasswordHash: "hash"}
	if err := repo.CreateUser(ctx, &user); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateCategory(ctx, &model.Category{Name: "Go", Slug: "go"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateTag(ctx, &model.Tag{Name: "Gin", Slug: "gin"}); err != nil {
		t.Fatal(err)
	}
	categories, err := repo.ListCategories(ctx)
	if err != nil || len(categories) != 1 {
		t.Fatalf("unexpected categories: %#v error=%v", categories, err)
	}
	tags, err := repo.ListTags(ctx)
	if err != nil || len(tags) != 1 {
		t.Fatalf("unexpected tags: %#v error=%v", tags, err)
	}
	post := model.Post{Title: "Repository post", Slug: "repository-post", Content: "body", Status: "published", AuthorID: user.ID}
	if err := repo.SavePost(ctx, &post); err != nil {
		t.Fatal(err)
	}
	posts, total, err := repo.ListPosts(ctx, 1, 10, "published")
	if err != nil || total != 1 || len(posts) != 1 {
		t.Fatalf("unexpected posts: total=%d posts=%d error=%v", total, len(posts), err)
	}
	results, total, err := repo.SearchPosts(ctx, "body", 1, 10)
	if err != nil || total != 1 || len(results) != 1 {
		t.Fatalf("unexpected search results: total=%d results=%d error=%v", total, len(results), err)
	}
	found, err := repo.FindPost(ctx, post.Slug, true)
	if err != nil || found.ID != post.ID {
		t.Fatalf("lookup by slug failed: %#v error=%v", found, err)
	}
	comment := model.Comment{PostID: post.ID, Name: "Reader", Email: "reader@example.com", Body: "Great post"}
	if err := repo.CreateComment(ctx, &comment); err != nil {
		t.Fatal(err)
	}
	comments, err := repo.ListComments(ctx, post.ID, false)
	if err != nil || len(comments) != 0 {
		t.Fatalf("pending comment was publicly visible: %#v error=%v", comments, err)
	}
	if err := repo.SetCommentApproval(ctx, comment.ID, true); err != nil {
		t.Fatal(err)
	}
	comments, err = repo.ListComments(ctx, post.ID, false)
	if err != nil || len(comments) != 1 {
		t.Fatalf("approved comment missing: %#v error=%v", comments, err)
	}
	if err := repo.DeletePost(ctx, post.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeletePost(ctx, post.ID); err != ErrNotFound {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}
