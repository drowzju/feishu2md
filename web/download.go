package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp" // 添加正则表达式包
	"strings"
	"time"

	"github.com/88250/lute"
	"github.com/Wsine/feishu2md/core"
	"github.com/Wsine/feishu2md/utils"
	"github.com/gin-gonic/gin"
)

func downloadHandler(c *gin.Context) {
	// get parameters
	feishu_docx_url, err := url.QueryUnescape(c.Query("url"))
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid encoded feishu/larksuite URL")
		return
	}

	// 获取输出路径参数
	outputPath := c.Query("output_path")
	if outputPath == "" {
		outputPath = "output" // 默认输出路径
	}

	// 获取直接传递的token和type参数
	directToken := c.Query("token")
	directType := c.Query("type")

	// 记录请求参数
	log.Printf("下载请求参数: URL=%s, 输出路径=%s, 直接Token=%s, 直接类型=%s",
		feishu_docx_url, outputPath, directToken, directType)

	// 根据不同参数来源确定文档token和类型
	var docType, docToken string

	if directToken != "" {
		// 优先使用直接传递的token和type
		docToken = directToken
		docType = directType
		if docType == "" {
			docType = "docx" // 默认类型
		}
		log.Printf("使用直接传递的参数: token=%s, type=%s", docToken, docType)
	} else {
		// 从URL解析token和type
		var err error
		docType, docToken, err = utils.ValidateDocumentURL(feishu_docx_url)
		if err != nil {
			log.Printf("URL验证失败: %s", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("无效的文档URL: %s", err),
			})
			return
		}
		log.Printf("从URL解析的参数: token=%s, type=%s", docToken, docType)
	}

	fmt.Println("Captured document token:", docToken)

	// Create client with context
	ctx := context.Background()
	config := core.NewConfig(
		os.Getenv("FEISHU_APP_ID"),
		os.Getenv("FEISHU_APP_SECRET"),
	)

	log.Printf("应用凭证: AppID=%s", config.Feishu.AppId)

	// 更新配置中的输出路径
	config.Output.ImageDir = filepath.Join(outputPath, "images")

	// 确保输出目录存在
	if err := os.MkdirAll(config.Output.ImageDir, 0755); err != nil {
		log.Printf("创建输出目录失败: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("创建输出目录失败: %s", err),
		})
		return
	}

	client := core.NewClient(
		config.Feishu.AppId, config.Feishu.AppSecret,
	)

	// Process the download
	parser := core.NewParser(config.Output)
	markdown := ""

	// for a wiki page, we need to renew docType and docToken first
	if docType == "wiki" {
		log.Printf("处理wiki类型文档，获取节点信息: token=%s", docToken)
		node, err := client.GetWikiNodeInfo(ctx, docToken)
		if err != nil {
			log.Printf("获取知识库节点信息失败: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("获取知识库节点信息失败: %s", err),
			})
			return
		}
		log.Printf("获取到节点信息: 标题=%s, 对象类型=%s, 对象Token=%s",
			node.Title, node.ObjType, node.ObjToken)
		docType = node.ObjType
		docToken = node.ObjToken
	}

	if docType == "docs" {
		log.Printf("不支持的文档类型: docs")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "不支持的文档类型: docs",
		})
		return
	}

	// 获取文档内容
	log.Printf("开始获取文档内容: token=%s, type=%s", docToken, docType)
	docx, blocks, err := client.GetDocxContent(ctx, docToken)

	// 这里是原来出错的地方，改进错误处理
	if err != nil {
		// 记录详细错误信息
		log.Printf("获取文档内容失败: %s", err)

		// 检查是否是404错误
		if strings.Contains(err.Error(), "404") {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "文档不存在或无权访问",
				"error":   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "获取文档内容失败",
				"error":   err.Error(),
			})
		}
		return
	}

	// 记录文档基本信息
	log.Printf("成功获取文档内容: 标题=%s, 块数量=%d", docx.Title, len(blocks))

	markdown = parser.ParseDocxContent(docx, blocks)

	// 获取文档标题并处理为合法文件名
	docTitle := docx.Title
	if docTitle == "" {
		docTitle = docToken // 如果标题为空，仍使用token作为备选
	}
	// 替换文件名中的非法字符
	docTitle = sanitizeFilename(docTitle)

	// 获取自定义路径参数
	customPath := c.Query("path")
	log.Printf("自定义路径参数: %s", customPath)

	// 如果提供了自定义路径，创建对应的目录结构
	if customPath != "" {
		// 解码路径
		decodedPath, err := url.QueryUnescape(customPath)
		if err != nil {
			log.Printf("路径解码失败: %s", err)
			decodedPath = customPath
		}

		// 构建完整输出路径
		fullOutputPath := filepath.Join(outputPath, decodedPath)
		log.Printf("使用自定义路径: %s", fullOutputPath)

		// 创建目录
		if err := os.MkdirAll(fullOutputPath, 0755); err != nil {
			log.Printf("创建自定义路径目录失败: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("创建自定义路径目录失败: %s", err),
			})
			return
		}

		// 更新文件保存路径
		outputPath = fullOutputPath
	}

	// 处理图片
	log.Printf("开始处理文档中的图片，共 %d 个", len(parser.ImgTokens))
	zipBuffer := new(bytes.Buffer)
	writer := zip.NewWriter(zipBuffer)
	for i, imgToken := range parser.ImgTokens {
		log.Printf("处理图片 %d/%d: token=%s", i+1, len(parser.ImgTokens), imgToken)
		localLink, rawImage, err := client.DownloadImageRaw(ctx, imgToken, config.Output.ImageDir)
		if err != nil {
			log.Printf("下载图片失败: %s", err)
			// 继续处理其他图片，而不是中断整个过程
			continue
		}

		// 确保图片文件被写入磁盘
		imgFilePath := filepath.Join(config.Output.ImageDir, filepath.Base(localLink))
		err = os.WriteFile(imgFilePath, rawImage, 0644)
		if err != nil {
			log.Printf("保存图片文件失败: %s", err)
			continue
		}

		// 验证文件是否成功写入
		if _, err := os.Stat(imgFilePath); os.IsNotExist(err) {
			log.Printf("图片文件写入验证失败，文件不存在: %s", imgFilePath)
			continue
		}

		log.Printf("图片下载成功: %s", imgFilePath)
		markdown = strings.Replace(markdown, imgToken, localLink, 1)

		// 添加到ZIP文件
		f, err := writer.Create(localLink)
		if err != nil {
			log.Printf("创建ZIP文件条目失败: %s", err)
			continue
		}
		_, err = f.Write(rawImage)
		if err != nil {
			log.Printf("写入图片数据失败: %s", err)
			continue
		}
	}

	engine := lute.New(func(l *lute.Lute) {
		l.RenderOptions.AutoSpace = true
	})
	result := engine.FormatStr("md", markdown)

	// 保存Markdown文件到本地
	mdFilePath := filepath.Join(outputPath, docTitle+".md")
	log.Printf("保存Markdown文件到: %s", mdFilePath)
	err = os.WriteFile(mdFilePath, []byte(result), 0644)
	if err != nil {
		log.Printf("保存Markdown文件失败: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("保存Markdown文件失败: %s", err),
		})
		return
	}

	log.Printf("文档下载和保存成功: %s", mdFilePath)

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "文档下载成功",
		"file_path": mdFilePath,
	})
}

