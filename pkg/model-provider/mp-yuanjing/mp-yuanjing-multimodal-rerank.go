package mp_yuanjing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/UnicomAI/wanwu/pkg/log"
	mp_common "github.com/UnicomAI/wanwu/pkg/model-provider/mp-common"
	"github.com/UnicomAI/wanwu/pkg/util"
)

type MultiModalRerank struct {
	ApiKey              string   `json:"apiKey"`                     // ApiKey
	EndpointUrl         string   `json:"endpointUrl"`                // 推理url
	ContextSize         *int     `json:"contextSize"`                // 上下文长度
	MaxTextLength       *int64   `json:"maxTextLength"`              // 最大文本长度
	MaxImageSize        *int64   `json:"maxImageSize,omitempty"`     // 最大图片大小限制
	MaxVideoClipSize    *int64   `json:"maxVideoClipSize,omitempty"` // 最大视频片大小限制
	SupportFileTypes    []string `json:"supportFileTypes"`           // 支持的文件类型列表
	SupportImageInQuery bool     `json:"supportImageInQuery"`        // 是否支持query中传图片格式
}

func (cfg *MultiModalRerank) Tags() []mp_common.Tag {
	tags := []mp_common.Tag{
		{
			Text: mp_common.TagMultiModalRerank,
		},
	}
	tags = append(tags, mp_common.GetTagsByContentSize(cfg.ContextSize)...)
	return tags
}

// NewReq 主函数：仅负责参数透传、调用子函数、组装结果，逻辑极简
func (cfg *MultiModalRerank) NewReq(req *mp_common.MultiModalRerankReq) (mp_common.IMultiModalRerankReq, error) {
	m := map[string]interface{}{
		"model": req.Model,
	}
	if req.Instruction != nil {
		m["instruction"] = *req.Instruction
	}

	queryMap, err := processQuery(req.Query)
	if err != nil {
		return nil, err
	}
	m["query"] = queryMap

	docsMap, err := processDocuments(req.Documents)
	if err != nil {
		return nil, err
	}
	m["documents"] = docsMap

	if req.ReturnDocuments != nil {
		m["return_documents"] = *req.ReturnDocuments
	}

	if req.TopN != nil {
		m["top_n"] = *req.TopN
	}

	return mp_common.NewRerankReq(m), nil
}

func processDocuments(documents []mp_common.MultiDocument) (map[string]interface{}, error) {
	content := make([]map[string]interface{}, 0, len(documents))
	for idx, doc := range documents {
		item := make(map[string]interface{})
		if doc.Text != "" {
			item["type"] = "text"
			item["text"] = doc.Text
		} else if doc.Image != "" {
			// 图片类型
			item["type"] = "image_url"
			item["image_url"] = map[string]string{
				"url": doc.Image,
			}
		} else {
			return nil, fmt.Errorf("documents第%d个元素无效: image和text必选其一", idx+1)
		}
		content = append(content, item)
	}
	return map[string]interface{}{"content": content}, nil
}

func processQuery(query interface{}) (map[string]interface{}, error) {
	queryContent := make([]map[string]interface{}, 0, 2)

	switch q := query.(type) {
	case string:
		if q == "" {
			return nil, fmt.Errorf("query字符串不能为空")
		}
		queryContent = append(queryContent, map[string]interface{}{
			"type": "text",
			"text": q,
		})

	case map[string]interface{}:
		image, _ := q["image"].(string)
		text, _ := q["text"].(string)
		if image == "" && text == "" {
			return nil, fmt.Errorf("query对象无效: image和text必选其一，不能都为空")
		}
		if image != "" {
			queryContent = append(queryContent, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]string{
					"url": image,
				},
			})
		}
		if text != "" {
			queryContent = append(queryContent, map[string]interface{}{
				"type": "text",
				"text": text,
			})
		}

	default:
		return nil, fmt.Errorf("query类型不支持: %T，仅支持字符串或{image:string,text:string}对象", q)
	}

	return map[string]interface{}{"content": queryContent}, nil
}

func (cfg *MultiModalRerank) MultiModalRerank(ctx context.Context, req mp_common.IMultiModalRerankReq, headers ...mp_common.Header) (mp_common.IMultiModalRerankResp, error) {
	b, err := mp_common.MultiModalRerank(ctx, "yuanjing", cfg.ApiKey, cfg.rerankUrl(), req.Data(), headers...)
	if err != nil {
		return nil, err
	}
	return &multiRerankResp{raw: string(b)}, nil
}

func (cfg *MultiModalRerank) rerankUrl() string {
	ret, _ := url.JoinPath(cfg.EndpointUrl, "")
	return ret
}

