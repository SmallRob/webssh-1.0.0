package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"webssh/controller"
	"webssh/core"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

//go:embed public/*
var f embed.FS

var (
	port       = flag.Int("p", 8888, "服务运行端口")
	v          = flag.Bool("v", false, "显示版本号")
	authInfo   = flag.String("a", "", "开启账号密码登录验证, '-a user:pass'的格式传参")
	timeout    int
	savePass   bool
	version    string
	buildDate  string
	goVersion  string
	gitVersion string
	username   string
	password   string
)

func init() {
	flag.IntVar(&timeout, "t", 120, "ssh连接超时时间(min)")
	flag.BoolVar(&savePass, "s", true, "保存ssh密码")
	if envVal, ok := os.LookupEnv("savePass"); ok {
		if b, err := strconv.ParseBool(envVal); err == nil {
			savePass = b
		}
	}
	if envVal, ok := os.LookupEnv("authInfo"); ok {
		*authInfo = envVal
	}
	if envVal, ok := os.LookupEnv("port"); ok {
		if b, err := strconv.Atoi(envVal); err == nil {
			*port = b
		}
	}
	flag.Parse()
	if *v {
		fmt.Printf("Version: %s\n\n", version)
		fmt.Printf("BuildDate: %s\n\n", buildDate)
		fmt.Printf("GoVersion: %s\n\n", goVersion)
		fmt.Printf("GitVersion: %s\n\n", gitVersion)
		os.Exit(0)
	}
	if *authInfo != "" {
		accountInfo := strings.Split(*authInfo, ":")
		if len(accountInfo) != 2 || accountInfo[0] == "" || accountInfo[1] == "" {
			fmt.Println("请按'user:pass'的格式来传参或设置环境变量, 且账号密码都不能为空!")
			os.Exit(0)
		}
		username, password = accountInfo[0], accountInfo[1]
	}
}

func main() {
    server := gin.New()
    server.Use(gin.Recovery())
    server.SetTrustedProxies(nil)
    server.Use(gzip.Gzip(gzip.DefaultCompression))

	// --- API Routes ---
	// No BasicAuth for API routes as per original logic.
	// If auth is needed for APIs, these routes should be moved inside the auth-enabled group below.
	server.GET("/term", func(c *gin.Context) {
		controller.TermWs(c, time.Duration(timeout)*time.Minute)
	})
	// RDP / VNC 远程桌面通道：与 /term 采用相同的 sshInfo 描述符，
	// 由 core 层的 RDCleanPath 代理 / RFB 中继接管。
	server.GET("/rdp", func(c *gin.Context) {
		controller.RemoteWs(c, core.ProtocolRDP)
	})
	server.GET("/vnc", func(c *gin.Context) {
		controller.RemoteWs(c, core.ProtocolVNC)
	})
	// 内置协议清单（ssh / rdp / vnc），供前端协议选择器使用
	server.GET("/protocols", func(c *gin.Context) {
		c.JSON(200, controller.ProtocolList(c))
	})
	server.GET("/check", func(c *gin.Context) {
		responseBody := controller.CheckConnection(c)
		responseBody.Data = map[string]interface{}{
			"savePass": savePass,
		}
		c.JSON(200, responseBody)
	})
	// 快捷服务器列表：每次请求实时读取配置文件，修改后无需重启容器
	server.GET("/servers", func(c *gin.Context) {
		path := os.Getenv("SERVERS_FILE")
		if path == "" {
			path = "/webssh/servers.json"
		}
		data, err := os.ReadFile(path)
		if err != nil {
			c.JSON(200, []interface{}{})
			return
		}
		c.Data(200, "application/json; charset=utf-8", data)
	})
	file := server.Group("/file")
	{
		file.GET("/list", func(c *gin.Context) {
			c.JSON(200, controller.FileList(c))
		})
		file.GET("/download", func(c *gin.Context) {
			controller.DownloadFile(c)
		})
		file.POST("/upload", func(c *gin.Context) {
			c.JSON(200, controller.UploadFile(c))
		})
		file.GET("/progress", func(c *gin.Context) {
			controller.UploadProgressWs(c)
		})
		file.GET("/delete", func(c *gin.Context) {
			c.JSON(200, controller.DeleteFileOrDir(c))
		})
		file.GET("/rename", func(c *gin.Context) {
			c.JSON(200, controller.RenameFileOrDir(c))
		})
		file.GET("/mkdir", func(c *gin.Context) {
			c.JSON(200, controller.CreateNewFolder(c))
		})
		file.GET("/read", func(c *gin.Context) {
			c.JSON(200, controller.ReadFileContent(c))
		})
		file.POST("/save", func(c *gin.Context) {
			c.JSON(200, controller.SaveFileContent(c))
		})
		file.GET("/search", func(c *gin.Context) {
			c.JSON(200, controller.SearchFiles(c))
		})
		file.GET("/info", func(c *gin.Context) {
			c.JSON(200, controller.GetFileInfo(c))
		})
	}

	// --- Static Files & SPA Frontend ---
	// Serve static files from the 'static' directory
	staticFS, _ := fs.Sub(f, "public/static")
	server.StaticFS("/static", http.FS(staticFS))
	
	// For any other route, serve the index.html file.
	// This makes it compatible with Vue Router's history mode.
	server.NoRoute(func(c *gin.Context) {
		if *authInfo != "" {
			// If auth is enabled, check credentials.
			// This is a simplified check. For production, use a proper session/token mechanism.
			user, pass, hasAuth := c.Request.BasicAuth()
			if !hasAuth || user != username || pass != password {
				c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
		}
		
		indexHTML, err := f.ReadFile("public/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "index.html not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})

	fmt.Printf("Github：https://github.com/eooce/webssh\n")
	server.Run(fmt.Sprintf(":%d", *port))
}
