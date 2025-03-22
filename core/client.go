package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chyroc/lark"
	"github.com/chyroc/lark_rate_limiter"
)

type Client struct {
	larkClient *lark.Lark
	httpClient *http.Client // 添加 HTTP 客户端
	appID      string       // 添加应用 ID
	appSecret  string       // 添加应用密钥
}

func NewClient(appID, appSecret string) *Client {
	return &Client{
		larkClient: lark.New(
			lark.WithAppCredential(appID, appSecret),
			lark.WithTimeout(60*time.Second),
			lark.WithApiMiddleware(lark_rate_limiter.Wait(4, 4)),
		),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		appID:     appID,
		appSecret: appSecret,
	}
}

func (c *Client) DownloadImage(ctx context.Context, imgToken, outDir string) (string, error) {
	resp, _, err := c.larkClient.Drive.DownloadDriveMedia(ctx, &lark.DownloadDriveMediaReq{
		FileToken: imgToken,
	})
	if err != nil {
		return imgToken, err
	}
	fileext := filepath.Ext(resp.Filename)
	filename := fmt.Sprintf("%s/%s%s", outDir, imgToken, fileext)
	err = os.MkdirAll(filepath.Dir(filename), 0o755)
	if err != nil {
		return imgToken, err
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return imgToken, err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.File)
	if err != nil {
		return imgToken, err
	}
	return filename, nil
}

func (c *Client) DownloadImageRaw(ctx context.Context, imgToken, imgDir string) (string, []byte, error) {
	resp, _, err := c.larkClient.Drive.DownloadDriveMedia(ctx, &lark.DownloadDriveMediaReq{
		FileToken: imgToken,
	})
	if err != nil {
		return imgToken, nil, err
	}
	fileext := filepath.Ext(resp.Filename)
	filename := fmt.Sprintf("%s/%s%s", imgDir, imgToken, fileext)
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.File)
	return filename, buf.Bytes(), nil
}

func (c *Client) GetDocxContent(ctx context.Context, docToken string) (*lark.DocxDocument, []*lark.DocxBlock, error) {
	resp, _, err := c.larkClient.Drive.GetDocxDocument(ctx, &lark.GetDocxDocumentReq{
		DocumentID: docToken,
	})
	if err != nil {
		return nil, nil, err
	}
	docx := &lark.DocxDocument{
		DocumentID: resp.Document.DocumentID,
		RevisionID: resp.Document.RevisionID,
		Title:      resp.Document.Title,
	}
	var blocks []*lark.DocxBlock
	var pageToken *string
	for {
		resp2, _, err := c.larkClient.Drive.GetDocxBlockListOfDocument(ctx, &lark.GetDocxBlockListOfDocumentReq{
			DocumentID: docx.DocumentID,
			PageToken:  pageToken,
		})
		if err != nil {
			return docx, nil, err
		}
		blocks = append(blocks, resp2.Items...)
		pageToken = &resp2.PageToken
		if !resp2.HasMore {
			break
		}
	}
	return docx, blocks, nil
}

func (c *Client) GetWikiNodeInfo(ctx context.Context, token string) (*lark.GetWikiNodeRespNode, error) {
	resp, _, err := c.larkClient.Drive.GetWikiNode(ctx, &lark.GetWikiNodeReq{
		Token: token,
	})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *Client) GetDriveFolderFileList(ctx context.Context, pageToken *string, folderToken *string) ([]*lark.GetDriveFileListRespFile, error) {
	resp, _, err := c.larkClient.Drive.GetDriveFileList(ctx, &lark.GetDriveFileListReq{
		PageSize:    nil,
		PageToken:   pageToken,
		FolderToken: folderToken,
	})
	if err != nil {
		return nil, err
	}
	files := resp.Files
	for resp.HasMore {
		resp, _, err = c.larkClient.Drive.GetDriveFileList(ctx, &lark.GetDriveFileListReq{
			PageSize:    nil,
			PageToken:   &resp.NextPageToken,
			FolderToken: folderToken,
		})
		if err != nil {
			return nil, err
		}
		files = append(files, resp.Files...)
	}
	return files, nil
}

func (c *Client) GetWikiName(ctx context.Context, spaceID string) (string, error) {
	resp, _, err := c.larkClient.Drive.GetWikiSpace(ctx, &lark.GetWikiSpaceReq{
		SpaceID: spaceID,
	})

	if err != nil {
		return "", err
	}

	return resp.Space.Name, nil
}

func (c *Client) GetWikiNodeList(ctx context.Context, spaceID string, parentNodeToken *string) ([]*lark.GetWikiNodeListRespItem, error) {
	resp, _, err := c.larkClient.Drive.GetWikiNodeList(ctx, &lark.GetWikiNodeListReq{
		SpaceID:         spaceID,
		PageSize:        nil,
		PageToken:       nil,
		ParentNodeToken: parentNodeToken,
	})

	if err != nil {
		return nil, err
	}

	nodes := resp.Items

	for resp.HasMore {
		resp, _, err := c.larkClient.Drive.GetWikiNodeList(ctx, &lark.GetWikiNodeListReq{
			SpaceID:         spaceID,
			PageSize:        nil,
			PageToken:       &resp.PageToken,
			ParentNodeToken: parentNodeToken,
		})

		if err != nil {
			return nil, err
		}

		nodes = append(nodes, resp.Items...)
	}

	return nodes, nil
}

