package file

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type failingRAGIndexer struct{}

func (failingRAGIndexer) IndexFile(ctx context.Context, filePath string) error {
	return errors.New("index failed")
}

func multipartFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}
	return req.MultipartForm.File["file"][0]
}

func TestUploadRagFileKeepsExistingKnowledgeBaseWhenNewIndexFails(t *testing.T) {
	t.Chdir(t.TempDir())

	userDir := filepath.Join("uploads", "alice")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	oldFile := filepath.Join(userDir, "old.txt")
	if err := os.WriteFile(oldFile, []byte("old knowledge"), 0644); err != nil {
		t.Fatalf("WriteFile old knowledge: %v", err)
	}

	originalNewIndexer := newRAGIndexer
	originalDeleteIndex := deleteRAGIndex
	originalEmbeddingModel := getRAGEmbeddingModel
	defer func() {
		newRAGIndexer = originalNewIndexer
		deleteRAGIndex = originalDeleteIndex
		getRAGEmbeddingModel = originalEmbeddingModel
	}()

	getRAGEmbeddingModel = func() string {
		return "test-embedding"
	}
	newRAGIndexer = func(filename, embeddingModel string) (ragFileIndexer, error) {
		return failingRAGIndexer{}, nil
	}
	var deletedIndexes []string
	deleteRAGIndex = func(ctx context.Context, filename string) error {
		deletedIndexes = append(deletedIndexes, filename)
		return nil
	}

	_, err := UploadRagFile("alice", multipartFileHeader(t, "new.txt", []byte("new knowledge")))
	if err == nil {
		t.Fatal("UploadRagFile returned nil error when indexing failed")
	}
	if _, err := os.Stat(oldFile); err != nil {
		t.Fatalf("old knowledge file was not preserved: %v", err)
	}
	for _, deleted := range deletedIndexes {
		if deleted == "old.txt" {
			t.Fatalf("deleted old index before successful replacement: %v", deletedIndexes)
		}
	}
}
