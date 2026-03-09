package service

import (
	"fmt"
	"mime/multipart"
	"os"
	"sort"
	"strconv"
	"strings"

	errs "github.com/UnicomAI/wanwu/api/proto/err-code"
	"github.com/UnicomAI/wanwu/internal/bff-service/model/request"
	"github.com/UnicomAI/wanwu/internal/bff-service/model/response"
	grpc_util "github.com/UnicomAI/wanwu/pkg/grpc-util"
	"github.com/UnicomAI/wanwu/pkg/log"
	"github.com/UnicomAI/wanwu/pkg/minio"
	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/gin-gonic/gin"
)

const (
	FileUploadCheckFileStatusFailed  = 0
	FileUploadCheckFileStatusSuccess = 1
	FileUploadTmpLocalDir            = "tmp"
)

func CheckFile(ctx *gin.Context, r *request.CheckFileReq) (*response.CheckFileResp, error) {
	exist, err := util.FileExist(BuildUploadFilePath(r.FileName, r.Sequence, r.ChunkName))
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_check", err.Error())
	}
	status := FileUploadCheckFileStatusSuccess
	if !exist {
		status = FileUploadCheckFileStatusFailed
	}
	return &response.CheckFileResp{
		Status: status,
	}, nil
}

func CheckFileList(ctx *gin.Context, r *request.CheckFileListReq) (*response.CheckFileListResp, error) {
	dirPath := BuildUploadFilePathDir(r.ChunkName)
	list, err := util.DirFileList(dirPath, false, false)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_file_not_exist", err.Error())
	}
	sequences, err := BuildUploadFileSeqList(list, false)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_check", err.Error())
	}
	return &response.CheckFileListResp{
		UploadedFileSequences: sequences,
	}, nil
}

func UploadFile(ctx *gin.Context, r *request.UploadFileReq) (*response.UploadFileResp, error) {
	var err error
	defer func() {
		if err != nil {
			if err := clearChunkFile(r.FileName, r.Sequence, r.ChunkName); err != nil {
				log.Errorf("upload file but clear chunk file err: %v", err)
				return
			}
			return
		}
	}()
	defer util.PrintPanicStack()

	filePath := BuildUploadFilePath(r.FileName, r.Sequence, r.ChunkName)
	exist, err := util.FileExist(filePath)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_file_not_exist", err.Error())
	}
	if !exist {
		err = saveFileInfo(ctx, filePath)
		if err != nil {
			return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_save", err.Error())
		}
	}
	return &response.UploadFileResp{
		Status: FileUploadCheckFileStatusSuccess,
	}, nil
}

func MergeFile(ctx *gin.Context, r *request.MergeFileReq) (*response.MergeFileResp, error) {
	var err error
	var mergeFilePath = BuildMergeFilePath(r.FileName, r.ChunkName)
	defer func() {
		if err != nil {
			if err := util.DeleteFile(mergeFilePath); err != nil {
				log.Errorf("merge file but delete file err: %v", err)
				return
			}
			return
		}
		if err := clearChunkDir(r.ChunkName); err != nil {
			log.Errorf("merge file but clear chunk dir err: %v", err)
			return
		}
	}()
	defer util.PrintPanicStack()

	dir := BuildUploadFilePathDir(r.ChunkName)
	list, err := util.DirFileList(dir, false, true)
	if err != nil || len(list) != r.ChunkTotal {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_check", err.Error())
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i] < list[j]
	})
	sequences, err := BuildUploadFileSeqList(list, true)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_check", err.Error())
	} else if len(sequences) != r.ChunkTotal {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_check", fmt.Sprintf("sequences num %v but total chunk %v", len(sequences), r.ChunkTotal))
	}
	for i := 1; i <= r.ChunkTotal; i++ {
		if i != sequences[i-1] {
			return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_check", "file upload not completed")
		}
	}
	file, err := util.MergeFile(list, mergeFilePath)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_merge", err.Error())
	}
	open, err := os.Open(file.FilePath)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_file_open", fmt.Sprintf("open file (%v) err: %v", file.FilePath, err))
	}
	defer func() {
		if err := open.Close(); err != nil {
			log.Errorf("merge file but close file (%v) err: %v", file.FilePath, err)
			return
		}
	}()
	defer util.PrintPanicStack()

	if file.TotalByteCount != r.FileSize {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_file_merge", fmt.Sprintf("merge file total size (%v) but origin file size (%v)", file.TotalByteCount, r.FileSize))
	}
	fileName, _, err := minio.UploadFileCommon(ctx, open, util.FileExt(file.FilePath), file.TotalByteCount, r.IsExpired)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_file_merge", fmt.Sprintf("merge file but upload minio err: %v", err))
	}
	filePath, err := minio.GetUploadFileCommon(ctx, fileName, r.IsExpired)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_file_merge", fmt.Sprintf("merge file but get minio file err: %v", err))
	}
	return &response.MergeFileResp{
		FileName: fileName,
		FilePath: filePath,
	}, nil
}