// WikiNode 定义知识库节点结构
type WikiNode struct {
	NodeToken string
	ObjToken  string
	ObjType   string
	Title     string
	SpaceID   string // 添加SpaceID字段
}

// GetWikiNodeChildren 获取知识库节点的子节点
func (c *Client) GetWikiNodeChildren(ctx context.Context, nodeToken string) ([]*WikiNode, error) {
	// 首先获取节点信息，以获取 SpaceID
	nodeInfo, _, err := c.larkClient.Drive.GetWikiNode(ctx, &lark.GetWikiNodeReq{
		Token: nodeToken,
	})
	if err != nil {
		return nil, fmt.Errorf("获取节点信息失败: %w", err)
	}

	// 使用正确的API调用获取子节点，提供 SpaceID
	resp, _, err := c.larkClient.Drive.GetWikiNodeList(ctx, &lark.GetWikiNodeListReq{
		SpaceID:         nodeInfo.Node.SpaceID,
		ParentNodeToken: &nodeToken,
	})
	if err != nil {
		return nil, err
	}

	var nodes []*WikiNode
	for _, item := range resp.Items {
		node := &WikiNode{
			NodeToken: item.NodeToken,
			ObjToken:  item.ObjToken,
			ObjType:   item.ObjType,
			Title:     item.Title,
			SpaceID:   nodeInfo.Node.SpaceID, // 保存SpaceID
		}
		nodes = append(nodes, node)
	}

	// 如果有更多页，继续获取
	for resp.HasMore {
		resp, _, err = c.larkClient.Drive.GetWikiNodeList(ctx, &lark.GetWikiNodeListReq{
			SpaceID:         nodeInfo.Node.SpaceID,
			PageToken:       &resp.PageToken,
			ParentNodeToken: &nodeToken,
		})
		if err != nil {
			return nodes, nil // 返回已获取的节点
		}

		for _, item := range resp.Items {
			node := &WikiNode{
				NodeToken: item.NodeToken,
				ObjToken:  item.ObjToken,
				ObjType:   item.ObjType,
				Title:     item.Title,
				SpaceID:   nodeInfo.Node.SpaceID, // 保存SpaceID
			}
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// WikiSpace 表示知识库空间信息
type WikiSpace struct {
	SpaceID string `json:"space_id"`
	Name    string `json:"name"`
}

// GetTenantAccessToken 获取租户访问令牌
func (c *Client) GetTenantAccessToken(ctx context.Context) (string, error) {
	// 构建请求体
	reqBody := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// 发送请求
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("获取访问令牌失败: %s (代码: %d)", result.Msg, result.Code)
	}

	return result.TenantAccessToken, nil
}

// GetAllWikiSpaces 获取用户可访问的所有知识库空间
func (c *Client) GetAllWikiSpaces(ctx context.Context) ([]*WikiSpace, error) {
	// 获取访问令牌
	token, err := c.GetTenantAccessToken(ctx)
	if err != nil {
		fmt.Printf("获取访问令牌失败: %v\n", err)
		return nil, err
	}
	fmt.Printf("成功获取访问令牌: %s...\n", token[:10])

	// 构建请求URL
	url := "https://open.feishu.cn/open-apis/wiki/v2/spaces"
	fmt.Printf("请求URL: %s\n", url)

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Printf("创建请求失败: %v\n", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	fmt.Printf("请求头: Authorization=Bearer %s..., Content-Type=%s\n", 
		token[:10], "application/json; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("发送请求失败: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()
	
	fmt.Printf("响应状态码: %d\n", resp.StatusCode)

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应体失败: %v\n", err)
		return nil, err
	}
	
	// 打印完整的响应体
	fmt.Printf("响应体: %s\n", string(body))

	var result struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				SpaceID string `json:"space_id"`
				Name    string `json:"name"`
			} `json:"items"`
		} `json:"data"`
		Msg string `json:"msg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("解析JSON失败: %v\n", err)
		return nil, err
	}

	if result.Code != 0 {
		fmt.Printf("API返回错误: 代码=%d, 消息=%s\n", result.Code, result.Msg)
		return nil, fmt.Errorf("API错误: %s (代码: %d)", result.Msg, result.Code)
	}

	// 构建返回结果
	spaces := make([]*WikiSpace, 0, len(result.Data.Items))
	for _, item := range result.Data.Items {
		fmt.Printf("找到空间: ID=%s, 名称=%s\n", item.SpaceID, item.Name)
		spaces = append(spaces, &WikiSpace{
			SpaceID: item.SpaceID,
			Name:    item.Name,
		})
	}

	fmt.Printf("总共找到 %d 个空间\n", len(spaces))
	return spaces, nil
}
