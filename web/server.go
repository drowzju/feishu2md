package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// 配置信息结构
type ConfigInfo struct {
	AppID      string `json:"app_id"`
	AppSecret  string `json:"app_secret"`
	OutputPath string `json:"output_path"`
}

// 初始化日志系统，将日志输出到文件
func initLogger() (*os.File, error) {
	// 创建日志目录
	logDir := filepath.Join(os.Getenv("APPDATA"), "feishu2md", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建日志文件，使用日期作为文件名
	logFileName := fmt.Sprintf("feishu2md_%s.log", time.Now().Format("2006-01-02"))
	logFilePath := filepath.Join(logDir, logFileName)

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 设置日志输出到文件和控制台
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Printf("日志系统初始化完成，日志文件: %s", logFilePath)

	return logFile, nil
}

// 在main函数中添加命令行参数解析
func main() {
	// 解析命令行参数
	var port int
	var logToFile bool

	flag.IntVar(&port, "port", 8080, "服务器端口")
	flag.BoolVar(&logToFile, "log-to-file", false, "是否将日志输出到文件")
	flag.Parse()

	// 设置日志
	if logToFile {
		// 初始化日志系统
		logFile, err := initLogger()
		if err != nil {
			// 如果日志初始化失败，仅输出到控制台
			log.SetFlags(log.LstdFlags | log.Lshortfile)
			log.Printf("警告: 日志系统初始化失败: %v", err)
		} else {
			defer logFile.Close()
		}
	} else {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	log.Printf("后端服务启动中...")

	// 创建路由
	router := setupRouter()

	// 启动服务器
	log.Printf("服务器启动在 http://localhost:%d", port)
	if err := router.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}

// 保存配置处理函数
func saveConfigHandler(c *gin.Context) {
	var config ConfigInfo
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的配置数据"})
		return
	}

	// 创建配置目录
	configDir := filepath.Join(os.Getenv("APPDATA"), "feishu2md")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "创建配置目录失败"})
		return
	}

	// 设置环境变量
	os.Setenv("FEISHU_APP_ID", config.AppID)
	os.Setenv("FEISHU_APP_SECRET", config.AppSecret)

	// 保存配置到文件
	configFile := filepath.Join(configDir, "config.json")
	file, err := os.Create(configFile)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "创建配置文件失败"})
		return
	}
	defer file.Close()

	// 写入配置
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "写入配置失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "配置已保存"})
}

// 获取配置处理函数
func getConfigHandler(c *gin.Context) {
	configFile := filepath.Join(os.Getenv("APPDATA"), "feishu2md", "config.json")

	// 检查配置文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		c.JSON(200, gin.H{
			"success": true,
			"config": ConfigInfo{
				AppID:      "",
				AppSecret:  "",
				OutputPath: filepath.Join(os.Getenv("USERPROFILE"), "Documents"),
			},
		})
		return
	}

	// 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "读取配置文件失败"})
		return
	}

	var config ConfigInfo
	if err := json.Unmarshal(data, &config); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "解析配置文件失败"})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"config":  config,
	})
}

func setupRouter() *gin.Engine {
	router := gin.New()

	// 设置CORS
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 注册路由
	router.GET("/download", downloadHandler)
	router.GET("/config", getConfigHandler)
	router.POST("/config", saveConfigHandler)

	// Wiki相关接口
	router.GET("/wiki/space-info", getWikiSpaceInfoHandler)
	router.GET("/wiki/top-nodes", getWikiTopNodesHandler)
	router.GET("/wiki/node-children", getWikiNodeChildrenHandler)
	router.POST("/wiki/save-tree", saveWikiTreeHandler)
	router.GET("/wiki/spaces", getAllWikiSpacesHandler) // 获取所有空间列表的路由

	return router
}