// 处理文件名中的非法字符
func sanitizeFilename(filename string) string {
	// 替换Windows文件名中不允许的字符: \ / : * ? " < > |
	invalidChars := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename

	for _, char := range invalidChars {
		result = strings.ReplaceAll(result, char, "_")
	}

	// 限制文件名长度，避免过长
	if len(result) > 100 {
		result = result[:100]
	}

	return result
}

// 文档节点结构，用于构建树状结构
type DocNode struct {
	Title    string     `json:"title"`
	URL      string     `json:"url"`
	Type     string     `json:"type"`
	Children []*DocNode `json:"children"`
}

// 知识库文档树响应
type WikiDocsTreeResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Tree    *DocNode `json:"tree"`
}

// 获取知识库所有文档链接的处理函数
func getWikiDocsHandler(c *gin.Context) {
	// 获取参数
	wikiURL, err := url.QueryUnescape(c.Query("url"))
	if err != nil {
		c.JSON(http.StatusBadRequest, WikiDocsTreeResponse{
			Success: false,
			Message: "无效的知识库URL",
		})
		return
	}

	// 获取输出路径参数
	outputPath := c.Query("output_path")
	if outputPath == "" {
		outputPath = "output" // 默认输出路径
	}

	// 验证知识库URL
	docType, spaceToken, err := utils.ValidateDocumentURL(wikiURL)
	if err != nil || docType != "wiki" {
		c.JSON(http.StatusBadRequest, WikiDocsTreeResponse{
			Success: false,
			Message: "无效的知识库URL，请确保提供的是知识库设置链接",
		})
		return
	}

	// 创建客户端
	ctx := context.Background()
	config := core.NewConfig(
		os.Getenv("FEISHU_APP_ID"),
		os.Getenv("FEISHU_APP_SECRET"),
	)
	client := core.NewClient(
		config.Feishu.AppId, config.Feishu.AppSecret,
	)

	// 获取知识库根节点
	rootNode, err := client.GetWikiNodeInfo(ctx, spaceToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, WikiDocsTreeResponse{
			Success: false,
			Message: fmt.Sprintf("获取知识库信息失败: %s", err),
		})
		return
	}

	// 尝试获取知识库空间名称
	spaceName := rootNode.Title // 默认使用节点标题作为备选
	spaceInfo, err := client.GetWikiName(ctx, rootNode.SpaceID)
	if err == nil && spaceInfo != "" {
		spaceName = spaceInfo
		fmt.Printf("获取到知识库空间名称: %s\n", spaceName)
	} else {
		fmt.Printf("获取知识库空间名称失败，使用节点标题作为备选: %s\n", spaceName)
	}

	// 添加调试日志
	fmt.Printf("空间名称: %s, 节点Token: %s\n", spaceName, rootNode.NodeToken)

	// 创建根文档节点
	docTree := &DocNode{
		Title:    spaceName,
		URL:      fmt.Sprintf("https://feishu.cn/wiki/%s", rootNode.NodeToken),
		Type:     "space",
		Children: []*DocNode{},
	}

	// 获取知识库中的顶级节点列表
	topNodes, err := client.GetWikiNodeList(ctx, rootNode.SpaceID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, WikiDocsTreeResponse{
			Success: false,
			Message: fmt.Sprintf("获取知识库顶级节点失败: %s", err),
		})
		return
	}

	fmt.Printf("知识库有 %d 个顶级节点\n", len(topNodes))

	// 处理每个顶级节点
	for _, item := range topNodes {
		topNode := &core.WikiNode{
			NodeToken: item.NodeToken,
			ObjToken:  item.ObjToken,
			ObjType:   item.ObjType,
			Title:     item.Title,
			SpaceID:   rootNode.SpaceID,
		}

		// 创建顶级文档节点
		docNode := &DocNode{
			Title:    topNode.Title,
			Children: []*DocNode{},
		}

		// 设置URL和类型
		if topNode.ObjType == "docx" || topNode.ObjType == "doc" {
			docNode.URL = fmt.Sprintf("https://feishu.cn/docx/%s", topNode.ObjToken)
			docNode.Type = topNode.ObjType
		} else {
			docNode.URL = fmt.Sprintf("https://feishu.cn/wiki/%s", topNode.NodeToken)
			docNode.Type = "folder"
		}

		// 递归构建文档树
		err = buildDocTree(ctx, client, topNode, docNode)
		if err != nil {
			fmt.Printf("处理顶级节点 %s 失败: %s\n", topNode.Title, err)
			// 继续处理其他节点
		}

		// 添加到根节点
		docTree.Children = append(docTree.Children, docNode)
	}

	// 生成树状结构的文本文件
	treeText := generateTreeText(docTree, 0)

	// 确保输出目录存在
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, WikiDocsTreeResponse{
			Success: false,
			Message: fmt.Sprintf("创建输出目录失败: %s", err),
		})
		return
	}

	// 保存树状结构文本到文件 - 使用空间名称作为文件名
	safeSpaceName := sanitizeFilename(spaceName)
	// 添加调试日志
	fmt.Printf("处理后的空间名称: %s\n", safeSpaceName)
	treeFilePath := filepath.Join(outputPath, safeSpaceName+"_文档树.md")
	fmt.Printf("完整文件路径: %s\n", treeFilePath)
	err = os.WriteFile(treeFilePath, []byte(treeText), 0644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, WikiDocsTreeResponse{
			Success: false,
			Message: fmt.Sprintf("保存文档树文件失败: %s", err),
		})
		return
	}

	// 返回文档树
	c.JSON(http.StatusOK, WikiDocsTreeResponse{
		Success: true,
		Message: fmt.Sprintf("成功生成文档树，已保存到 %s", treeFilePath),
		Tree:    docTree,
	})
}

