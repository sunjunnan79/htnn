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
	"github.com/go-redis/redis_rate/v10"
	"mosn.io/htnn/api/pkg/filtermanager/api"
	"mosn.io/htnn/types/plugins/limitToken"
	"regexp"
	"time"
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
	}

	if f.tokenRate(keys) {
		return &api.LocalResponse{Code: 409}
	}

	return api.Continue
}

func (f *filter) EncodeData(data api.BufferInstance, endStream bool) api.ResultAction {

	return api.Continue
}

func (f *filter) tokenRate(keys []string) bool {
	//对各个key进行限流,任何一个key只要被限流就会失败
	for _, key := range keys {
		limit := redis_rate.Limit{
			Rate:   int(f.config.Rule.Rate),
			Burst:  int(f.config.Rule.Burst),
			Period: time.Second,
		}

		ctx := context.Background()
		res, err := f.config.limiter.Allow(ctx, key, limit)
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
		raw, ok, isMatchMode = headers.Host(), true, true

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
		return f.match(raw)
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

func (f *filter) match(s string) ([]string, error) {
	result := make([]string, 0, len(f.config.Rule.Keys))
	for _, key := range f.config.Rule.Keys {
		reg, err := regexp.Compile(key)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp key %q: %w", key, err)
		}
		if matches := reg.FindStringSubmatch(s); len(matches) > 1 {
			result = append(result, matches[1])
		}
	}
	return result, nil
}