func CleanFile(ctx *gin.Context, r *request.CleanFileReq) (*response.CleanFileResp, error) {
	err := clearChunkDir(r.ChunkName)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_file_clear", err.Error())
	}
	return &response.CleanFileResp{
		Status: FileUploadCheckFileStatusSuccess,
	}, nil
}

func DeleteFile(ctx *gin.Context, r *request.DeleteFileReq) (*response.DeleteFileResp, error) {
	list := r.FileList
	if len(list) == 0 {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_not_empty")
	}
	for _, file := range list {
		err := minio.DeleteFileCommon(ctx, file, r.IsExpired)
		if err != nil {
			log.Errorf("delete file (%v) err: %v", file, err)
		}
	}
	return &response.DeleteFileResp{
		Status: FileUploadCheckFileStatusSuccess,
	}, nil
}

func saveFileInfo(ctx *gin.Context, filePath string) error {
	form, err := ctx.MultipartForm()
	if err != nil {
		return fmt.Errorf("read file (%v) err: %v", filePath, err)
	}
	files := form.File["files"]
	if len(files) == 0 {
		return fmt.Errorf("file (%v) not exist", filePath)
	}
	fileInfo := files[0]
	err = ctx.SaveUploadedFile(fileInfo, filePath)
	if err != nil {
		return fmt.Errorf("save file (%v) err: %v", filePath, err)
	}
	return nil
}

func clearChunkDir(chunkName string) error {
	dir := BuildFilePathDir(chunkName)
	exist, err := util.FileExist(dir)
	if err != nil {
		return fmt.Errorf("check dir (%v) err: %v", dir, err)
	}
	if exist {
		err = util.DeleteDir(dir)
		if err != nil {
			return fmt.Errorf("delete dir (%v) err: %v", dir, err)
		}
	}
	return nil
}

func clearChunkFile(fileName string, sequence int, chunkName string) error {
	filePath := BuildUploadFilePath(fileName, sequence, chunkName)
	exist, err := util.FileExist(filePath)
	if err != nil {
		return fmt.Errorf("check file (%v) err: %v", filePath, err)
	}
	if exist {
		err = util.DeleteFile(filePath)
		if err != nil {
			return fmt.Errorf("delete file (%v) err: %v", filePath, err)
		}
	}
	dirPath := BuildUploadFilePathDir(chunkName)
	exist, err = util.FileExist(dirPath)
	if err != nil {
		return fmt.Errorf("check dir (%v) err: %v", dirPath, err)
	}
	if exist {
		dir, err := os.ReadDir(dirPath)
		if err != nil {
			return fmt.Errorf("read dir (%v) err: %v", dirPath, err)
		}
		if len(dir) == 0 {
			err = util.DeleteDir(dirPath)
			if err != nil {
				return fmt.Errorf("delete dir (%v) err: %v", dirPath, err)
			}
		}
	}
	return nil
}