// 递归构建文档树
func buildDocTree(ctx context.Context, client *core.Client, node *core.WikiNode, docNode *DocNode) error {
	// 打印当前处理的节点信息，帮助调试
	fmt.Printf("处理节点: %s, 类型: %s\n", node.Title, node.ObjType)

	// 添加延迟，避免触发限速
	time.Sleep(100 * time.Millisecond)

	// 获取子节点，添加重试机制
	var children []*core.WikiNode
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		children, err = client.GetWikiNodeChildren(ctx, node.NodeToken)
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "frequency limit") {
			retryDelay := time.Duration(1<<uint(i)) * time.Second // 指数退避: 1s, 2s, 4s
			log.Printf("触发限速，等待 %v 后重试 (%d/%d)...", retryDelay, i+1, maxRetries)
			time.Sleep(retryDelay)
		} else {
			log.Printf("获取节点 %s 的子节点失败: %s", node.Title, err)
			return nil // 非限速错误，继续处理其他节点
		}
	}

	if err != nil {
		log.Printf("获取节点 %s 的子节点失败，已重试 %d 次: %s", node.Title, maxRetries, err)
		return nil // 继续处理其他节点
	}

	fmt.Printf("节点 %s 有 %d 个子节点\n", node.Title, len(children))

	// 递归处理子节点
	for _, child := range children {
		childNode := &DocNode{
			Title:    child.Title,
			Children: []*DocNode{},
		}

		// 设置URL和类型
		if child.ObjType == "docx" || child.ObjType == "doc" {
			childNode.URL = fmt.Sprintf("https://feishu.cn/docx/%s", child.ObjToken)
			childNode.Type = child.ObjType
		} else {
			childNode.URL = fmt.Sprintf("https://feishu.cn/wiki/%s", child.NodeToken)
			childNode.Type = "folder"
		}

		// 递归处理子节点
		err := buildDocTree(ctx, client, child, childNode)
		if err != nil {
			log.Printf("处理子节点 %s 失败: %s", child.Title, err)
			// 继续处理其他子节点，不中断
		}

		// 添加到父节点
		docNode.Children = append(docNode.Children, childNode)
	}

	return nil
}

