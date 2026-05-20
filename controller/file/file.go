package file

import (
	"log"
	"net/http"
	"simple_ai/common/code"
	"simple_ai/controller"
	"simple_ai/service/file"
	"simple_ai/utils"

	"github.com/gin-gonic/gin"
)

var uploadRagFileService = file.UploadRagFile

type (
	UploadFileResponse struct {
		FilePath string `json:"file_path,omitempty"`
		controller.Response
	}
)

func UploadRagFile(c *gin.Context) {
	res := new(UploadFileResponse)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, utils.MaxUploadRequestSize)

	uploadedFile, err := c.FormFile("file")
	if err != nil {
		log.Println("FormFile fail ", err)
		c.JSON(http.StatusOK, res.CodeOf(code.CodeInvalidParams))
		return
	}

	username := c.GetString("userName")
	if username == "" {
		log.Println("Username not found in context")
		c.JSON(http.StatusOK, res.CodeOf(code.CodeInvalidToken))
		return
	}

	//indexer 会在 service 层根据实际文件名创建
	filePath, err := uploadRagFileService(username, uploadedFile)
	if err != nil {
		log.Println("UploadFile fail ", err)
		c.JSON(http.StatusOK, res.CodeOf(code.CodeServerBusy))
		return
	}

	res.Success()
	res.FilePath = filePath
	c.JSON(http.StatusOK, res)
}
