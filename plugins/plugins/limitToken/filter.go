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
	"strings"
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

func (f *filter) DecodeHeaders(headers api.RequestHeaderMap, endStream bool) api.ResultAction {

	keys, err := f.getKey(headers)
	if err != nil {
		api.LogErrorf("error getting key: %v", err)
		return api.Continue
	}

	if f.tokenRate(keys) {
		return &api.LocalResponse{Code: 409}
	}

	return api.Continue
}

func (f *filter) DecodeRequest(headers api.RequestHeaderMap, data api.BufferInstance, trailers api.RequestTrailerMap) api.ResultAction {

	promptTokens, err := f.extractPromptTokens(data.Bytes())
	if err != nil {
		api.LogErrorf("error extracting prompt tokens: %v", err)
		return api.Continue
	}

	if ok := f.config.tokenStat.IsExceeded(promptTokens); !ok {
		return &api.LocalResponse{Code: 409}
	}

	return api.Continue
}

func (f *filter) EncodeResponse(headers api.ResponseHeaderMap, data api.BufferInstance, trailers api.ResponseTrailerMap) api.ResultAction {

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

	f.config.tokenStat.Add(res.Usage.PromptTokens, res.Usage.CompletionTokens)

	return api.Continue
}

func (f *filter) extractPromptTokens(data []byte) (int, error) {

	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Model string `json:"model"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		return 0, err
	}

	var sb strings.Builder
	for _, m := range req.Messages {
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}

	text := sb.String()

	enc, err := tiktoken.EncodingForModel(req.Model)
	if err != nil {
		return 0, err
	}

	tokenCount := len(enc.Encode(text, nil, nil))
	return tokenCount, nil
}

func (f *filter) tokenRate(keys []string) bool {

	for _, key := range keys {
		limit := redis_rate.Limit{
			Rate:   int(f.config.Rule.Rate),
			Burst:  int(f.config.Rule.Burst),
			Period: time.Second,
		}

		ctx := context.Background()
		res, err := f.config.redisLimiter.Allow(ctx, key, limit)
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