// 生成树状结构的文本
func generateTreeText(node *DocNode, level int) string {
	var result strings.Builder

	// 添加缩进
	indent := strings.Repeat("  ", level)

	// 添加当前节点
	if node.Type == "docx" || node.Type == "doc" {
		result.WriteString(fmt.Sprintf("%s- [%s](%s)\n", indent, node.Title, node.URL))
	} else {
		result.WriteString(fmt.Sprintf("%s- **%s**\n", indent, node.Title))
	}

	// 递归处理子节点
	for _, child := range node.Children {
		result.WriteString(generateTreeText(child, level+1))
	}

	return result.String()
}

// 获取知识库空间信息
func getWikiSpaceInfoHandler(c *gin.Context) {
	// 获取参数
	wikiURL := c.Query("url")
	spaceID := c.Query("space_id")

	log.Printf("收到空间信息请求，URL: %s, SpaceID: %s", wikiURL, spaceID)

	// 创建客户端
	ctx := context.Background()
	config := core.NewConfig(
		os.Getenv("FEISHU_APP_ID"),
		os.Getenv("FEISHU_APP_SECRET"),
	)
	log.Printf("应用凭证: AppID=%s, AppSecret=%s", config.Feishu.AppId, "***")

	client := core.NewClient(
		config.Feishu.AppId, config.Feishu.AppSecret,
	)

	// 根据不同参数获取空间信息
	var spaceName string
	var nodeToken string
	var spaceIDToUse string

	if spaceID != "" {
		// 直接使用 space_id 参数
		log.Printf("使用提供的 space_id: %s", spaceID)
		spaceIDToUse = spaceID
	} else if wikiURL != "" {
		// 使用 URL 参数
		var err error
		wikiURL, err = url.QueryUnescape(wikiURL)
		if err != nil {
			log.Printf("URL解码失败: %s", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "无效的知识库URL",
			})
			return
		}

		// 检查是否是空间URL (形如 https://feishu.cn/wiki/space/7398737263215149060)
		spaceURLPattern := regexp.MustCompile(`https://feishu\.cn/wiki/space/(\d+)`)
		matches := spaceURLPattern.FindStringSubmatch(wikiURL)

		if len(matches) > 1 {
			// 从空间URL中提取space_id
			spaceIDToUse = matches[1]
			log.Printf("从URL中提取到space_id: %s", spaceIDToUse)
		} else {
			// 尝试作为普通知识库URL处理
			docType, spaceToken, err := utils.ValidateDocumentURL(wikiURL)
			if err != nil || docType != "wiki" {
				log.Printf("URL验证失败: 类型=%s, Token=%s, 错误=%v", docType, spaceToken, err)
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": "无效的知识库URL，请确保提供的是知识库设置链接或空间链接",
				})
				return
			}

			log.Printf("URL验证成功: 类型=%s, Token=%s", docType, spaceToken)

			// 获取知识库根节点
			log.Printf("开始获取知识库根节点信息，Token: %s", spaceToken)
			rootNode, err := client.GetWikiNodeInfo(ctx, spaceToken)
			if err != nil {
				log.Printf("获取知识库根节点失败: %s", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("获取知识库信息失败: %s", err),
				})
				return
			}

			log.Printf("获取知识库根节点成功: 标题=%s, SpaceID=%s, NodeToken=%s",
				rootNode.Title, rootNode.SpaceID, rootNode.NodeToken)

			// 设置相关信息
			spaceIDToUse = rootNode.SpaceID
			nodeToken = rootNode.NodeToken
		}
	} else {
		// 两个参数都没有提供
		log.Printf("未提供 space_id 或 url 参数")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请提供 space_id 或 url 参数",
		})
		return
	}

	// 使用space_id获取空间信息
	if spaceIDToUse != "" {
		// 获取空间名称
		var err error
		spaceName, err = client.GetWikiName(ctx, spaceIDToUse)
		if err != nil {
			log.Printf("通过 space_id 获取空间名称失败: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("获取知识库空间名称失败: %s", err),
			})
			return
		}
		log.Printf("通过 space_id 获取空间名称成功: %s", spaceName)

		// 如果没有节点token，获取空间的顶级节点作为入口节点
		if nodeToken == "" {
			topNodes, err := client.GetWikiNodeList(ctx, spaceIDToUse, nil)
			if err != nil || len(topNodes) == 0 {
				log.Printf("获取空间顶级节点失败或空间没有节点: %v", err)
				// 没有节点时，仍然返回空间信息，但没有节点token
				nodeToken = ""
			} else {
				// 使用第一个顶级节点的token
				nodeToken = topNodes[0].NodeToken
				log.Printf("获取到空间的第一个顶级节点: %s, token: %s", topNodes[0].Title, nodeToken)
			}
		}
	}

	// 返回空间信息
	response := gin.H{
		"success": true,
		"space_info": gin.H{
			"space_id":   spaceIDToUse,
			"space_name": spaceName,
			"node_token": nodeToken,
		},
	}
	log.Printf("返回空间信息: %+v", response)

	c.JSON(http.StatusOK, response)
}

