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
	"strings"
	"time" // 添加time包导入

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

	// Validate the url to download
	docType, docToken, err := utils.ValidateDocumentURL(feishu_docx_url)
	fmt.Println("Captured document token:", docToken)

	// Create client with context
	ctx := context.Background()
	config := core.NewConfig(
		os.Getenv("FEISHU_APP_ID"),
		os.Getenv("FEISHU_APP_SECRET"),
	)

	// 更新配置中的输出路径
	config.Output.ImageDir = filepath.Join(outputPath, "images")

	// 确保输出目录存在
	if err := os.MkdirAll(config.Output.ImageDir, 0755); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("创建输出目录失败: %s", err))
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
		node, err := client.GetWikiNodeInfo(ctx, docToken)
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal error: client.GetWikiNodeInfo")
			log.Panicf("error: %s", err)
			return
		}
		docType = node.ObjType
		docToken = node.ObjToken
	}
	if docType == "docs" {
		c.String(http.StatusBadRequest, "Unsupported docs document type")
		return
	}

	docx, blocks, err := client.GetDocxContent(ctx, docToken)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal error: client.GetDocxContent")
		log.Panicf("error: %s", err)
		return
	}
	markdown = parser.ParseDocxContent(docx, blocks)

	// 获取文档标题并处理为合法文件名
	docTitle := docx.Title
	if docTitle == "" {
		docTitle = docToken // 如果标题为空，仍使用token作为备选
	}
	// 替换文件名中的非法字符
	docTitle = sanitizeFilename(docTitle)

	zipBuffer := new(bytes.Buffer)
	writer := zip.NewWriter(zipBuffer)
	for _, imgToken := range parser.ImgTokens {
		localLink, rawImage, err := client.DownloadImageRaw(ctx, imgToken, config.Output.ImageDir)
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal error: client.DownloadImageRaw")
			log.Panicf("error: %s", err)
			return
		}
		markdown = strings.Replace(markdown, imgToken, localLink, 1)
		f, err := writer.Create(localLink)
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal error: zipWriter.Create")
			log.Panicf("error: %s", err)
			return
		}
		_, err = f.Write(rawImage)
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal error: zipWriter.Create.Write")
			log.Panicf("error: %s", err)
			return
		}
	}

	engine := lute.New(func(l *lute.Lute) {
		l.RenderOptions.AutoSpace = true
	})
	result := engine.FormatStr("md", markdown)

	// Set response
	if len(parser.ImgTokens) > 0 {
		mdName := fmt.Sprintf("%s.md", docTitle)
		f, err := writer.Create(mdName)
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal error: zipWriter.Create")
			log.Panicf("error: %s", err)
			return
		}
		_, err = f.Write([]byte(result))
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal error: zipWriter.Create.Write")
			log.Panicf("error: %s", err)
			return
		}

		err = writer.Close()
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal error: zipWriter.Close")
			log.Panicf("error: %s", err)
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, docTitle))
		c.Data(http.StatusOK, "application/octet-stream", zipBuffer.Bytes())
	} else {
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, docTitle))
		c.Data(http.StatusOK, "application/octet-stream", []byte(result))
	}
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

	// 添加调试日志
	fmt.Printf("空间名称: %s, 节点Token: %s\n", rootNode.Title, rootNode.NodeToken)

	// 创建根文档节点
	docTree := &DocNode{
		Title:    rootNode.Title,
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
	spaceName := sanitizeFilename(rootNode.Title)
	// 添加调试日志
	fmt.Printf("处理后的空间名称: %s\n", spaceName)
	treeFilePath := filepath.Join(outputPath, spaceName+"_文档树.md")
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
