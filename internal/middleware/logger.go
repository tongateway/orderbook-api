package middleware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

const requestIDKey contextKey = "request_id"

// RequestLogger returns a Gin middleware that logs each HTTP request via slog.
// It captures method, path, status, duration, bytes written and a few useful
// context attributes. If logger is nil, slog.Default() is used.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		start := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// Add request ID to context
		ctx := context.WithValue(c.Request.Context(), requestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		// Log incoming request data
		logIncomingRequest(logger, c, requestID)

		c.Next()

		status := c.Writer.Status()
		if status == 0 {
			status = http.StatusOK
		}

		// Collect errors from context if any
		var errMsg string
		var fullError string
		if err, exists := c.Get("error"); exists {
			if e, ok := err.(error); ok {
				errMsg = e.Error()
				fullError = formatError(e)
			} else if s, ok := err.(string); ok {
				errMsg = s
				fullError = s
			}
		}

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int("bytes", c.Writer.Size()),
			slog.Duration("duration", time.Since(start)),
			slog.String("query", c.Request.URL.RawQuery),
			slog.String("remote_addr", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.String("request_id", requestID),
		}

		if errMsg != "" {
			attrs = append(attrs, slog.String("error", errMsg))
			if fullError != errMsg && fullError != "" {
				attrs = append(attrs, slog.String("error_full", fullError))
			}
		}

		// Convert attrs to key-value pairs for slog
		args := make([]any, 0, len(attrs)*2)
		for _, attr := range attrs {
			args = append(args, attr.Key, attr.Value)
		}

		// Log level based on status code
		if status >= 500 {
			logger.ErrorContext(
				c.Request.Context(),
				"http request",
				args...,
			)
		} else if status >= 400 {
			logger.WarnContext(
				c.Request.Context(),
				"http request",
				args...,
			)
		} else {
			logger.InfoContext(
				c.Request.Context(),
				"http request",
				args...,
			)
		}
	}
}

// RecoveryLogger returns a Gin middleware that recovers from panics and logs them
func RecoveryLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get request ID if available
				requestID, _ := c.Get("request_id")
				requestIDStr := ""
				if id, ok := requestID.(string); ok {
					requestIDStr = id
				}

				// Log panic with stack trace
				logger.ErrorContext(
					c.Request.Context(),
					"panic recovered",
					slog.String("error", fmt.Sprintf("%v", err)),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
					slog.String("query", c.Request.URL.RawQuery),
					slog.String("remote_addr", c.ClientIP()),
					slog.String("user_agent", c.Request.UserAgent()),
					slog.String("request_id", requestIDStr),
					slog.String("stack", getStackTrace()),
				)

				// Set error in context for RequestLogger
				c.Set("error", fmt.Sprintf("panic: %v", err))

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":      "Internal server error",
					"request_id": requestIDStr,
				})
			}
		}()

		c.Next()
	}
}

// getStackTrace returns the current stack trace as a string
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// logIncomingRequest logs incoming request details
func logIncomingRequest(logger *slog.Logger, c *gin.Context, requestID string) {
	// Read request body if available (for any method that might have body)
	var bodyPreview string
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		bodyBytes, err := c.GetRawData()
		if err == nil && len(bodyBytes) > 0 {
			// Restore body for handler
			c.Request.Body = http.MaxBytesReader(nil, io.NopCloser(bytes.NewReader(bodyBytes)), 1<<20)

			// Log body preview (first 1000 chars for better debugging)
			if len(bodyBytes) > 1000 {
				bodyPreview = string(bodyBytes[:1000]) + "..."
			} else {
				bodyPreview = string(bodyBytes)
			}
		}
	}

	// Parse query parameters for GET requests
	var queryParams map[string]string
	if c.Request.URL.RawQuery != "" {
		queryParams = make(map[string]string)
		for k, v := range c.Request.URL.Query() {
			if len(v) > 0 {
				queryParams[k] = v[0]
			}
		}
	}

	// Collect headers (excluding sensitive ones)
	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		lowerKey := strings.ToLower(k)
		// Skip sensitive headers
		if lowerKey != "authorization" && lowerKey != "cookie" && lowerKey != "x-api-key" {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		} else {
			headers[k] = "[REDACTED]"
		}
	}

	// Log incoming request
	attrs := []any{
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"query", c.Request.URL.RawQuery,
		"remote_addr", c.ClientIP(),
		"user_agent", c.Request.UserAgent(),
		"request_id", requestID,
		"content_type", c.Request.Header.Get("Content-Type"),
		"content_length", c.Request.ContentLength,
	}

	if bodyPreview != "" {
		attrs = append(attrs, "body_preview", bodyPreview)
	}

	if len(queryParams) > 0 {
		attrs = append(attrs, "query_params", queryParams)
	}

	if len(headers) > 0 {
		attrs = append(attrs, "headers", headers)
	}

	// Log incoming request at INFO level
	logger.InfoContext(
		c.Request.Context(),
		"incoming request",
		attrs...,
	)
}