// 获取知识库顶级节点列表
func getWikiTopNodesHandler(c *gin.Context) {
	// 获取参数
	spaceID := c.Query("space_id")
	if spaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少space_id参数",
		})
		return
	}

	log.Printf("开始获取知识库顶级节点，SpaceID: %s", spaceID)

	// 创建客户端
	ctx := context.Background()
	// 增加超时时间到120秒
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	config := core.NewConfig(
		os.Getenv("FEISHU_APP_ID"),
		os.Getenv("FEISHU_APP_SECRET"),
	)
	log.Printf("应用凭证: AppID=%s", config.Feishu.AppId)

	client := core.NewClient(
		config.Feishu.AppId, config.Feishu.AppSecret,
	)

	// 获取知识库中的顶级节点列表，添加重试机制
	var topNodes []*core.WikiNode
	var err error
	maxRetries := 5           // 增加重试次数
	var pageToken string = "" // 添加分页支持

	// 记录所有分页的节点
	allNodes := []*core.WikiNode{}

	// 分页获取所有节点
	for {
		// 尝试获取当前页的节点
		pageSuccess := false
		for i := 0; i < maxRetries; i++ {
			log.Printf("尝试获取顶级节点 (页码标记: %s) (%d/%d)...", pageToken, i+1, maxRetries)

			// 为每次请求创建新的上下文，避免使用已超时的上下文
			reqCtx, reqCancel := context.WithTimeout(context.Background(), 60*time.Second)

			// 记录开始时间，用于计算耗时
			startTime := time.Now()

			// 获取节点列表并转换类型
			var rawNodes []*core.WikiNode
			var nextPageToken string

			// 使用分页参数
			if pageToken == "" {
				rawNodes, nextPageToken, err = client.GetWikiNodeListWithPagination(reqCtx, spaceID, nil)
			} else {
				rawNodes, nextPageToken, err = client.GetWikiNodeListWithPagination(reqCtx, spaceID, &pageToken)
			}

			reqCancel() // 请求完成后取消上下文

			// 计算耗时
			elapsed := time.Since(startTime)
			log.Printf("获取顶级节点请求耗时: %v", elapsed)

			if err == nil {
				// 将当前页节点添加到总列表
				allNodes = append(allNodes, rawNodes...)
				log.Printf("成功获取顶级节点页，本页 %d 个节点，当前总计 %d 个节点", len(rawNodes), len(allNodes))

				// 更新分页标记
				pageToken = nextPageToken
				pageSuccess = true
				break
			}

			// 详细记录错误信息
			log.Printf("获取顶级节点页失败 (%d/%d): %s", i+1, maxRetries, err)

			if strings.Contains(err.Error(), "frequency limit") {
				retryDelay := time.Duration(2<<uint(i)) * time.Second // 指数退避: 2s, 4s, 8s, 16s, 32s
				log.Printf("触发限速，等待 %v 后重试...", retryDelay)
				time.Sleep(retryDelay)
			} else if strings.Contains(err.Error(), "context deadline exceeded") ||
				strings.Contains(err.Error(), "timeout") {
				// 超时错误增加等待时间再重试
				retryDelay := time.Duration(5*(i+1)) * time.Second // 5s, 10s, 15s, 20s, 25s
				log.Printf("请求超时，等待 %v 后重试...", retryDelay)
				time.Sleep(retryDelay)
			} else {
				// 其他错误也尝试重试
				retryDelay := time.Duration(3<<uint(i)) * time.Second
				log.Printf("遇到错误，等待 %v 后重试...", retryDelay)
				time.Sleep(retryDelay)
			}
		}

		// 如果当前页获取失败，返回错误
		if !pageSuccess {
			log.Printf("获取顶级节点页最终失败，已获取 %d 个节点", len(allNodes))
			if len(allNodes) > 0 {
				// 如果已经获取了一些节点，继续处理
				log.Printf("尽管有错误，但已获取部分节点，将继续处理")
				topNodes = allNodes
				break
			} else {
				// 如果一个节点都没获取到，返回错误
				c.JSON(http.StatusInternalServerError, gin.H{
					"success":       false,
					"message":       fmt.Sprintf("获取知识库顶级节点失败: %s", err),
					"error_details": err.Error(),
				})
				return
			}
		}

		// 如果没有下一页，结束循环
		if pageToken == "" {
			log.Printf("所有页面获取完成，共 %d 个节点", len(allNodes))
			topNodes = allNodes
			break
		}
	}

	// 构建返回数据
	var nodes []gin.H
	for i, item := range topNodes {
		nodeType := "folder"
		nodeURL := fmt.Sprintf("https://feishu.cn/wiki/%s", item.NodeToken)

		if item.ObjType == "docx" || item.ObjType == "doc" {
			nodeType = item.ObjType
			nodeURL = fmt.Sprintf("https://feishu.cn/docx/%s", item.ObjToken)
		}

		log.Printf("顶级节点 %d: 标题=%s, 类型=%s", i+1, item.Title, nodeType)

		nodes = append(nodes, gin.H{
			"title":      item.Title,
			"node_token": item.NodeToken,
			"obj_token":  item.ObjToken,
			"obj_type":   item.ObjType,
			"type":       nodeType,
			"url":        nodeURL,
		})
	}

	log.Printf("返回顶级节点列表，共 %d 个节点", len(nodes))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"nodes":   nodes,
	})
}

