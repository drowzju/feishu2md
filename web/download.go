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

// 文档链接信息结构
type DocLinkInfo struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

// 知识库文档链接列表响应
type WikiDocsResponse struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Docs    []DocLinkInfo `json:"docs"`
}

// 获取知识库所有文档链接的处理函数
func getWikiDocsHandler(c *gin.Context) {
	// 获取参数
	wikiURL, err := url.QueryUnescape(c.Query("url"))
	if err != nil {
		c.JSON(http.StatusBadRequest, WikiDocsResponse{
			Success: false,
			Message: "无效的知识库URL",
		})
		return
	}

	// 验证知识库URL
	docType, spaceToken, err := utils.ValidateDocumentURL(wikiURL)
	if err != nil || docType != "wiki" {
		c.JSON(http.StatusBadRequest, WikiDocsResponse{
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
		c.JSON(http.StatusInternalServerError, WikiDocsResponse{
			Success: false,
			Message: fmt.Sprintf("获取知识库信息失败: %s", err),
		})
		return
	}

	// 将 lark.GetWikiNodeRespNode 转换为 core.WikiNode
	wikiRootNode := &core.WikiNode{
		NodeToken: rootNode.NodeToken,
		ObjToken:  rootNode.ObjToken,
		ObjType:   rootNode.ObjType,
		Title:     rootNode.Title,
		SpaceID:   rootNode.SpaceID,
	}

	// 递归获取所有文档链接
	var docLinks []DocLinkInfo

	// 首先处理根节点本身
	if wikiRootNode.ObjType == "docx" || wikiRootNode.ObjType == "doc" {
		docURL := fmt.Sprintf("https://feishu.cn/docx/%s", wikiRootNode.ObjToken)
		docLinks = append(docLinks, DocLinkInfo{
			Title: wikiRootNode.Title,
			URL:   docURL,
			Type:  wikiRootNode.ObjType,
		})
		fmt.Printf("添加根文档: %s\n", wikiRootNode.Title)
	}

	// 获取知识库中的顶级节点列表
	topNodes, err := client.GetWikiNodeList(ctx, wikiRootNode.SpaceID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, WikiDocsResponse{
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
			SpaceID:   wikiRootNode.SpaceID,
		}

		err = collectWikiDocs(ctx, client, topNode, &docLinks)
		if err != nil {
			fmt.Printf("处理顶级节点 %s 失败: %s\n", topNode.Title, err)
			// 继续处理其他节点
		}
	}

	// 返回文档链接列表
	c.JSON(http.StatusOK, WikiDocsResponse{
		Success: true,
		Message: fmt.Sprintf("成功获取 %d 个文档链接", len(docLinks)),
		Docs:    docLinks,
	})
}

// 递归收集知识库中的所有文档链接
func collectWikiDocs(ctx context.Context, client *core.Client, node *core.WikiNode, docLinks *[]DocLinkInfo) error {
	// 打印当前处理的节点信息，帮助调试
	fmt.Printf("处理节点: %s, 类型: %s\n", node.Title, node.ObjType)

	// 如果是文档节点，添加到链接列表
	if node.ObjType == "docx" || node.ObjType == "doc" {
		// 使用更通用的URL格式
		docURL := fmt.Sprintf("https://feishu.cn/docx/%s", node.ObjToken)
		*docLinks = append(*docLinks, DocLinkInfo{
			Title: node.Title,
			URL:   docURL,
			Type:  node.ObjType,
		})
		fmt.Printf("添加文档: %s\n", node.Title)
	}

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
		err := collectWikiDocs(ctx, client, child, docLinks)
		if err != nil {
			log.Printf("处理子节点 %s 失败: %s", child.Title, err)
			// 继续处理其他子节点，不中断
		}
	}

	return nil
}
