package file

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"simple_ai/common/code"
	"simple_ai/controller"
	"simple_ai/utils"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUploadRagFileRejectsOversizedRequestBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalUpload := uploadRagFileService
	defer func() { uploadRagFileService = originalUpload }()

	called := false
	uploadRagFileService = func(username string, file *multipart.FileHeader) (string, error) {
		called = true
		return "", nil
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "large.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("a"), int(utils.MaxUploadRequestSize)+1)); err != nil {
		t.Fatalf("write multipart body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/file/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	r := gin.New()
	r.POST("/api/v1/file/upload", func(c *gin.Context) {
		c.Set("userName", "alice")
		UploadRagFile(c)
	})
	r.ServeHTTP(rec, req)

	if called {
		t.Fatal("upload service was called for an oversized request")
	}
	if got := rec.Code; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	var res controller.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if res.StatusCode != code.CodeInvalidParams {
		t.Fatalf("status_code = %d, want %d", res.StatusCode, code.CodeInvalidParams)
	}
}
