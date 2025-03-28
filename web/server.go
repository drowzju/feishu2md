package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
	router := setupRouter()

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
