package objects

import (
	"bucket-brigade/config"
	"bucket-brigade/models"
	"bucket-brigade/pkg/buckets"
	"bucket-brigade/pkg/contents"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type stubObjectRepository struct {
	getCalls    int
	createCalls int
}

func (r *stubObjectRepository) GetByKeyAndBucket(_ context.Context, _ string, _ *models.Bucket) (*models.Object, error) {
	r.getCalls++
	if r.getCalls == 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &models.Object{
		ID:        99,
		Key:       "key",
		BucketID:  7,
		ContentID: 42,
		Content:   models.ObjectContent{ID: 42},
	}, nil
}

func (r *stubObjectRepository) Create(_ context.Context, _ *models.Object) error {
	r.createCalls++
	return sqlite3.Error{ExtendedCode: sqlite3.ErrConstraintUnique}
}

func (r *stubObjectRepository) Save(_ context.Context, _ *models.Object) error {
	return errors.New("unexpected call to Save")
}

func (r *stubObjectRepository) Delete(_ context.Context, _ *models.Object) error {
	return errors.New("unexpected call to Delete")
}

func (r *stubObjectRepository) WithTx(_ *gorm.DB) ObjectRepository {
	return r
}

func (r *stubObjectRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type stubBucketService struct {
	bucket *models.Bucket
}

func (s *stubBucketService) GetByName(_ context.Context, _ string) (*models.Bucket, error) {
	return s.bucket, nil
}

func (s *stubBucketService) GetOrCreateBucket(_ context.Context, _ string) (*models.Bucket, error) {
	return s.bucket, nil
}

func (s *stubBucketService) WithTx(_ *gorm.DB) buckets.Service {
	return s
}

type stubContentService struct {
	contentResult   *contents.ContentResult
	incrementCalls  int
	decrementCalls  int
	finalizeCalls   int
	cleanupNewCalls int
}

func (s *stubContentService) GetOrCreateContent(_ context.Context, _ io.Reader, _ *models.Bucket) (*contents.ContentResult, error) {
	return s.contentResult, nil
}

func (s *stubContentService) IncrementRefCount(_ context.Context, _ *models.ObjectContent) error {
	s.incrementCalls++
	return nil
}

func (s *stubContentService) DecrementRefCount(_ context.Context, _ *models.ObjectContent) (*contents.PendingContentCleanup, error) {
	s.decrementCalls++
	return nil, nil
}

func (s *stubContentService) CleanupNewContent(_ string) {
	s.cleanupNewCalls++
}

func (s *stubContentService) FinalizeUnreferencedContent(_ context.Context, _ *contents.PendingContentCleanup) {
	s.finalizeCalls++
}

func (s *stubContentService) CleanupZeroRefContents(_ context.Context) {}

func (s *stubContentService) WithTx(_ *gorm.DB) contents.Service {
	return s
}

func TestUploadObjectTreatsDuplicateObjectCreateAsOverwrite(t *testing.T) {
	repo := &stubObjectRepository{}
	bucketService := &stubBucketService{
		bucket: &models.Bucket{ID: 7, Name: "bucket"},
	}
	contentService := &stubContentService{
		contentResult: &contents.ContentResult{
			Content: &models.ObjectContent{ID: 42, BucketID: 7},
			Created: false,
		},
	}

	service := NewObjectService(
		repo,
		&config.Config{},
		bucketService,
		contentService,
		logrus.NewEntry(logrus.New()),
	)

	if err := service.UploadObject(context.Background(), "bucket", "key", strings.NewReader("shared content")); err != nil {
		t.Fatalf("UploadObject returned error: %v", err)
	}

	if repo.createCalls != 1 {
		t.Fatalf("expected exactly one create attempt, got %d", repo.createCalls)
	}
	if repo.getCalls != 2 {
		t.Fatalf("expected duplicate create path to re-read object, got %d lookups", repo.getCalls)
	}
	if contentService.incrementCalls != 0 {
		t.Fatalf("expected no refcount increment when the existing object already points to the same content")
	}
	if contentService.cleanupNewCalls != 0 {
		t.Fatalf("expected no rollback cleanup for reused content")
	}
	if contentService.finalizeCalls != 1 {
		t.Fatalf("expected finalize cleanup to be invoked once after transaction commit")
	}
}
