package core

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

// CreateSftp 创建sftp客户端
func (sclient *SSHClient) CreateSftp() error {
	err := sclient.GenerateClient()
	if err != nil {
		return err
	}
	client, err := sftp.NewClient(sclient.Client)
	if err != nil {
		return err
	}
	sclient.Sftp = client
	return nil
}

// Mkdirs 创建目录
func (sclient *SSHClient) Mkdirs(path string) error {
	if _, err := sclient.Sftp.Stat(path); os.IsNotExist(err) {
		return sclient.Sftp.MkdirAll(path)
	}
	return nil
}

// Download 下载文件
func (sclient *SSHClient) Download(srcPath string) (*sftp.File, error) {
	return sclient.Sftp.Open(srcPath)
}

// Upload 上传文件
func (sclient *SSHClient) Upload(file multipart.File, id, dstPath string) error {
	dstFile, err := sclient.Sftp.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	defer func() {
		// 上传完后删掉slice里面的数据
		if len(WcList) < 2 {
			WcList = nil
		} else {
			for i := 0; i < len(WcList); i++ {
				if WcList[i].Id == id {
					WcList = append(WcList[:i], WcList[i+1:]...)
					break
				}
			}
		}
	}()
	wc := WriteCounter{Id: id}
	WcList = append(WcList, &wc)
	_, err = io.Copy(dstFile, io.TeeReader(file, &wc))
	if err != nil {
		return err
	}
	return nil
}

// DeleteFile 删除文件或目录
func (sclient *SSHClient) DeleteFile(path string) error {
	info, err := sclient.Sftp.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return sclient.Sftp.RemoveDirectory(path)
	}
	return sclient.Sftp.Remove(path)
}

// RenameFile 重命名文件或目录
func (sclient *SSHClient) RenameFile(oldPath, newPath string) error {
	return sclient.Sftp.Rename(oldPath, newPath)
}

// ReadFile 读取文件内容
func (sclient *SSHClient) ReadFile(path string) ([]byte, error) {
	file, err := sclient.Sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// SaveFile 保存文件内容
func (sclient *SSHClient) SaveFile(path string, content []byte) error {
	file, err := sclient.Sftp.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(content)
	return err
}

// SearchFiles 搜索文件
func (sclient *SSHClient) SearchFiles(rootPath, keyword string) ([]FileSearchResult, error) {
	var results []FileSearchResult
	maxDepth := 5

	var walk func(dir string, depth int) error
	walk = func(dir string, depth int) error {
		if depth > maxDepth {
			return nil
		}
		entries, err := sclient.Sftp.ReadDir(dir)
		if err != nil {
			return nil // 跳过无权限的目录
		}
		for _, entry := range entries {
			fullPath := dir + "/" + entry.Name()
			if dir == "/" {
				fullPath = "/" + entry.Name()
			}
			if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(keyword)) {
				results = append(results, FileSearchResult{
					Name:  entry.Name(),
					Path:  fullPath,
					IsDir: entry.IsDir(),
					Size:  Bytefmt(uint64(entry.Size())),
				})
			}
			if entry.IsDir() {
				if err := walk(fullPath, depth+1); err != nil {
					continue
				}
			}
			if len(results) >= 100 {
				return fmt.Errorf("max results reached")
			}
		}
		return nil
	}

	err := walk(rootPath, 0)
	if err != nil && !strings.Contains(err.Error(), "max results reached") {
		return nil, err
	}
	return results, nil
}

// FileSearchResult 文件搜索结果
type FileSearchResult struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  string `json:"size"`
}

// GetFileInfo 获取文件信息（用于判断是否为文本文件）
func (sclient *SSHClient) GetFileInfo(path string) (*FileEditInfo, error) {
	info, err := sclient.Sftp.Stat(path)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(path))
	textExtensions := map[string]bool{
		".txt": true, ".md": true, ".json": true, ".xml": true,
		".yaml": true, ".yml": true, ".toml": true, ".ini": true,
		".conf": true, ".cfg": true, ".log": true, ".csv": true,
		".go": true, ".py": true, ".js": true, ".ts": true,
		".jsx": true, ".tsx": true, ".vue": true, ".html": true,
		".htm": true, ".css": true, ".scss": true, ".less": true,
		".java": true, ".c": true, ".cpp": true, ".h": true,
		".hpp": true, ".rs": true, ".rb": true, ".php": true,
		".sh": true, ".bash": true, ".zsh": true, ".fish": true,
		".sql": true, ".r": true, ".swift": true, ".kt": true,
		".scala": true, ".lua": true, ".pl": true, ".pm": true,
		".dockerfile": true, ".makefile": true, ".gitignore": true,
		".env": true, ".editorconfig": true, ".prettierrc": true,
		".eslintrc": true, ".babelrc": true,
	}

	langMap := map[string]string{
		".md": "markdown", ".json": "json", ".xml": "xml",
		".yaml": "yaml", ".yml": "yaml", ".toml": "toml",
		".ini": "ini", ".go": "go", ".py": "python",
		".js": "javascript", ".ts": "typescript", ".jsx": "jsx",
		".tsx": "tsx", ".vue": "vue", ".html": "html",
		".htm": "html", ".css": "css", ".scss": "scss",
		".less": "less", ".java": "java", ".c": "c",
		".cpp": "cpp", ".h": "c", ".hpp": "cpp",
		".rs": "rust", ".rb": "ruby", ".php": "php",
		".sh": "shell", ".bash": "shell", ".sql": "sql",
		".r": "r", ".swift": "swift", ".kt": "kotlin",
		".scala": "scala", ".lua": "lua", ".txt": "plaintext",
		".log": "plaintext", ".conf": "ini", ".cfg": "ini",
		".csv": "plaintext",
	}

	isText := textExtensions[ext] || info.Size() < 1024*1024 // 小于1MB默认为文本
	language := langMap[ext]
	if language == "" {
		if isText {
			language = "plaintext"
		}
	}

	return &FileEditInfo{
		Name:     info.Name(),
		Size:     Bytefmt(uint64(info.Size())),
		IsText:   isText,
		Language: language,
		ModTime:  info.ModTime().Format("2006-01-02 15:04:05"),
	}, nil
}

// FileEditInfo 文件编辑信息
type FileEditInfo struct {
	Name     string `json:"name"`
	Size     string `json:"size"`
	IsText   bool   `json:"isText"`
	Language string `json:"language"`
	ModTime  string `json:"modTime"`
}