// formatError formats error with full details including unwrapped errors
func formatError(err error) string {
	if err == nil {
		return ""
	}

	var result strings.Builder
	result.WriteString(err.Error())

	// Try to unwrap error
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			break
		}
		result.WriteString(" | unwrapped: ")
		result.WriteString(unwrapped.Error())
		err = unwrapped
	}

	return result.String()
}

// FormatErrorFull formats error with full details including stack trace
func FormatErrorFull(err error) string {
	if err == nil {
		return ""
	}

	var result strings.Builder
	result.WriteString(err.Error())

	// Unwrap all errors
	currentErr := err
	for {
		unwrapped := errors.Unwrap(currentErr)
		if unwrapped == nil {
			break
		}
		result.WriteString(fmt.Sprintf(" | unwrapped: %v", unwrapped))
		currentErr = unwrapped
	}

	// Add stack trace for debugging
	buf := make([]byte, 2048)
	n := runtime.Stack(buf, false)
	if n > 0 {
		result.WriteString(fmt.Sprintf(" | stack: %s", string(buf[:n])))
	}

	return result.String()
}

// GormLogger 实现 gorm 日志接口
type GormLogger struct {
	Log           *slog.Logger
	SlowThreshold time.Duration
	TraceKey      string // context 中的 trace key
}

// NewGormLogger  gorm 日志
func NewGormLogger(l *slog.Logger, slowThreshold time.Duration, traceKey string) *GormLogger {
	return &GormLogger{
		Log:           l,
		SlowThreshold: slowThreshold,
		TraceKey:      traceKey,
	}
}
func (g *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	return g
}
func (g *GormLogger) Info(ctx context.Context, s string, i ...any) {
	if g.Log.Enabled(ctx, slog.LevelInfo) {
		g.Log.Info(fmt.Sprintf(s, i...))
	}
}
func (g *GormLogger) Warn(ctx context.Context, s string, i ...any) {
	if g.Log.Enabled(ctx, slog.LevelWarn) {
		g.Log.Warn(fmt.Sprintf(s, i...), g.getTraceAttr(ctx))
	}
}
func (g *GormLogger) Error(ctx context.Context, s string, i ...any) {
	if g.Log.Enabled(ctx, slog.LevelError) {
		g.Log.Error(fmt.Sprintf(s, i...), g.getTraceAttr(ctx))
	}
}

type traceCallbackFn = func() (sql string, rowsAffected int64) // 别名
var attrsSlicePool = sync.Pool{
	New: func() any {
		slice := make([]any, 0, 10) // **注意** 合适的长度
		return &slice
	},
}

func (g *GormLogger) Trace(ctx context.Context, begin time.Time, fc traceCallbackFn, err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	attrsPtr := attrsSlicePool.Get().(*[]any)
	defer attrsSlicePool.Put(attrsPtr)
	*attrsPtr = (*attrsPtr)[:0] // Reset slice
	*attrsPtr = append(*attrsPtr,
		g.getTraceAttr(ctx),
		slog.Int64("latency", elapsed.Milliseconds()),
		slog.Int64("rows", rows),
		slog.String("line", utils.FileWithLineNum()),
		slog.String("sql", sql),
	)
	attrs := *attrsPtr
	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && g.Log.Enabled(ctx, slog.LevelError):
		g.Log.Error("sql trace error", append(attrs, slog.String("error", err.Error()))...)
	case g.SlowThreshold != 0 && elapsed > g.SlowThreshold && g.Log.Enabled(ctx, slog.LevelWarn):
		g.Log.Warn("slow sql", append(attrs, slog.String("slowThreshold", g.SlowThreshold.String()))...)
	default:
		if g.Log.Enabled(ctx, slog.LevelDebug) {
			g.Log.Debug("sql trace", attrs...)
		}
	}
}
func (g *GormLogger) getTraceAttr(ctx context.Context) slog.Attr {
	traceID, _ := ctx.Value(g.TraceKey).(string)
	return slog.String(g.TraceKey, traceID)
}
