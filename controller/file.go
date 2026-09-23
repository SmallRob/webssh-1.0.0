package controller

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"webssh/core"

	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
)

// File 结构体
type File struct {
	Name       string
	Size       string
	ModifyTime string
	IsDir      bool
}

// Bytefmt returns a human-readable byte string of the form 10M, 12.5K, and so forth.  The following units are available:
//	E: Exabyte
//	P: Petabyte
//	T: Terabyte
//	G: Gigabyte
//	M: Megabyte
//	K: Kilobyte
//	B: Byte
// The unit that results in the smallest number greater than or equal to 1 is always chosen.
//
// 实现已下沉到 core 包，这里保留同名导出的薄封装，避免上层调用点改动。
func Bytefmt(bytes uint64) string {
	return core.Bytefmt(bytes)
}

type fileSplice []File

// Len 比较大小
func (f fileSplice) Len() int { return len(f) }

// Swap 交换
func (f fileSplice) Swap(i, j int) { f[i], f[j] = f[j], f[i] }

// Less 比大小
func (f fileSplice) Less(i, j int) bool { return f[i].IsDir }

// UploadFile 上传文件
func UploadFile(c *gin.Context) *ResponseBody {
	var (
		sshClient core.SSHClient
		err       error
	)
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	sshInfo := c.PostForm("sshInfo")
	id := c.PostForm("id")
	if sshClient, err = core.DecodedMsgToSSHClient(sshInfo); err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer file.Close()
	path := strings.TrimSpace(c.DefaultPostForm("path", ""))
	if path == "" {
		path = detectHomeDir(sshClient.Sftp, sshClient.Username)
	}
	pathArr := []string{strings.TrimRight(path, "/")}
	if dir := c.DefaultPostForm("dir", ""); "" != dir {
		pathArr = append(pathArr, dir)
		if err := sshClient.Mkdirs(strings.Join(pathArr, "/")); err != nil {
			responseBody.Msg = err.Error()
			return &responseBody
		}
	}
	pathArr = append(pathArr, header.Filename)
	err = sshClient.Upload(file, id, strings.Join(pathArr, "/"))
	if err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// DownloadFile 下载文件
func DownloadFile(c *gin.Context) *ResponseBody {
	var (
		sshClient core.SSHClient
		err       error
	)
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	path := strings.TrimSpace(c.DefaultQuery("path", ""))
	if path == "" {
		path = detectHomeDir(sshClient.Sftp, sshClient.Username)
	}
	sshInfo := c.DefaultQuery("sshInfo", "")
	if sshClient, err = core.DecodedMsgToSSHClient(sshInfo); err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()
	if sftpFile, err := sshClient.Download(path); err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
	} else {
		defer sftpFile.Close()
		c.Writer.WriteHeader(http.StatusOK)
		fileMeta := strings.Split(path, "/")
		c.Header("Content-Disposition", "attachment; filename="+fileMeta[len(fileMeta)-1])
		_, _ = io.Copy(c.Writer, sftpFile)
	}
	return &responseBody
}

// UploadProgressWs 获取上传进度ws
func UploadProgressWs(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}
	id := c.Query("id")

	var ready, find bool
	for {
		if !ready && core.WcList == nil {
			continue
		}
		for _, v := range core.WcList {
			if v.Id == id {
				wsConn.WriteMessage(1, []byte(strconv.Itoa(v.Total)))
				find = true
				if !ready {
					ready = true
				}
				break
			}
		}
		if ready && !find {
			wsConn.Close()
			break
		}

		if ready {
			time.Sleep(300 * time.Millisecond)
			find = false
		}
	}
	return &responseBody
}

// FileList 获取文件列表
func FileList(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	path := c.DefaultQuery("path", "")
	sshInfo := c.DefaultQuery("sshInfo", "")
	sshClient, err := core.DecodedMsgToSSHClient(sshInfo)
	if err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()

	// 检测 home 目录
	home := detectHomeDir(sshClient.Sftp, sshClient.Username)

	// 如果 path 为 / 且 home 不为 /，且不是 root 用户，自动切换到 home
	if path == "/" && home != "/" && sshClient.Username != "root" {
		path = home
	}

	// 1. 如果 path 为空，首次进入，普通用户进入 home，root 进入 /
	if path == "" {
		if sshClient.Username == "root" {
			path = "/"
		} else {
			path = home
		}
	}

	files, err := sshClient.Sftp.ReadDir(path)
	if err != nil {
		if strings.Contains(err.Error(), "exist") {
			responseBody.Msg = fmt.Sprintf("Directory %s: no such file or directory", path)
		} else {
			responseBody.Msg = err.Error()
		}
		return &responseBody
	}
	var (
		fileList fileSplice
		fileSize string
	)
	for _, mFile := range files {
		if mFile.IsDir() {
			fileSize = strconv.FormatInt(mFile.Size(), 10)
		} else {
			fileSize = Bytefmt(uint64(mFile.Size()))
		}
		file := File{Name: mFile.Name(), IsDir: mFile.IsDir(), Size: fileSize, ModifyTime: mFile.ModTime().Format("2006-01-02 15:04:05")}
		fileList = append(fileList, file)
	}
	sort.Stable(fileList)
	responseBody.Data = gin.H{
		"list": fileList,
		"home": home, // home 字段始终返回 home 目录
	}
	return &responseBody
}

