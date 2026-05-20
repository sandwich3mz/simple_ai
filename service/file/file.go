package file

import (
	"context"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"simple_ai/common/rag"
	"simple_ai/config"
	"simple_ai/utils"
)

type ragFileIndexer interface {
	IndexFile(ctx context.Context, filePath string) error
}

var (
	newRAGIndexer = func(filename, embeddingModel string) (ragFileIndexer, error) {
		return rag.NewRAGIndexer(filename, embeddingModel)
	}
	deleteRAGIndex       = rag.DeleteIndex
	getRAGEmbeddingModel = func() string {
		return config.GetConfig().RagModelConfig.RagEmbeddingModel
	}
)

func UploadRagFile(username string, file *multipart.FileHeader) (string, error) {
	// 校验文件类型和文件名
	if err := utils.ValidateFile(file); err != nil {
		log.Printf("File validation failed: %v", err)
		return "", err
	}

	// 创建用户目录
	userDir := filepath.Join("uploads", username)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		log.Printf("Failed to create user directory %s: %v", userDir, err)
		return "", err
	}

	oldFiles, err := listKnowledgeFiles(userDir)
	if err != nil {
		log.Printf("Failed to list user directory %s: %v", userDir, err)
		return "", err
	}

	// 生成UUID作为唯一文件名
	uuid := utils.GenerateUUID()

	ext := filepath.Ext(file.Filename)
	filename := uuid + ext
	filePath := filepath.Join(userDir, filename)
	tempFilePath := filepath.Join(userDir, "."+filename+".tmp")

	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		log.Printf("Failed to open uploaded file: %v", err)
		return "", err
	}
	defer src.Close()

	// 创建目标文件
	dst, err := os.Create(tempFilePath)
	if err != nil {
		log.Printf("Failed to create temporary file %s: %v", tempFilePath, err)
		return "", err
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		log.Printf("Failed to copy file content: %v", err)
		os.Remove(tempFilePath)
		return "", err
	}
	if err := dst.Close(); err != nil {
		log.Printf("Failed to close temporary file %s: %v", tempFilePath, err)
		os.Remove(tempFilePath)
		return "", err
	}

	log.Printf("File uploaded successfully: %s", tempFilePath)

	// 创建 RAG 索引器并对文件进行向量化
	indexer, err := newRAGIndexer(filename, getRAGEmbeddingModel())
	if err != nil {
		log.Printf("Failed to create RAG indexer: %v", err)
		os.Remove(tempFilePath)
		return "", err
	}

	// 读取文件内容并创建向量索引
	if err := indexer.IndexFile(context.Background(), tempFilePath); err != nil {
		log.Printf("Failed to index file: %v", err)
		os.Remove(tempFilePath)
		deleteRAGIndex(context.Background(), filename)
		return "", err
	}

	if err := os.Rename(tempFilePath, filePath); err != nil {
		log.Printf("Failed to publish indexed file %s: %v", filePath, err)
		os.Remove(tempFilePath)
		deleteRAGIndex(context.Background(), filename)
		return "", err
	}

	for _, oldFile := range oldFiles {
		oldPath := filepath.Join(userDir, oldFile)
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to delete old file %s: %v", oldPath, err)
			return "", err
		}
		if err := deleteRAGIndex(context.Background(), oldFile); err != nil {
			log.Printf("Failed to delete index for %s: %v", oldFile, err)
			return "", err
		}
	}

	log.Printf("File indexed successfully: %s", filename)
	return filePath, nil
}

func listKnowledgeFiles(userDir string) ([]string, error) {
	files, err := os.ReadDir(userDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	names := make([]string, 0, len(files))
	for _, f := range files {
		if f.IsDir() || len(f.Name()) == 0 || f.Name()[0] == '.' {
			continue
		}
		names = append(names, f.Name())
	}
	return names, nil
}
