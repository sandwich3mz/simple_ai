package utils

import (
	"mime/multipart"
	"testing"
)

func TestValidateFileRejectsFilesLargerThanFiveMiB(t *testing.T) {
	err := ValidateFile(&multipart.FileHeader{
		Filename: "knowledge.md",
		Size:     MaxUploadFileSize + 1,
	})
	if err == nil {
		t.Fatal("ValidateFile returned nil for file larger than max size")
	}
}

func TestValidateFileAllowsMarkdownAndTextAtMaxSize(t *testing.T) {
	for _, filename := range []string{"knowledge.md", "knowledge.txt"} {
		t.Run(filename, func(t *testing.T) {
			err := ValidateFile(&multipart.FileHeader{
				Filename: filename,
				Size:     MaxUploadFileSize,
			})
			if err != nil {
				t.Fatalf("ValidateFile(%q) returned error: %v", filename, err)
			}
		})
	}
}