// 获取知识库节点的子节点
func getWikiNodeChildrenHandler(c *gin.Context) {
	// 获取参数
	nodeToken := c.Query("node_token")
	if nodeToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少node_token参数",
		})
		return
	}

	// 创建客户端
	ctx := context.Background()
	config := core.NewConfig(
		os.Getenv("FEISHU_APP_ID"),
		os.Getenv("FEISHU_APP_SECRET"),
	)
	client := core.NewClient(
		config.Feishu.AppId, config.Feishu.AppSecret,
	)

	// 获取子节点，添加重试机制
	var children []*core.WikiNode
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		children, err = client.GetWikiNodeChildren(ctx, nodeToken)
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "frequency limit") {
			retryDelay := time.Duration(1<<uint(i)) * time.Second // 指数退避: 1s, 2s, 4s
			log.Printf("触发限速，等待 %v 后重试 (%d/%d)...", retryDelay, i+1, maxRetries)
			time.Sleep(retryDelay)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("获取节点子节点失败: %s", err),
			})
			return
		}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取节点子节点失败，已重试 %d 次: %s", maxRetries, err),
		})
		return
	}

	// 构建返回数据
	var nodes []gin.H
	for _, child := range children {
		nodeType := "folder"
		nodeURL := fmt.Sprintf("https://feishu.cn/wiki/%s", child.NodeToken)

		if child.ObjType == "docx" || child.ObjType == "doc" {
			nodeType = child.ObjType
			nodeURL = fmt.Sprintf("https://feishu.cn/docx/%s", child.ObjToken)
		}

		nodes = append(nodes, gin.H{
			"title":      child.Title,
			"node_token": child.NodeToken,
			"obj_token":  child.ObjToken,
			"obj_type":   child.ObjType,
			"type":       nodeType,
			"url":        nodeURL,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"nodes":   nodes,
	})
}

