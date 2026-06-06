package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var rxUnicodeEscape = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

func decodeUnicodeEscapes(val string) string {
	return rxUnicodeEscape.ReplaceAllStringFunc(val, func(s string) string {
		r, err := strconv.ParseUint(s[2:], 16, 64)
		if err != nil {
			return s
		}
		return string(rune(r))
	})
}

func formatJSON(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, data, "", "    ")
	if err != nil {
		// Not a valid JSON, return as-is after stripping newlines for clean printing
		return strings.ReplaceAll(string(data), "\n", " ")
	}
	return decodeUnicodeEscapes(prettyJSON.String())
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// DebugLogger acts as an AOP interceptor to log request input parameters and response return values.
func DebugLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Read and log Request Body (Input)
		var reqBody []byte
		if c.Request.Body != nil {
			var err error
			reqBody, err = io.ReadAll(c.Request.Body)
			if err == nil {
				// Restore c.Request.Body reader for the next handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			}
		}

		// Log request headers (selective)
		pixivUserHeader := c.GetHeader("X-Pixiv-User-Id")

		// Print request details
		reqBodyStr := formatJSON(reqBody)
		slog.Debug("Request Details",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"pixiv_user_id", pixivUserHeader,
			"body", reqBodyStr,
		)

		// 2. Intercept and log Response Body (Output)
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		// Call next handlers
		c.Next()

		// Print response details
		respBodyStr := formatJSON(blw.body.Bytes())
		// Limit logging response body if too long (e.g. max 2000 characters)
		if len(respBodyStr) > 2000 {
			respBodyStr = respBodyStr[:2000] + "... (truncated)"
		}

		slog.Debug("Response Details",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"body", respBodyStr,
		)
	}
}
