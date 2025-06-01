// Copyright The HTNN Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package limitToken

import (
	"context"
	"fmt"
	"mosn.io/htnn/plugins/plugins/limitToken/provider"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"mosn.io/htnn/api/pkg/filtermanager/api"
	"mosn.io/htnn/types/plugins/limitToken"
)

const (
	ConsumerHeader         string = "x-mse-consumer"
	PromptToken            string = "prompt-token"
	PredictCompletionToken string = "predict-used-token"
	HistoryStreamDelta     string = "history-stream-delta"
	AllKeys                string = "all-keys"
)

func factory(c interface{}, callbacks api.FilterCallbackHandler) api.Filter {
	return &filter{
		callbacks: callbacks,
		config:    c.(*config),
	}
}

type filter struct {
	api.PassThroughFilter
	callbacks api.FilterCallbackHandler
	config    *config
}

func (f *filter) DecodeRequest(headers api.RequestHeaderMap, data api.BufferInstance, trailers api.RequestTrailerMap) api.ResultAction {
	reqID := f.getReqID()

	// 请求结构转换
	req, err := f.config.llmAdapter.ConvertRequestToHTNN(data.Bytes())
	if err != nil {
		api.LogErrorf("failed to convert request to HTNN format: %v", err)
		return nil
	}

	// 预测Token是否超限
	if exceeded := f.config.tokenStat.IsExceeded(req.PromptToken); !exceeded {
		api.LogErrorf("token exceeded for prompt: %d", req.PromptToken)
		return &api.LocalResponse{Code: 409}
	}

	// 获取keys
	keys, err := f.getKey(headers)
	if err != nil {
		api.LogErrorf("error getting key: %v", err)
		return api.Continue
	}

	// 预测CompletionToken
	predictCompletionToken := f.config.tokenStat.PredictCompletionTokens(req.PromptToken)

	// 单位时间内 token 是否合规
	if ok := f.tokenRate(keys, predictCompletionToken); !ok {
		api.LogErrorf("token rate exceeded in DecodeRequest, keys: %v, token: %d", keys, predictCompletionToken)
		return &api.LocalResponse{Code: 409}
	}

	// 保存上下文信息
	f.callbacks.PluginState().Set(reqID, PredictCompletionToken, predictCompletionToken)
	f.callbacks.PluginState().Set(reqID, AllKeys, keys)
	f.callbacks.PluginState().Set(reqID, PromptToken, req.PromptToken)

	return api.Continue
}

func (f *filter) EncodeResponse(headers api.ResponseHeaderMap, data api.BufferInstance, trailers api.ResponseTrailerMap) api.ResultAction {
	reqID := f.getReqID()

	// 解析响应
	resp, err := f.config.llmAdapter.ConvertResponseFromHTNN(data.Bytes())
	if err != nil {
		api.LogErrorf("failed to convert response from HTNN: %v", err)
		return nil
	}

	// 获取 AllKeys
	value := f.callbacks.PluginState().Get(reqID, AllKeys)
	if value == nil {
		api.LogErrorf("missing AllKeys in PluginState, reqID: %s", reqID)
		return nil
	}
	keys, ok := value.([]string)
	if !ok {
		api.LogErrorf("invalid type for AllKeys, expected []string, got: %T", value)
		return nil
	}

	// 获取预测值
	value = f.callbacks.PluginState().Get(reqID, PredictCompletionToken)
	if value == nil {
		api.LogErrorf("missing PredictCompletionToken in PluginState, reqID: %s", reqID)
		return nil
	}
	predictCompletionToken, ok := value.(int)
	if !ok {
		api.LogErrorf("invalid type for PredictCompletionToken, expected int, got: %T", value)
		return nil
	}

	// 计算实际差值并判断单位时间速率
	tokenGap := resp.CompletionTokens - predictCompletionToken
	if ok := f.tokenRate(keys, tokenGap); !ok {
		api.LogErrorf("token rate exceeded in EncodeResponse, tokenGap: %d", tokenGap)
		return &api.LocalResponse{Code: 409}
	}

	// 清理缓存
	f.callbacks.PluginState().Set(reqID, AllKeys, nil)
	f.callbacks.PluginState().Set(reqID, PredictCompletionToken, nil)
	f.callbacks.PluginState().Set(reqID, PromptToken, nil)

	// 上报统计
	f.config.tokenStat.Add(resp.PromptTokens, resp.CompletionTokens)

	return api.Continue
}