// 生成并保存文档树文件
func saveWikiTreeHandler(c *gin.Context) {
	// 获取参数
	var request struct {
		OutputPath string   `json:"output_path"`
		SpaceName  string   `json:"space_name"`
		Tree       *DocNode `json:"tree"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	outputPath := request.OutputPath
	if outputPath == "" {
		outputPath = "output" // 默认输出路径
	}

	// 生成树状结构的文本文件
	treeText := generateTreeText(request.Tree, 0)

	// 确保输出目录存在
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("创建输出目录失败: %s", err),
		})
		return
	}

	// 保存树状结构文本到文件 - 使用空间名称作为文件名
	safeSpaceName := sanitizeFilename(request.SpaceName)
	treeFilePath := filepath.Join(outputPath, safeSpaceName+"_文档树.md")
	err := os.WriteFile(treeFilePath, []byte(treeText), 0644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("保存文档树文件失败: %s", err),
		})
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   fmt.Sprintf("成功生成文档树，已保存到 %s", treeFilePath),
		"file_path": treeFilePath,
	})
}

// 获取所有知识库空间列表
func getAllWikiSpacesHandler(c *gin.Context) {
	log.Printf("收到获取所有空间列表的请求")

	// 创建客户端
	ctx := context.Background()
	config := core.NewConfig(
		os.Getenv("FEISHU_APP_ID"),
		os.Getenv("FEISHU_APP_SECRET"),
	)
	log.Printf("应用凭证: AppID=%s, AppSecret=%s", config.Feishu.AppId, "***")

	client := core.NewClient(
		config.Feishu.AppId, config.Feishu.AppSecret,
	)

	// 获取所有知识库空间列表
	log.Printf("开始获取所有知识库空间列表...")
	spaces, err := client.GetAllWikiSpaces(ctx)
	if err != nil {
		log.Printf("获取知识库空间列表失败: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取知识库空间列表失败: %s", err),
		})
		return
	}

	log.Printf("成功获取到 %d 个知识库空间", len(spaces))

	// 构建返回数据
	var spacesList []gin.H
	for i, space := range spaces {
		log.Printf("空间 %d: ID=%s, 名称=%s", i+1, space.SpaceID, space.Name)
		spacesList = append(spacesList, gin.H{
			"space_id":   space.SpaceID,
			"space_name": space.Name,
			"url":        fmt.Sprintf("https://feishu.cn/wiki/space/%s", space.SpaceID),
		})
	}

	log.Printf("返回空间列表数据，共 %d 个空间", len(spacesList))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"spaces":  spacesList,
	})
}
