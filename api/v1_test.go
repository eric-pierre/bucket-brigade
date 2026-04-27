package api

import (
	apimiddleware "bucket-brigade/api/middleware"
	"bucket-brigade/config"
	"bucket-brigade/dbs"
	"bucket-brigade/models"
	"bucket-brigade/pkg/buckets"
	"bucket-brigade/pkg/contents"
	"bucket-brigade/pkg/objects"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

type problemResponse struct {
	Type       string            `json:"type"`
	Title      string            `json:"title"`
	Status     int               `json:"status"`
	Detail     string            `json:"detail"`
	Instance   string            `json:"instance"`
	Parameters map[string]string `json:"parameters"`
}

func setupTestRouter(cfg *config.Config, db *gorm.DB) *RESTApiV1 {
	bucketRepo := buckets.NewBucketRepository(db)
	contentRepo := contents.NewObjectContentRepository(db)
	objectRepo := objects.NewObjectRepository(db)

	bucketService := buckets.NewBucketService(bucketRepo)
	contentService := contents.NewObjectContentService(contentRepo, cfg)
	contentService.CleanupZeroRefContents()
	objectService := objects.NewObjectService(objectRepo, cfg, bucketService, contentService)

	return NewRESTApiV1(cfg, objectService)
}

func TestAPI(t *testing.T) {
	// Setup
	cfg, err := config.LoadConfig("properties-test")
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	cfg.Database.SQLitePath = "test_brigade.db"
	cfg.Storage.BasePath = "./test_data"
	cfg.Server.MaxUploadBytes = 1024
	db, err := dbs.InitDb(cfg)
	if err != nil {
		t.Fatalf("InitDb: %v", err)
	}
	if err := dbs.Migrate(cfg, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		os.Remove(cfg.Database.SQLitePath)
		os.RemoveAll(cfg.Storage.BasePath)
	}()

	restApi := setupTestRouter(cfg, db)
	router := restApi.router

	bucket := "testbucket"
	objectId := "testobject"
	content := []byte("hello world")

	// 1. Upload
	req, _ := http.NewRequest("PUT", "/objects/"+bucket+"/"+objectId, bytes.NewBuffer(content))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]string
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, objectId, resp["id"])

	// 2. Download
	req, _ = http.NewRequest("GET", "/objects/"+bucket+"/"+objectId, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rr.Header().Get("Content-Type"))
	assert.Equal(t, "11", rr.Header().Get("Content-Length"))
	assert.Equal(t, content, rr.Body.Bytes())

	// 3. Download Non-existent
	req, _ = http.NewRequest("GET", "/objects/"+bucket+"/nonexistent", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, apimiddleware.ProblemJSONContentType, rr.Header().Get("Content-Type"))

	var notFound problemResponse
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &notFound))
	assert.Equal(t, "https://bucket-brigade.dev/problems/object-not-found", notFound.Type)
	assert.Equal(t, "Not Found", notFound.Title)
	assert.Equal(t, http.StatusNotFound, notFound.Status)
	assert.Equal(t, "object not found", notFound.Detail)
	assert.Equal(t, "/objects/"+bucket+"/nonexistent", notFound.Instance)
	assert.Equal(t, bucket, notFound.Parameters["bucket"])
	assert.Equal(t, "nonexistent", notFound.Parameters["objectId"])

	// 4. Delete
	req, _ = http.NewRequest("DELETE", "/objects/"+bucket+"/"+objectId, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// 5. Download after delete
	req, _ = http.NewRequest("GET", "/objects/"+bucket+"/"+objectId, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	// 6. Delete Non-existent
	req, _ = http.NewRequest("DELETE", "/objects/"+bucket+"/nonexistent", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var deleteNotFound problemResponse
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &deleteNotFound))
	assert.Equal(t, "object not found", deleteNotFound.Detail)
	assert.Equal(t, bucket, deleteNotFound.Parameters["bucket"])
	assert.Equal(t, "nonexistent", deleteNotFound.Parameters["objectId"])

	// 7. Upload too large
	tooLarge := bytes.Repeat([]byte("a"), 2048)
	req, _ = http.NewRequest("PUT", "/objects/"+bucket+"/too-large", bytes.NewBuffer(tooLarge))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Equal(t, apimiddleware.ProblemJSONContentType, rr.Header().Get("Content-Type"))

	var tooLargeResp problemResponse
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &tooLargeResp))
	assert.Equal(t, "https://bucket-brigade.dev/problems/request-too-large", tooLargeResp.Type)
	assert.Equal(t, "Request Entity Too Large", tooLargeResp.Title)
	assert.Equal(t, http.StatusRequestEntityTooLarge, tooLargeResp.Status)
	assert.Equal(t, "request body exceeds configured upload limit", tooLargeResp.Detail)
	assert.Equal(t, bucket, tooLargeResp.Parameters["bucket"])
	assert.Equal(t, "too-large", tooLargeResp.Parameters["objectId"])

	// 8. Invalid bucket
	invalidBucket := strings.Repeat("b", 256)
	req, _ = http.NewRequest("GET", "/objects/"+invalidBucket+"/"+objectId, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, apimiddleware.ProblemJSONContentType, rr.Header().Get("Content-Type"))

	var invalidBucketResp problemResponse
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &invalidBucketResp))
	assert.Equal(t, "bucket must be at most 255 characters", invalidBucketResp.Detail)
	assert.Equal(t, invalidBucket, invalidBucketResp.Parameters["bucket"])
	assert.Equal(t, objectId, invalidBucketResp.Parameters["objectId"])

	// 9. Invalid objectId
	invalidObjectId := strings.Repeat("o", 256)
	req, _ = http.NewRequest("GET", "/objects/"+bucket+"/"+invalidObjectId, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var invalidObjectResp problemResponse
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &invalidObjectResp))
	assert.Equal(t, "objectId must be at most 255 characters", invalidObjectResp.Detail)
	assert.Equal(t, bucket, invalidObjectResp.Parameters["bucket"])
	assert.Equal(t, invalidObjectId, invalidObjectResp.Parameters["objectId"])
}

