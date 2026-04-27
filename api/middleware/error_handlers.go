package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	problem "github.com/neocotic/go-problem"
	"github.com/sirupsen/logrus"
)

const ProblemJSONContentType = problem.ContentTypeJSON

var (
	notFoundProblemType = problem.Type{
		Status: http.StatusNotFound,
		Title:  http.StatusText(http.StatusNotFound),
		URI:    "https://bucket-brigade.dev/problems/object-not-found",
	}
	badRequestProblemType = problem.Type{
		Status: http.StatusBadRequest,
		Title:  http.StatusText(http.StatusBadRequest),
		URI:    "https://bucket-brigade.dev/problems/invalid-request",
	}
	internalProblemType = problem.Type{
		Status: http.StatusInternalServerError,
		Title:  http.StatusText(http.StatusInternalServerError),
		URI:    "https://bucket-brigade.dev/problems/internal-error",
	}
	requestTooLargeProblemType = problem.Type{
		Status: http.StatusRequestEntityTooLarge,
		Title:  http.StatusText(http.StatusRequestEntityTooLarge),
		URI:    "https://bucket-brigade.dev/problems/request-too-large",
	}
)

func newProblem(defType problem.Type, detail, instance string, parameters map[string]string) *problem.Problem {
	opts := []problem.Option{
		problem.WithDetail(detail),
		problem.WithInstance(instance),
	}
	if len(parameters) > 0 {
		opts = append(opts, problem.WithExtension("parameters", parameters))
	}
	return defType.New(opts...)
}

func NewNotFoundError(detail, instance string, parameters map[string]string) error {
	return newProblem(notFoundProblemType, detail, instance, parameters)
}

func NewBadRequestError(detail, instance string, parameters map[string]string) error {
	return newProblem(badRequestProblemType, detail, instance, parameters)
}

func NewInternalError(detail, instance string, parameters map[string]string) error {
	return newProblem(internalProblemType, detail, instance, parameters)
}

func NewRequestTooLargeError(detail, instance string, parameters map[string]string) error {
	return newProblem(requestTooLargeProblemType, detail, instance, parameters)
}

func writeProblem(c *gin.Context, prob *problem.Problem) {
	if err := problem.DefaultGenerator.WriteProblemJSON(prob, c.Writer, c.Request, problem.WriteOptions{
		ContentType: ProblemJSONContentType,
		LogDisabled: true,
	}); err != nil {
		logrus.Errorf("failed to write RFC 9457 problem response: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Abort()
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			if prob, ok := problem.As(err.Err); ok {
				writeProblem(c, prob)
				return
			}

			logrus.Errorf("unhandled API error: %v", err.Err)
			writeProblem(c, newProblem(internalProblemType, "internal server error", c.Request.URL.Path, nil))
		}
	}
}
