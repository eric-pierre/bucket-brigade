package contents

import (
	"bucket-brigade/models"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type stubObjectContentRepository struct {
	existingContent *models.ObjectContent
	createErr       error
	createCalls     int
}

func (r *stubObjectContentRepository) GetBySha256AndBucket(_ context.Context, _ string, _ uint) (*models.ObjectContent, error) {
	if r.createCalls == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	if r.existingContent == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.existingContent, nil
}

func (r *stubObjectContentRepository) GetOne(_ context.Context, _ uint) (*models.ObjectContent, error) {
	return nil, errors.New("unexpected call to GetOne")
}

func (r *stubObjectContentRepository) ListByRefCount(_ context.Context, _ int64) ([]models.ObjectContent, error) {
	return nil, errors.New("unexpected call to ListByRefCount")
}

func (r *stubObjectContentRepository) Lock(_ context.Context, _ *models.ObjectContent) (*models.ObjectContent, error) {
	return nil, errors.New("unexpected call to Lock")
}

func (r *stubObjectContentRepository) Create(_ context.Context, _ *models.ObjectContent) error {
	r.createCalls++
	return r.createErr
}

func (r *stubObjectContentRepository) Update(_ context.Context, _ *models.ObjectContent) error {
	return errors.New("unexpected call to Update")
}

func (r *stubObjectContentRepository) Delete(_ context.Context, _ *models.ObjectContent) error {
	return errors.New("unexpected call to Delete")
}

func (r *stubObjectContentRepository) WithTx(_ *gorm.DB) ObjectContentRepository {
	return r
}

func TestGetOrCreateContentReturnsExistingContentAfterUniqueConstraintRace(t *testing.T) {
	t.Helper()

	contentsDir := t.TempDir()
	repo := &stubObjectContentRepository{
		existingContent: &models.ObjectContent{
			ID:       42,
			BucketID: 7,
			Sha256:   "existing",
			Path:     filepath.Join(contentsDir, "existing"),
		},
		createErr: sqlite3.Error{ExtendedCode: sqlite3.ErrConstraintUnique},
	}
	service := &ObjectContentService{
		repo:        repo,
		contentsDir: contentsDir,
		logger:      logrus.NewEntry(logrus.New()),
	}

	result, err := service.GetOrCreateContent(context.Background(), strings.NewReader("shared content"), &models.Bucket{ID: 7})
	if err != nil {
		t.Fatalf("GetOrCreateContent returned error: %v", err)
	}

	if result.Created {
		t.Fatalf("expected existing content result after unique constraint race")
	}
	if result.Content != repo.existingContent {
		t.Fatalf("expected existing content to be returned")
	}
}