func (f *filter) EncodeData(data api.BufferInstance, endStream bool) api.ResultAction {
	reqID := f.getReqID()

	// 获取历史 delta
	value := f.callbacks.PluginState().Get(reqID, HistoryStreamDelta)
	var delta *provider.HTNNStreamDelta
	if value != nil {
		var ok bool
		delta, ok = value.(*provider.HTNNStreamDelta)
		if !ok {
			api.LogErrorf("invalid type for HistoryStreamDelta, expected *HTNNStreamDelta, got: %T", value)
			return nil
		}
	} else {
		delta = &provider.HTNNStreamDelta{}
	}

	// 更新 delta
	newDelta, err := f.config.llmAdapter.ConvertStreamChunkFromHTNN(delta, data.Bytes())
	if err != nil {
		api.LogErrorf("failed to convert stream chunk: %v", err)
		return nil
	}
	delta = newDelta

	if delta.Finish {
		// 获取 key 列表
		value = f.callbacks.PluginState().Get(reqID, AllKeys)
		if value == nil {
			api.LogErrorf("missing AllKeys from PluginState, reqID: %s", reqID)
			return nil
		}
		keys, ok := value.([]string)
		if !ok {
			api.LogErrorf("invalid type for AllKeys, expected []string, got: %T", value)
			return nil
		}

		// 获取预测 token
		value = f.callbacks.PluginState().Get(reqID, PredictCompletionToken)
		if value == nil {
			api.LogErrorf("missing PredictCompletionToken from PluginState, reqID: %s", reqID)
			return nil
		}
		predictCompletionToken, ok := value.(int)
		if !ok {
			api.LogErrorf("invalid type for PredictCompletionToken, expected int, got: %T", value)
			return nil
		}

		// 检查 token 是否超限
		tokenGap := delta.CompletionTokens - predictCompletionToken
		if ok := f.tokenRate(keys, tokenGap); !ok {
			return &api.LocalResponse{Code: 409}
		}

		// 清理插件状态
		f.callbacks.PluginState().Set(reqID, AllKeys, nil)
		f.callbacks.PluginState().Set(reqID, PredictCompletionToken, nil)
		f.callbacks.PluginState().Set(reqID, HistoryStreamDelta, nil)

		// 获取 prompt token
		value = f.callbacks.PluginState().Get(reqID, PromptToken)
		if value == nil {
			api.LogErrorf("missing PromptToken from PluginState, reqID: %s", reqID)
			return nil
		}
		promptToken, ok := value.(int)
		if !ok {
			api.LogErrorf("invalid type for PromptToken, expected int, got: %T", value)
			return nil
		}

		// 上报统计
		f.config.tokenStat.Add(promptToken, delta.CompletionTokens)
	} else {

		// 非 Finish，暂存 delta
		f.callbacks.PluginState().Set(reqID, HistoryStreamDelta, delta)
	}

	return api.Continue
}

func (f *filter) tokenRate(keys []string, promptTokenLength int) bool {
	for _, key := range keys {
		limit := redis_rate.Limit{
			Rate:   int(f.config.Rule.Rate),
			Burst:  int(f.config.Rule.Burst),
			Period: time.Second,
		}

		ctx := context.Background()
		res, err := f.config.redisLimiter.AllowN(ctx, key, limit, promptTokenLength)
		if err != nil {
			api.LogErrorf("limitReq filter Redis error: %v", err)
			return false
		}

		if res.Allowed == 0 {
			api.LogInfof("limitReq filter, key: %s, denied: too many requests", key)
			return false
		}

		api.LogInfof("limitReq filter, key: %s, allowed: %d, retry after: %s", key, res.Allowed, res.RetryAfter)
	}

	return true
}

func (f *filter) getReqID() string {
	// 生成请求唯一标识
	streamInfo := f.callbacks.StreamInfo()
	workerID := streamInfo.WorkerID()
	remoteAddr := streamInfo.DownstreamRemoteParsedAddress()
	reqID := fmt.Sprintf("%d-%s-%d-%d",
		workerID,
		remoteAddr.IP,
		remoteAddr.Port,
		streamInfo.AttemptCount())

	return reqID
}

func (f *filter) getKey(headers api.RequestHeaderMap) ([]string, error) {
	var raw string
	var ok, isMatchMode bool

	switch v := f.config.Rule.LimitBy.(type) {
	case *limitToken.Rule_LimitByHeader:
		raw, ok = headers.Get(v.LimitByHeader)

	case *limitToken.Rule_LimitByParam:
		raw, ok = f.getQueryValue(headers, v.LimitByParam)

	case *limitToken.Rule_LimitByCookie:
		raw, ok = f.getCookieValue(headers, v.LimitByCookie)

	case *limitToken.Rule_LimitByConsumer:
		raw, ok = headers.Get(ConsumerHeader)

	case *limitToken.Rule_LimitByPerIp:
		raw, ok = headers.Host(), true
		isMatchMode = true

	case *limitToken.Rule_LimitByPerHeader:
		raw, ok = headers.Get(v.LimitByPerHeader)
		isMatchMode = true

	case *limitToken.Rule_LimitByPerParam:
		raw, ok = f.getQueryValue(headers, v.LimitByPerParam)
		isMatchMode = true

	case *limitToken.Rule_LimitByPerCookie:
		raw, ok = f.getCookieValue(headers, v.LimitByPerCookie)
		isMatchMode = true

	case *limitToken.Rule_LimitByPerConsumer:
		raw, ok = headers.Get(ConsumerHeader)
		isMatchMode = true

	default:
		return nil, fmt.Errorf("unknown limit type: %v", f.config.Rule.LimitBy)
	}

	if !ok {
		return nil, nil
	}

	if isMatchMode {
		result := make([]string, 0, len(f.config.regexps))
		for _, reg := range f.config.regexps {
			if matches := reg.FindStringSubmatch(raw); len(matches) > 1 {
				result = append(result, matches[1])
			}
		}
		return result, nil
	}

	return []string{raw}, nil
}

func (f *filter) getQueryValue(headers api.RequestHeaderMap, key string) (string, bool) {
	val := headers.URL().Query().Get(key)
	if val != "" {
		return val, true
	}
	return "", false
}

func (f *filter) getCookieValue(headers api.RequestHeaderMap, key string) (string, bool) {
	cookie := headers.Cookie(key)
	if cookie != nil {
		return cookie.String(), true
	}
	return "", false
}