// DeleteFileOrDir 删除文件或目录
func DeleteFileOrDir(c *gin.Context) *ResponseBody {
	var (
		sshClient core.SSHClient
		err       error
	)
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	path := strings.TrimSpace(c.DefaultQuery("path", ""))
	sshInfo := c.DefaultQuery("sshInfo", "")
	if path == "" {
		responseBody.Msg = "path is required"
		return &responseBody
	}
	if sshClient, err = core.DecodedMsgToSSHClient(sshInfo); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()
	if err := sshClient.DeleteFile(path); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// RenameFileOrDir 重命名文件或目录
func RenameFileOrDir(c *gin.Context) *ResponseBody {
	var (
		sshClient core.SSHClient
		err       error
	)
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	oldPath := strings.TrimSpace(c.DefaultQuery("oldPath", ""))
	newPath := strings.TrimSpace(c.DefaultQuery("newPath", ""))
	sshInfo := c.DefaultQuery("sshInfo", "")
	if oldPath == "" || newPath == "" {
		responseBody.Msg = "oldPath and newPath are required"
		return &responseBody
	}
	if sshClient, err = core.DecodedMsgToSSHClient(sshInfo); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()
	if err := sshClient.RenameFile(oldPath, newPath); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// CreateNewFolder 创建新文件夹
func CreateNewFolder(c *gin.Context) *ResponseBody {
	var (
		sshClient core.SSHClient
		err       error
	)
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	path := strings.TrimSpace(c.DefaultQuery("path", ""))
	sshInfo := c.DefaultQuery("sshInfo", "")
	if path == "" {
		responseBody.Msg = "path is required"
		return &responseBody
	}
	if sshClient, err = core.DecodedMsgToSSHClient(sshInfo); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()
	if err := sshClient.Mkdirs(path); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// ReadFileContent 读取文件内容
func ReadFileContent(c *gin.Context) *ResponseBody {
	var (
		sshClient core.SSHClient
		err       error
	)
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	path := strings.TrimSpace(c.DefaultQuery("path", ""))
	sshInfo := c.DefaultQuery("sshInfo", "")
	if path == "" {
		responseBody.Msg = "path is required"
		return &responseBody
	}
	if sshClient, err = core.DecodedMsgToSSHClient(sshInfo); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()

	fileInfo, err := sshClient.GetFileInfo(path)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}

	content, err := sshClient.ReadFile(path)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}

	responseBody.Data = gin.H{
		"content":  string(content),
		"info":     fileInfo,
	}
	return &responseBody
}

// SaveFileContent 保存文件内容
func SaveFileContent(c *gin.Context) *ResponseBody {
	var (
		sshClient core.SSHClient
		err       error
	)
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	path := c.PostForm("path")
	sshInfo := c.PostForm("sshInfo")
	content := c.PostForm("content")

	if path == "" {
		responseBody.Msg = "path is required"
		return &responseBody
	}
	if sshClient, err = core.DecodedMsgToSSHClient(sshInfo); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()
	if err := sshClient.SaveFile(path, []byte(content)); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// SearchFiles 搜索文件
func SearchFiles(c *gin.Context) *ResponseBody {
	var (
		sshClient core.SSHClient
		err       error
	)
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	keyword := strings.TrimSpace(c.DefaultQuery("keyword", ""))
	rootPath := strings.TrimSpace(c.DefaultQuery("path", "/"))
	sshInfo := c.DefaultQuery("sshInfo", "")
	if keyword == "" {
		responseBody.Msg = "keyword is required"
		return &responseBody
	}
	if sshClient, err = core.DecodedMsgToSSHClient(sshInfo); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()
	results, err := sshClient.SearchFiles(rootPath, keyword)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	responseBody.Data = gin.H{
		"results": results,
		"count":   len(results),
	}
	return &responseBody
}

// GetFileInfo 获取文件信息
func GetFileInfo(c *gin.Context) *ResponseBody {
	var (
		sshClient core.SSHClient
		err       error
	)
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	path := strings.TrimSpace(c.DefaultQuery("path", ""))
	sshInfo := c.DefaultQuery("sshInfo", "")
	if path == "" {
		responseBody.Msg = "path is required"
		return &responseBody
	}
	if sshClient, err = core.DecodedMsgToSSHClient(sshInfo); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err := sshClient.CreateSftp(); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer sshClient.Close()
	fileInfo, err := sshClient.GetFileInfo(path)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	responseBody.Data = fileInfo
	return &responseBody
}

// 自动检测home目录
func detectHomeDir(sftpClient *sftp.Client, username string) string {
	// 1. 尝试获取当前工作目录
	if wd, err := sftpClient.Getwd(); err == nil && wd != "" {
		return wd
	}

	// 2. 如果是 root 用户，直接返回 /root
	if username == "root" {
		return "/root"
	}

	// 3. 先检测 /usr/home/用户名，再检测 /home/用户名
	potentialHome := fmt.Sprintf("/usr/home/%s", username)
	if _, err := sftpClient.Stat(potentialHome); err == nil {
		return potentialHome
	}
	potentialHome = fmt.Sprintf("/home/%s", username)
	if _, err := sftpClient.Stat(potentialHome); err == nil {
		return potentialHome
	}

	// 4. 如果都失败了，返回根目录
	return "/home"
}
