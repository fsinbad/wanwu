package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	sse_util "github.com/UnicomAI/wanwu/pkg/sse-util"

	"github.com/UnicomAI/wanwu/internal/assistant-service/service"

	assistant_service "github.com/UnicomAI/wanwu/api/proto/assistant-service"
	errs "github.com/UnicomAI/wanwu/api/proto/err-code"
	"github.com/UnicomAI/wanwu/internal/assistant-service/client/model"
	"github.com/UnicomAI/wanwu/internal/assistant-service/config"
	"github.com/UnicomAI/wanwu/pkg/es"
	grpc_util "github.com/UnicomAI/wanwu/pkg/grpc-util"
	http_client "github.com/UnicomAI/wanwu/pkg/http-client"
	"github.com/UnicomAI/wanwu/pkg/log"
	"github.com/UnicomAI/wanwu/pkg/util"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ConversationCreate 创建对话
func (s *Service) ConversationCreate(ctx context.Context, req *assistant_service.ConversationCreateReq) (*assistant_service.ConversationCreateResp, error) {
	// 组装model参数
	conversation := &model.Conversation{
		AssistantId:      util.MustU32(req.AssistantId),
		Title:            req.Prompt, // 使用prompt作为初始标题
		ConversationType: req.ConversationType,
		UserId:           req.Identity.UserId,
		OrgId:            req.Identity.OrgId,
	}

	// 调用client方法创建对话
	if status := s.cli.CreateConversation(ctx, conversation); status != nil {
		return nil, errStatus(errs.Code_AssistantConversationErr, status)
	}

	return &assistant_service.ConversationCreateResp{
		ConversationId: util.Int2Str(conversation.ID),
	}, nil
}

// ConversationDelete 删除对话
func (s *Service) ConversationDelete(ctx context.Context, req *assistant_service.ConversationDeleteReq) (*emptypb.Empty, error) {
	// 转换ID
	conversationID, err := strconv.ParseUint(req.ConversationId, 10, 32)
	if err != nil {
		return nil, err
	}

	// 调用client方法删除对话
	if status := s.cli.DeleteConversation(ctx, uint32(conversationID)); status != nil {
		return nil, errStatus(errs.Code_AssistantConversationErr, status)
	}

	// 删除es中的对话详情
	fieldConditions := map[string]interface{}{
		"conversationId.keyword": req.ConversationId,
		"userId.keyword":         req.Identity.UserId,
	}
	indexPattern := "conversation_detail_infos_*"
	if err := es.Assistant().DeleteByFields(ctx, indexPattern, fieldConditions); err != nil {
		log.Errorf("从ES删除对话详情失败，conversationId: %s, error: %v", req.ConversationId, err)
	}

	return &emptypb.Empty{}, nil
}

// GetConversationIdByAssistantId 获取对话记录id
func (s *Service) GetConversationIdByAssistantId(ctx context.Context, req *assistant_service.GetConversationIdByAssistantIdReq) (*assistant_service.ConversationIdResp, error) {
	// 调用client方法获取对话
	conversation, status := s.cli.GetConversationByAssistantID(ctx, req.AssistantId, req.ConversationType)
	if status != nil {
		return nil, errStatus(errs.Code_AssistantConversationErr, status)
	}

	return &assistant_service.ConversationIdResp{
		ConversationId: util.Int2Str(conversation.ID),
	}, nil
}

// GetConversationList 对话列表
func (s *Service) GetConversationList(ctx context.Context, req *assistant_service.GetConversationListReq) (*assistant_service.GetConversationListResp, error) {
	// 计算offset
	offset := (req.PageNo - 1) * req.PageSize

	// 调用client方法获取对话列表
	conversations, total, status := s.cli.GetConversationList(ctx, req.AssistantId, req.ConversationType, req.Identity.UserId, req.Identity.OrgId, offset, req.PageSize)
	if status != nil {
		return nil, errStatus(errs.Code_AssistantConversationErr, status)
	}

	// 转换为响应格式
	var conversationInfos []*assistant_service.ConversationInfo
	for _, conversation := range conversations {
		conversationInfos = append(conversationInfos, &assistant_service.ConversationInfo{
			ConversationId: util.Int2Str(conversation.ID),
			AssistantId:    util.Int2Str(conversation.AssistantId),
			Title:          conversation.Title,
			CreatTime:      conversation.CreatedAt,
		})
	}

	return &assistant_service.GetConversationListResp{
		Data:     conversationInfos,
		Total:    total,
		PageSize: req.PageSize,
		PageNo:   req.PageNo,
	}, nil
}

