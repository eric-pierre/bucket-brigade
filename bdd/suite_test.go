package bdd_test

import (
	"bucket-brigade/api"
	"bucket-brigade/config"
	"bucket-brigade/dbs"
	"bucket-brigade/pkg/buckets"
	"bucket-brigade/pkg/contents"
	"bucket-brigade/pkg/objects"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/sirupsen/logrus"
)

type noopHealthService struct{}

func (noopHealthService) Ping() error { return nil }

type scenarioCtx struct {
	handler     http.Handler
	response    *httptest.ResponseRecorder
	tmpDir      string
	storagePath string
}

func (s *scenarioCtx) setup() error {
	tmpDir, err := os.MkdirTemp("", "bucket-brigade-bdd-*")
	if err != nil {
		return err
	}
	s.tmpDir = tmpDir

	cfg, err := config.LoadConfig("properties-test")
	if err != nil {
		return err
	}
	cfg.Database.SQLitePath = filepath.Join(tmpDir, "test.db")
	cfg.Storage.BasePath = filepath.Join(tmpDir, "data")
	s.storagePath = cfg.Storage.BasePath

	db, err := dbs.InitDb(cfg)
	if err != nil {
		return err
	}
	if err := dbs.Migrate(cfg, db); err != nil {
		return err
	}

	bucketRepo := buckets.NewBucketRepository(db)
	contentRepo := contents.NewObjectContentRepository(db)
	objectRepo := objects.NewObjectRepository(db)

	logger := logrus.NewEntry(logrus.New())

	bucketService := buckets.NewBucketService(bucketRepo, logger)
	contentService, err := contents.NewObjectContentService(contentRepo, cfg, logger)
	if err != nil {
		return err
	}
	objectService := objects.NewObjectService(objectRepo, cfg, bucketService, contentService, logger)

	s.handler = api.NewRESTApiV1(cfg, objectService, noopHealthService{}, logger).Handler()
	return nil
}

func (s *scenarioCtx) teardown() error {
	if s.tmpDir != "" {
		return os.RemoveAll(s.tmpDir)
	}
	return nil
}

func (s *scenarioCtx) do(method, path string, body []byte) {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, path, bodyReader)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	s.response = rr
}

// Given steps

func (s *scenarioCtx) aCleanStorageEnvironment() error {
	return nil // setup is handled by the Before hook
}

func (s *scenarioCtx) givenObjectExists(key, bucket, content string) error {
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/objects/"+bucket+"/"+key, bytes.NewBufferString(content))
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		return fmt.Errorf("setup: PUT %s/%s returned %d", bucket, key, rr.Code)
	}
	return nil
}

// When steps

func (s *scenarioCtx) iUploadObject(key, bucket, content string) error {
	s.do("PUT", "/objects/"+bucket+"/"+key, []byte(content))
	return nil
}

func (s *scenarioCtx) iDownloadObject(key, bucket string) error {
	s.do("GET", "/objects/"+bucket+"/"+key, nil)
	return nil
}

func (s *scenarioCtx) iDeleteObject(key, bucket string) error {
	s.do("DELETE", "/objects/"+bucket+"/"+key, nil)
	return nil
}

func (s *scenarioCtx) iUploadObjectToLongBucketWithContent(key, content string) error {
	s.do("PUT", "/objects/"+strings.Repeat("b", 256)+"/"+key, []byte(content))
	return nil
}

func (s *scenarioCtx) iUploadLongKeyObjectToBucketWithContent(bucket, content string) error {
	s.do("PUT", "/objects/"+bucket+"/"+strings.Repeat("o", 256), []byte(content))
	return nil
}

func (s *scenarioCtx) iUploadObjectWithBodyExceedingLimit(key, bucket string) error {
	s.do("PUT", "/objects/"+bucket+"/"+key, bytes.Repeat([]byte("a"), 2048))
	return nil
}

func (s *scenarioCtx) iUploadObjectWithEmptyBody(key, bucket string) error {
	s.do("PUT", "/objects/"+bucket+"/"+key, []byte{})
	return nil
}

func (s *scenarioCtx) iMakeARequestTo(method, path string) error {
	s.do(method, path, nil)
	return nil
}

// Then steps

func (s *scenarioCtx) theResponseStatusShouldBe(expected int) error {
	if s.response.Code != expected {
		return fmt.Errorf("expected status %d, got %d (body: %s)", expected, s.response.Code, s.response.Body.String())
	}
	return nil
}

func (s *scenarioCtx) theResponseBodyShouldBe(expected string) error {
	if actual := s.response.Body.String(); actual != expected {
		return fmt.Errorf("expected body %q, got %q", expected, actual)
	}
	return nil
}

