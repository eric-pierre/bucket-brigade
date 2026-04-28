package api

import (
	apimiddleware "bucket-brigade/api/middleware"
	"bucket-brigade/config"
	"bucket-brigade/models"
	"bucket-brigade/pkg/observability"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"gorm.io/gorm"
)

type RESTApiV1 struct {
	router        *gin.Engine
	srv           *http.Server
	cfg           *config.Config
	objectService ObjectService
	healthService HealthService
	logger        *logrus.Entry
}

type ObjectService interface {
	UploadObject(ctx context.Context, bucketName, objectId string, body io.Reader) error
	DownloadObject(ctx context.Context, bucketName, objectId string) (*models.Object, io.ReadSeekCloser, error)
	DeleteObject(ctx context.Context, bucketName, objectId string) error
}

type HealthService interface {
	Ping() error
}

func NewRESTApiV1(cfg *config.Config, objectService ObjectService, healthService HealthService, logger *logrus.Entry) *RESTApiV1 {
	gin.SetMode(gin.ReleaseMode)

	api := &RESTApiV1{
		router:        gin.New(),
		cfg:           cfg,
		objectService: objectService,
		healthService: healthService,
		logger:        logger,
	}
	api.router.RedirectTrailingSlash = false
	api.router.Use(gin.Recovery())
	api.router.Use(otelgin.Middleware(observability.ServiceName))
	api.router.Use(apimiddleware.Observability(logger))
	api.router.Use(apimiddleware.Prometheus())
	api.router.Use(apimiddleware.ErrorHandler(logger))

	api.router.GET("/livez", api.livenessProbe)
	api.router.GET("/readyz", api.readinessProbe)
	api.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	RegisterHandlersWithOptions(api.router, api, GinServerOptions{
		Middlewares: []MiddlewareFunc{MiddlewareFunc(apimiddleware.ValidateObjectParams(cfg))},
	})
	return api
}

func (api *RESTApiV1) Handler() http.Handler {
	return api.router
}

func (api *RESTApiV1) Start(addr string) error {
	timeoutConfig := api.cfg.Server.Timeouts
	api.srv = &http.Server{
		Addr:         addr,
		Handler:      api.router,
		ReadTimeout:  timeoutConfig.Read,
		WriteTimeout: timeoutConfig.Write,
		IdleTimeout:  timeoutConfig.Idle,
	}
	return api.srv.ListenAndServe()
}

func (api *RESTApiV1) Shutdown(ctx context.Context) error {
	return api.srv.Shutdown(ctx)
}

func problemInstance(c *gin.Context) string {
	return c.Request.URL.Path
}

func objectParameters(bucket, objectId string) map[string]string {
	return map[string]string{
		"bucket":   bucket,
		"objectId": objectId,
	}
}

func (api *RESTApiV1) UploadObject(c *gin.Context, bucket, objectId string) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, api.cfg.Server.MaxUploadBytes)

	if err := api.objectService.UploadObject(c.Request.Context(), bucket, objectId, c.Request.Body); err != nil {
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

func (api *RESTApiV1) DownloadObject(c *gin.Context, bucket, objectId string) {
	object, reader, err := api.objectService.DownloadObject(c.Request.Context(), bucket, objectId)
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
	c.Header("Content-Length", strconv.FormatInt(object.Content.Size, 10))
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, reader); err != nil {
		api.logger.WithFields(logrus.Fields{
			"bucket":    bucket,
			"object_id": objectId,
			"error":     err,
		}).Error("stream copy failed")
	}
}

func (api *RESTApiV1) livenessProbe(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (api *RESTApiV1) readinessProbe(c *gin.Context) {
	if err := api.healthService.Ping(); err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	c.Status(http.StatusOK)
}

func (api *RESTApiV1) DeleteObject(c *gin.Context, bucket, objectId string) {
	if err := api.objectService.DeleteObject(c.Request.Context(), bucket, objectId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Error(apimiddleware.NewNotFoundError("object not found", problemInstance(c), objectParameters(bucket, objectId)))
			return
		}
		c.Error(apimiddleware.NewInternalError("object delete failed", problemInstance(c), objectParameters(bucket, objectId)))
		return
	}

	c.Status(http.StatusOK)
}