func BuildFilePathDir(baseFileName string) string {
	fileMd5 := util.MD5([]byte(baseFileName))
	return fmt.Sprintf("%s/%s", FileUploadTmpLocalDir, fileMd5)
}

func BuildUploadFilePath(baseFileName string, sequence int, chunkName string) string {
	fileName := fmt.Sprintf("%010d_%s", sequence, baseFileName)
	return fmt.Sprintf("%s/upload/%s", BuildFilePathDir(chunkName), fileName)
}

func BuildUploadFilePathDir(chunkName string) string {
	return fmt.Sprintf("%s/upload", BuildFilePathDir(chunkName))
}

func BuildMergeFilePath(baseFileName string, chunkName string) string {
	mergeFileName := util.GenUUID() + util.FileExt(baseFileName)
	return fmt.Sprintf("%s/merge/%s", BuildFilePathDir(chunkName), mergeFileName)
}

func BuildUploadFileSeqList(filePathList []string, fullPath bool) ([]int, error) {
	var retList []int
	for _, filePath := range filePathList {
		seq, _, err := BuildChunkSequence(filePath, fullPath)
		if err != nil {
			return nil, err
		}
		retList = append(retList, seq)
	}
	return retList, nil
}

func BuildChunkSequence(storeFile string, fullPath bool) (int, string, error) {
	if fullPath {
		lastIndex := strings.LastIndex(storeFile, "/")
		if lastIndex < 0 {
			return 0, "", fmt.Errorf("store file (%v) is not full path", storeFile)
		}
		storeFile = storeFile[lastIndex+1:]
	}
	splitIndex := strings.Index(storeFile, "_")
	if splitIndex < 0 {
		return 0, "", fmt.Errorf("store file (%v) is empty", storeFile)
	}
	sequence, err := strconv.Atoi(storeFile[0:splitIndex])
	if err != nil {
		return 0, "", fmt.Errorf("store file (%v) is invalid", storeFile)
	}
	return sequence, storeFile[splitIndex+1:], nil
}

func DirectUploadFiles(ctx *gin.Context, r *request.DirectUploadFilesReq) (*response.DirectUploadFilesResp, error) {
	var uploadFiles []*response.DirectUploadFileInfo
	form, err := ctx.MultipartForm()
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_save", err.Error())
	}
	files := form.File["files"]
	if len(files) <= 0 {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_check", fmt.Errorf("file is empty").Error())
	}
	for _, file := range files {
		uploadFileInfo, err := directUploadFile(ctx, r, file)
		if err != nil {
			return nil, err
		}
		uploadFiles = append(uploadFiles, uploadFileInfo)
	}
	return &response.DirectUploadFilesResp{
		Files: uploadFiles,
	}, nil
}

func directUploadFile(ctx *gin.Context, r *request.DirectUploadFilesReq, file *multipart.FileHeader) (*response.DirectUploadFileInfo, error) {
	open, err := file.Open()
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_file_open", err.Error())
	}
	defer func() {
		if err = open.Close(); err != nil {
			log.Errorf("close file (%v) err: %v", file, err)
			return
		}
	}()
	defer util.PrintPanicStack()
	fileName, _, err := minio.UploadFileCommon(ctx, open, util.FileExt(file.Filename), file.Size, r.IsExpired)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_minio", fmt.Sprintf("upload minio err: %v", err))
	}
	filePath, err := minio.GetUploadFileCommon(ctx, fileName, r.IsExpired)
	if err != nil {
		return nil, grpc_util.ErrorStatusWithKey(errs.Code_BFFGeneral, "bff_file_upload_get_minio_path", fmt.Sprintf("get minio file err: %v", err))
	}
	return &response.DirectUploadFileInfo{
		FileName: file.Filename,
		FilePath: filePath,
		FileSize: file.Size,
		FileId:   fileName,
	}, nil
}
