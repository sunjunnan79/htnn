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
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/pkoukk/tiktoken-go"
	"mosn.io/htnn/api/pkg/filtermanager/api"
	"mosn.io/htnn/types/plugins/limitToken"
)

const (
	ConsumerHeader string = "x-mse-consumer"
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

	keys, err := f.getKey(headers)
	if err != nil {
		api.LogErrorf("error getting key: %v", err)
		return api.Continue
	}

	// TODO 添加前置类型转化
	promptTokenLength, err := f.extractPromptTokens(data.Bytes())
	if err != nil {
		api.LogErrorf("error extracting prompt tokens: %v", err)
		return api.Continue
	}

	// 是否超出单位时间限制
	if ok := f.tokenRate(keys, promptTokenLength); !ok {
		return &api.LocalResponse{Code: 409}
	}

	//是否超出预测限制
	if ok := f.config.tokenStat.IsExceeded(promptTokenLength); !ok {
		return &api.LocalResponse{Code: 409}
	}

	return api.Continue
}

func (f *filter) EncodeResponse(headers api.ResponseHeaderMap, data api.BufferInstance, trailers api.ResponseTrailerMap) api.ResultAction {

	// TODO 增加前置类型转化
	var res struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	err := json.Unmarshal(data.Bytes(), &res)
	if err != nil {
		api.LogInfof("fail to unmarshal response: %v", err)
		return api.Continue
	}

	// 用于限流预测
	f.config.tokenStat.Add(res.Usage.PromptTokens, res.Usage.CompletionTokens)

	return api.Continue
}

// TODO 传入类型转化成openai接口范式并添加token计算函数
func (f *filter) extractPromptTokens(data []byte) (int, error) {

	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			Name    string `json:"name,omitempty"`
		} `json:"messages"`
		Model string `json:"model"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		return 0, fmt.Errorf("unmarshal request failed: %w", err)
	}

	// TODO Find a way to support other model
	tkm, err := tiktoken.EncodingForModel(req.Model)
	if err != nil {
		return 0, fmt.Errorf("get tokenizer failed: %w", err)
	}

	var tokensPerMessage, tokensPerName, finalAddition int
	switch req.Model {
	case "gpt-3.5-turbo-0613",
		"gpt-3.5-turbo-16k-0613",
		"gpt-4-0314",
		"gpt-4-32k-0314",
		"gpt-4-0613",
		"gpt-4-32k-0613":
		tokensPerMessage = 3
		tokensPerName = 1
		finalAddition = 3
	case "gpt-3.5-turbo-0301":
		tokensPerMessage = 4
		tokensPerName = -1
		finalAddition = 3
	default:
		return 0, fmt.Errorf("fallback model logic failed for model: %s", req.Model)
	}

	totalTokens := 0
	for _, msg := range req.Messages {
		totalTokens += tokensPerMessage
		totalTokens += len(tkm.Encode(msg.Role, nil, nil))
		totalTokens += len(tkm.Encode(msg.Content, nil, nil))
		if msg.Name != "" {
			totalTokens += len(tkm.Encode(msg.Name, nil, nil))
			totalTokens += tokensPerName
		}
	}
	totalTokens += finalAddition

	return totalTokens, nil
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