// GetConversationDetailList 对话详情历史列表
func (s *Service) GetConversationDetailList(ctx context.Context, req *assistant_service.GetConversationDetailListReq) (*assistant_service.GetConversationDetailListResp, error) {
	// 计算分页参数
	from := (req.PageNo - 1) * req.PageSize
	size := int(req.PageSize)

	// 组装查询条件
	fieldConditions := map[string]interface{}{
		"conversationId": req.ConversationId,
		"userId.keyword": req.Identity.UserId,
		"orgId.keyword":  req.Identity.OrgId,
	}

	// 使用通配符查询所有对话详情索引
	indexPattern := "conversation_detail_infos_*"

	// 从ES查询数据
	documents, total, err := es.Assistant().SearchByFields(ctx, indexPattern, fieldConditions, int(from), size, "desc")
	if err != nil {
		log.Errorf("从ES查询对话详情失败，conversationId: %s, userId: %s, error: %v", req.ConversationId, req.Identity.UserId, err)
		return nil, fmt.Errorf("查询对话详情失败: %v", err)
	}

	// 转换查询结果为响应格式
	var conversationDetails []*assistant_service.ConversionDetailInfo
	for _, doc := range documents {
		var detail model.ConversationDetails
		if err := json.Unmarshal(doc, &detail); err != nil {
			log.Warnf("解析ES文档失败: %v", err)
			continue
		}

		conversationDetails = append(conversationDetails, &assistant_service.ConversionDetailInfo{
			Id:                   detail.Id,
			AssistantId:          detail.AssistantId,
			ConversationId:       detail.ConversationId,
			Prompt:               detail.Prompt,
			SysPrompt:            detail.SysPrompt,
			Response:             detail.Response,
			SearchList:           detail.SearchList,
			QaType:               detail.QaType,
			CreatedBy:            detail.UserId, // 使用CreatedBy字段映射UserId
			CreatedAt:            detail.CreatedAt,
			UpdatedAt:            detail.UpdatedAt,
			RequestFiles:         transRequestFiles(detail.FileInfo),
			FileSize:             detail.FileSize,
			FileName:             detail.FileName,
			SubConversationList:  buildSubConversationList(detail.SubConversationDetailList, len(detail.ResponseList) == 0),
			ConversationResponse: buildConversationResponse(detail.Response, detail.ResponseList, len(detail.SubConversationDetailList)),
		})
	}

	log.Infof("成功从ES查询对话详情，conversationId: %s, userId: %s, 总数: %d, 返回: %d",
		req.ConversationId, req.Identity.UserId, total, len(conversationDetails))

	return &assistant_service.GetConversationDetailListResp{
		Data:     conversationDetails,
		Total:    total,
		PageSize: req.PageSize,
		PageNo:   req.PageNo,
	}, nil
}

func (s *Service) AssistantConversionStream(req *assistant_service.AssistantConversionStreamReq, stream assistant_service.AssistantService_AssistantConversionStreamServer) error {
	//会话处理
	conversationProcessor := &service.ConversationProcessor{
		SSEWriter: sse_util.NewGrpcSSEWriter(stream, "AssistantConversionStreamNew", nil),
	}
	err := conversationProcessor.Process(stream.Context(), buildConversationParams(req), buildAgentSendRequest(req))
	if err != nil {
		log.Errorf("Assistant服务处理智能体流式对话失败，assistantId: %s, error: %v", req.AssistantId, err)
		return grpc_util.ErrorStatusWithKey(errs.Code_AssistantConversationErr, "assistant_conversation", "agent服务异常")
	}
	return nil
}

// extractFileInfos 从proto FileInfo中提取所有文件信息到model FileInfo
func extractFileInfos(fileInfos []*assistant_service.ConversionStreamFile) []model.FileInfo {
	if len(fileInfos) == 0 {
		return nil
	}
	var result []model.FileInfo
	for _, file := range fileInfos {
		if file != nil {
			result = append(result, model.FileInfo{
				FileName: file.FileName,
				FileSize: file.FileSize,
				FileUrl:  file.FileUrl,
			})
		}
	}
	return result
}

// extractFileUrls 从proto FileInfo中提取所有文件URL
func extractFileUrls(fileInfos []*assistant_service.ConversionStreamFile) []string {
	if len(fileInfos) == 0 {
		return nil
	}
	var fileUrls []string
	for _, file := range fileInfos {
		if file != nil && file.FileUrl != "" {
			fileUrls = append(fileUrls, file.FileUrl)
		}
	}
	return fileUrls
}

