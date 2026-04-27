package api

import (
	apimiddleware "bucket-brigade/api/middleware"
	"bucket-brigade/config"
	"bucket-brigade/models"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type RESTApiV1 struct {
	router        *gin.Engine
	cfg           *config.Config
	objectService ObjectService
}

type ObjectService interface {
	UploadObject(bucketName, objectId string, body io.Reader) error
	DownloadObject(bucketName, objectId string) (*models.Object, io.ReadSeekCloser, error)
	DeleteObject(bucketName, objectId string) error
}

func NewRESTApiV1(cfg *config.Config, objectService ObjectService) *RESTApiV1 {
	gin.SetMode(gin.ReleaseMode)

	api := &RESTApiV1{
		router:        gin.New(),
		cfg:           cfg,
		objectService: objectService,
	}
	api.router.Use(gin.Recovery())
	api.router.Use(apimiddleware.ErrorHandler())

	api.registerRoutes()
	return api
}

func (api *RESTApiV1) registerRoutes() {
	objectRoutes := api.router.Group("/objects/:bucket/:objectId")
	objectRoutes.Use(apimiddleware.ValidateObjectParams(api.cfg))
	objectRoutes.PUT("", api.UploadObject)
	objectRoutes.GET("", api.DownloadObject)
	objectRoutes.DELETE("", api.DeleteObject)
}

func (api *RESTApiV1) Start(addr string) error {
	timeoutConfig := api.cfg.Server.Timeouts
	srv := &http.Server{
		Addr:         addr,
		Handler:      api.router,
		ReadTimeout:  timeoutConfig.Read,
		WriteTimeout: timeoutConfig.Write,
		IdleTimeout:  timeoutConfig.Idle,
	}

	return srv.ListenAndServe()
}

func objectParameters(bucket, objectId string) map[string]string {
	return map[string]string{
		"bucket":   bucket,
		"objectId": objectId,
	}
}

func problemInstance(c *gin.Context) string {
	return c.Request.URL.Path
}

func (api *RESTApiV1) UploadObject(c *gin.Context) {
	bucket := c.Param("bucket")
	objectId := c.Param("objectId")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, api.cfg.Server.MaxUploadBytes)

	if err := api.objectService.UploadObject(bucket, objectId, c.Request.Body); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.Error(apimiddleware.NewRequestTooLargeError("request body exceeds configured upload limit", problemInstance(c), objectParameters(bucket, objectId)))
			return
		}
		c.Error(apimiddleware.NewInternalError("object upload failed", problemInstance(c), objectParameters(bucket, objectId)))
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": objectId})
}

func (api *RESTApiV1) DownloadObject(c *gin.Context) {
	bucket := c.Param("bucket")
	objectId := c.Param("objectId")

	object, reader, err := api.objectService.DownloadObject(bucket, objectId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Error(apimiddleware.NewNotFoundError("object not found", problemInstance(c), objectParameters(bucket, objectId)))
			return
		}
		c.Error(apimiddleware.NewInternalError("object download failed", problemInstance(c), objectParameters(bucket, objectId)))
		return
	}
	defer reader.Close()

	header := make([]byte, 512)
	n, err := reader.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		c.Error(apimiddleware.NewInternalError("object download failed", problemInstance(c), objectParameters(bucket, objectId)))
		return
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		c.Error(apimiddleware.NewInternalError("object download failed", problemInstance(c), objectParameters(bucket, objectId)))
		return
	}

	c.Header("Content-Type", http.DetectContentType(header[:n]))
	c.Header("Content-Length", fmt.Sprintf("%d", object.Content.Size))
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, reader); err != nil {
		logrus.Errorf("stream copy failed for %s/%s: %v", bucket, objectId, err)
	}
}

func (api *RESTApiV1) DeleteObject(c *gin.Context) {
	bucket := c.Param("bucket")
	objectId := c.Param("objectId")

	if err := api.objectService.DeleteObject(bucket, objectId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Error(apimiddleware.NewNotFoundError("object not found", problemInstance(c), objectParameters(bucket, objectId)))
			return
		}
		c.Error(apimiddleware.NewInternalError("object delete failed", problemInstance(c), objectParameters(bucket, objectId)))
		return
	}

	c.Status(http.StatusOK)
}