func TestReferenceCounting(t *testing.T) {
	// Setup
	cfg, err := config.LoadConfig("properties-test")
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	cfg.Database.SQLitePath = "test_brigade.db"
	cfg.Storage.BasePath = "./test_data"
	cfg.Server.MaxUploadBytes = 1024
	db, err := dbs.InitDb(cfg)
	if err != nil {
		t.Fatalf("InitDb: %v", err)
	}
	if err := dbs.Migrate(cfg, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		os.Remove(cfg.Database.SQLitePath)
		os.RemoveAll(cfg.Storage.BasePath)
	}()

	router := setupTestRouter(cfg, db).router

	bucket := "b1"
	content1 := []byte("shared content")
	content2 := []byte("unique content")

	// Upload obj1 and obj2 with same content
	req1, _ := http.NewRequest("PUT", "/objects/"+bucket+"/obj1", bytes.NewBuffer(content1))
	router.ServeHTTP(httptest.NewRecorder(), req1)
	req2, _ := http.NewRequest("PUT", "/objects/"+bucket+"/obj2", bytes.NewBuffer(content1))
	router.ServeHTTP(httptest.NewRecorder(), req2)

	// Verify both exist and share same content in DB
	var objs []models.Object
	db.Find(&objs)
	assert.Equal(t, 2, len(objs))
	assert.Equal(t, objs[0].ContentId, objs[1].ContentId)

	var contentRec models.ObjectContent
	db.First(&contentRec, objs[0].ContentId)
	assert.Equal(t, int64(2), contentRec.RefCount)

	// Update obj1 to new content
	req3, _ := http.NewRequest("PUT", "/objects/"+bucket+"/obj1", bytes.NewBuffer(content2))
	router.ServeHTTP(httptest.NewRecorder(), req3)

	// Verify obj1 now has different content, and old content refcount decreased
	var obj1 models.Object
	db.Where("key = ?", "obj1").First(&obj1)
	assert.NotEqual(t, objs[0].ContentId, obj1.ContentId)

	db.First(&contentRec, objs[0].ContentId)
	assert.Equal(t, int64(1), contentRec.RefCount)

	// Delete obj2
	req4, _ := http.NewRequest("DELETE", "/objects/"+bucket+"/obj2", nil)
	router.ServeHTTP(httptest.NewRecorder(), req4)

	// Verify old content is deleted from DB because refcount reached 0
	var oldContent models.ObjectContent
	err = db.First(&oldContent, objs[0].ContentId).Error
	assert.Error(t, err) // Should be record not found

	// Verify file is gone from disk
	_, err = os.Stat(contentRec.Path)
	assert.True(t, os.IsNotExist(err))
}