// --- multiRerankResp ---
type multiRerankResp struct {
	raw     string
	ID      string                  `json:"id"`                          // 响应唯一标识，如rerank-adb238c7dd1adc38
	Model   string                  `json:"model" validate:"required"`   // 调用的模型名称，如qwen3-vl-reranker-8b
	Usage   mp_common.Usage         `json:"usage"`                       // 令牌使用统计
	Results []MultiRerankResultItem `json:"results" validate:"required"` // 重排序结果数组，按得分排序
}

// MultiRerankResultItem 单个重排序结果项（数组元素）
type MultiRerankResultItem struct {
	Index          int                 `json:"index"`           // 原文档在请求中的索引位置
	Document       MultiRerankDocument `json:"document"`        // 文档详情（多模态/纯文本兼容）
	RelevanceScore float64             `json:"relevance_score"` // 相关性得分，浮点型
}

// MultiRerankDocument 文档详情结构体（适配text/multi_modal互斥规则）
type MultiRerankDocument struct {
	Text       *string                `json:"text"`        // 纯文本场景文档内容，多模态场景为null（指针兼容null）
	MultiModal *MultiRerankMultiModal `json:"multi_modal"` // 多模态内容，纯文本场景为null（指针兼容null）
}

// MultiRerankMultiModal 多模态内容结构体（文本/图片二选一）
type MultiRerankMultiModal struct {
	Type     string          `json:"type"`                // 多模态类型：text/image_url
	Text     string          `json:"text,omitempty"`      // 文本类型内容，image_url类型为null/缺省
	ImageURL *RerankImageURL `json:"image_url,omitempty"` // 图片类型内容，text类型为null/缺省
}

// RerankImageURL 图片URL嵌套结构体（多模态图片场景专用）
type RerankImageURL struct {
	URL string `json:"url"` // 图片远程地址
}

func (resp *multiRerankResp) String() string {
	return resp.raw
}

func (resp *multiRerankResp) Data() (interface{}, bool) {
	ret := make(map[string]interface{})
	if err := json.Unmarshal([]byte(resp.raw), &ret); err != nil {
		log.Errorf("yuanjing multi_rerank resp (%v) convert to data err: %v", resp.raw, err)
		return nil, false
	}
	return ret, true
}

func (resp *multiRerankResp) ConvertResp() (*mp_common.MultiModalRerankResp, bool) {
	if err := json.Unmarshal([]byte(resp.raw), resp); err != nil {
		log.Errorf("yuanjing multi_rerank resp (%v) convert to data err: %v", resp.raw, err)
		return nil, false
	}

	if err := util.Validate(resp); err != nil {
		log.Errorf("yuanjing multi_rerank resp validate err: %v", err)
		return nil, false
	}

	res := &mp_common.MultiModalRerankResp{
		Model: resp.Model,
		Usage: resp.Usage,
	}

	res.Results = make([]mp_common.Result, 0, len(resp.Results))
	for _, item := range resp.Results {
		res.Results = append(res.Results, convertToMpResult(item))
	}

	return res, true
}

func convertToMpResult(item MultiRerankResultItem) mp_common.Result {
	result := mp_common.Result{
		Index:          item.Index,
		RelevanceScore: item.RelevanceScore,
	}

	// 定义document映射的临时map（平级结构，适配目标格式）
	docMap := make(map[string]string)

	// 分支1：多模态场景（优先匹配，模型返回主要为此场景）
	if item.Document.MultiModal != nil {
		modal := item.Document.MultiModal
		switch modal.Type {
		// 图片类型：提取image_url.url → document.url
		case "image_url":
			if modal.ImageURL != nil && modal.ImageURL.URL != "" {
				docMap["url"] = modal.ImageURL.URL
			} else {
				log.Errorf("yuanjing multi_rerank convertToMpResult: 图片类型但image_url/url为空, item: %+v", item)
				result.Document = nil
				return result
			}
		// 文本类型：提取text → document.text
		case "text":
			if modal.Text != "" {
				docMap["text"] = modal.Text
			} else {
				log.Errorf("yuanjing multi_rerank convertToMpResult: 文本类型但text为空, item: %+v", item)
				result.Document = nil
				return result
			}
		// 未知多模态类型：日志记录+空兜底
		default:
			log.Errorf("yuanjing multi_rerank convertToMpResult: 不支持的multi_modal类型: %s, item: %+v", modal.Type, item)
			result.Document = nil
			return result
		}
		// 分支2：纯文本场景（兼容原逻辑，备用）
	} else if item.Document.Text != nil && *item.Document.Text != "" {
		docMap["text"] = *item.Document.Text
		// 分支3：异常场景（无有效内容）
	} else {
		log.Errorf("yuanjing multi_rerank convertToMpResult: 文档无有效内容（text/multi_modal均为空）, item: %+v", item)
		result.Document = nil
		return result
	}

	// 赋值重塑后的document（map自动序列化为平级JSON对象）
	result.Document = docMap
	return result
}
