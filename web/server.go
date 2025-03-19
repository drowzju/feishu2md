package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// 配置信息结构
type ConfigInfo struct {
	AppID      string `json:"app_id"`
	AppSecret  string `json:"app_secret"`
	OutputPath string `json:"output_path"`
}

func main() {
	// 设置日志
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 创建路由
	router := gin.Default()

	// 配置CORS，允许Flutter应用访问
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
	}))

	// 注册API路由
	router.GET("/download", downloadHandler)
	router.GET("/wiki-docs", getWikiDocsHandler)

	// 新增API：保存配置
	router.POST("/config", saveConfigHandler)

	// 新增API：获取配置
	router.GET("/config", getConfigHandler)

	// 启动服务器
	port := 8080
	fmt.Printf("服务器启动在 http://localhost:%d\n", port)
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