// transRequestFiles 将 model.FileInfo 转换为 assistant_service.RequestFile，并替换 fileUrl 为 minio 对外下载 url
func transRequestFiles(files []model.FileInfo) []*assistant_service.RequestFile {
	if files == nil {
		return nil
	}

	downloadURL := os.Getenv("MINIO_DOWNLOAD_URL")
	minioEndpoint := os.Getenv("MINIO_ENDPOINT")

	var result []*assistant_service.RequestFile
	for _, file := range files {
		// 替换 fileUrl 为 minio 对外下载 url
		replacedUrl := strings.Replace(file.FileUrl, "http://"+minioEndpoint+"/", downloadURL, 1)

		result = append(result, &assistant_service.RequestFile{
			FileName: file.FileName,
			FileSize: file.FileSize,
			FileUrl:  replacedUrl,
		})
	}
	return result
}

func buildConversationParams(req *assistant_service.AssistantConversionStreamReq) *service.ConversationParams {
	return &service.ConversationParams{
		AssistantId:    req.AssistantId,
		ConversationId: req.ConversationId,
		FileInfo:       extractFileInfos(req.FileInfo),
		OrgId:          req.Identity.OrgId,
		Query:          req.Prompt,
		UserId:         req.Identity.UserId,
	}
}

// buildAgentSendRequest 构建底层智能体能力接口请求体
func buildAgentSendRequest(req *assistant_service.AssistantConversionStreamReq) func(ctx context.Context) (string, *http.Response, context.CancelFunc, error) {
	var conversationID string
	// 历史聊天记录配置
	if req.ConversationId != "" {
		conversationID = req.ConversationId
	}
	// 底层智能体能力接口请求体
	chatReq := service.BuildAgentChatReq(&service.AgentUserInputParams{
		Input:          req.Prompt,
		Stream:         true,
		UploadFile:     extractFileUrls(req.FileInfo),
		ConversationId: conversationID,
		UserId:         req.Identity.UserId,
		OrgId:          req.Identity.OrgId,
		Draft:          req.Draft,
	}, util.MustU32(req.AssistantId))

	var monitorKey = "agent_chat_service"

	return func(ctx context.Context) (string, *http.Response, context.CancelFunc, error) {
		paramsBytes, err := json.Marshal(chatReq)
		if err != nil {
			return monitorKey, nil, nil, err
		}
		// 获取Assistant配置
		assistantConfig := config.Cfg().Assistant
		if assistantConfig.NewSseUrl == "" {
			return monitorKey, nil, nil, errors.New("智能体SSE URL配置错误")
		}
		params := &http_client.HttpRequestParams{
			Body:       paramsBytes,
			Timeout:    5 * time.Minute,
			Url:        assistantConfig.NewSseUrl,
			MonitorKey: monitorKey,
			LogLevel:   http_client.LogAll,
		}
		ctx, cancelFunction := context.WithTimeout(ctx, params.Timeout)
		result, err := http_client.Default().PostJsonOriResp(ctx, params)
		return monitorKey, result, cancelFunction, err
	}
}

func buildConversationResponse(response string, conversation []*model.ConversationResponse, startOrder int) []*assistant_service.ConversationResponse {
	if len(conversation) == 0 {
		return []*assistant_service.ConversationResponse{{Response: response, Order: int32(startOrder)}}
	}
	var retList []*assistant_service.ConversationResponse
	for _, resp := range conversation {
		retList = append(retList, &assistant_service.ConversationResponse{
			Response: resp.Response,
			Order:    int32(resp.Order),
		})
	}
	return retList
}

func buildSubConversationList(subConversationDetailList []*model.SubConversationDetail, oldData bool) []*assistant_service.SubConversation {
	if len(subConversationDetailList) == 0 {
		return make([]*assistant_service.SubConversation, 0)
	}
	var retList []*assistant_service.SubConversation
	for idx, detail := range subConversationDetailList {
		retList = append(retList, buildSubConversation(detail, idx, oldData))
	}
	return retList
}

func buildSubConversation(detail *model.SubConversationDetail, index int, oldData bool) *assistant_service.SubConversation {
	data := detail.EventData
	if data == nil {
		data = &model.SubEventData{}
	}
	var order = detail.Order
	if oldData {
		order = index
	}
	return &assistant_service.SubConversation{
		Response:         detail.Content,
		SearchList:       detail.SearchList,
		Id:               data.Id,
		Name:             data.Name,
		Profile:          data.Profile,
		TimeCost:         data.TimeCost,
		Status:           int32(data.Status),
		ConversationType: string(detail.ConversationType),
		Order:            int32(order),
	}
}