func (s *scenarioCtx) theResponseBodyShouldBeEmpty() error {
	if actual := s.response.Body.String(); actual != "" {
		return fmt.Errorf("expected empty body, got %q", actual)
	}
	return nil
}

func (s *scenarioCtx) theResponseBodyShouldContainKeyWithValue(key, expected string) error {
	var m map[string]string
	if err := json.Unmarshal(s.response.Body.Bytes(), &m); err != nil {
		return fmt.Errorf("parse response JSON: %w", err)
	}
	if actual := m[key]; actual != expected {
		return fmt.Errorf("expected %s=%q, got %q", key, expected, actual)
	}
	return nil
}

func (s *scenarioCtx) theContentLengthHeaderShouldBe(expected string) error {
	if actual := s.response.Header().Get("Content-Length"); actual != expected {
		return fmt.Errorf("expected Content-Length %q, got %q", expected, actual)
	}
	return nil
}

func (s *scenarioCtx) theProblemTypeShouldBe(expected string) error {
	var prob struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(s.response.Body.Bytes(), &prob); err != nil {
		return fmt.Errorf("parse problem JSON: %w", err)
	}
	if prob.Type != expected {
		return fmt.Errorf("expected problem type %q, got %q", expected, prob.Type)
	}
	return nil
}

func (s *scenarioCtx) theDataFolderShouldContainNContentFiles(expected int) error {
	contentsDir := filepath.Join(s.storagePath, "contents")
	entries, err := os.ReadDir(contentsDir)
	if err != nil {
		if os.IsNotExist(err) && expected == 0 {
			return nil
		}
		return fmt.Errorf("read contents dir: %w", err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	if count != expected {
		return fmt.Errorf("expected %d content file(s) in data folder, got %d", expected, count)
	}
	return nil
}

func (s *scenarioCtx) theProblemDetailShouldBe(expected string) error {
	var prob struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(s.response.Body.Bytes(), &prob); err != nil {
		return fmt.Errorf("parse problem JSON: %w", err)
	}
	if prob.Detail != expected {
		return fmt.Errorf("expected problem detail %q, got %q", expected, prob.Detail)
	}
	return nil
}

func InitializeScenario(sc *godog.ScenarioContext) {
	s := &scenarioCtx{}

	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		return ctx, s.setup()
	})

	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		return ctx, s.teardown()
	})

	sc.Step(`^a clean storage environment$`, s.aCleanStorageEnvironment)
	sc.Step(`^object "([^"]+)" in bucket "([^"]+)" contains "([^"]+)"$`, s.givenObjectExists)
	sc.Step(`^I upload object "([^"]+)" to bucket "([^"]+)" with content "([^"]+)"$`, s.iUploadObject)
	sc.Step(`^I download object "([^"]+)" from bucket "([^"]+)"$`, s.iDownloadObject)
	sc.Step(`^I delete object "([^"]+)" from bucket "([^"]+)"$`, s.iDeleteObject)
	sc.Step(`^I upload object "([^"]+)" to a bucket with a name of 256 characters with content "([^"]+)"$`, s.iUploadObjectToLongBucketWithContent)
	sc.Step(`^I upload an object with a key of 256 characters to bucket "([^"]+)" with content "([^"]+)"$`, s.iUploadLongKeyObjectToBucketWithContent)
	sc.Step(`^I upload object "([^"]+)" to bucket "([^"]+)" with a body exceeding the size limit$`, s.iUploadObjectWithBodyExceedingLimit)
	sc.Step(`^I upload object "([^"]+)" to bucket "([^"]+)" with an empty body$`, s.iUploadObjectWithEmptyBody)
	sc.Step(`^I make a (GET|PUT|DELETE) request to "([^"]+)"$`, s.iMakeARequestTo)
	sc.Step(`^the response status should be (\d+)$`, s.theResponseStatusShouldBe)
	sc.Step(`^the response body should be "([^"]+)"$`, s.theResponseBodyShouldBe)
	sc.Step(`^the response body should be empty$`, s.theResponseBodyShouldBeEmpty)
	sc.Step(`^the response body should contain key "([^"]+)" with value "([^"]+)"$`, s.theResponseBodyShouldContainKeyWithValue)
	sc.Step(`^the Content-Length header should be "([^"]+)"$`, s.theContentLengthHeaderShouldBe)
	sc.Step(`^the data folder should contain (\d+) content files?$`, s.theDataFolderShouldContainNContentFiles)
	sc.Step(`^the problem type should be "([^"]+)"$`, s.theProblemTypeShouldBe)
	sc.Step(`^the problem detail should be "([^"]+)"$`, s.theProblemDetailShouldBe)
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("non-zero exit code")
	}
}
