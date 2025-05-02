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
    "regexp"
    "sort"
    "sync"

    "github.com/go-redis/redis_rate/v10"
    "github.com/redis/go-redis/v9"
    "mosn.io/htnn/api/pkg/filtermanager/api"
    "mosn.io/htnn/api/pkg/plugins"
    "mosn.io/htnn/types/plugins/limitToken"
)

func init() {
    plugins.RegisterPlugin(limitToken.Name, &plugin{})
}

type plugin struct {
    limitToken.Plugin
}

func (p *plugin) Factory() api.FilterFactory {
    return factory
}

func (p *plugin) Config() api.PluginConfig {
    return &config{}
}

type config struct {
    limitToken.CustomConfig
    redisLimiter *redis_rate.Limiter
    tokenStat    *TokenStats
    regexps      []*regexp.Regexp
}

func (conf *config) Init(cb api.ConfigCallbackHandler) error {
    if err := conf.initRedisLimiter(); err != nil {
        return err
    }

    if err := conf.initTokenStats(); err != nil {
        return err
    }

    if err := conf.initRegexps(); err != nil {
        return err
    }

    return nil
}

func (conf *config) initRegexps() error {
    conf.regexps = make([]*regexp.Regexp, 0, len(conf.Rule.Keys))
    for _, pattern := range conf.Rule.Keys {
        re, err := regexp.Compile(pattern)
        if err != nil {
            return fmt.Errorf("invalid regexp key %q: %w", pattern, err)
        }
        conf.regexps = append(conf.regexps, re)
    }
    return nil
}

func (conf *config) initRedisLimiter() error {
    rdb := redis.NewClient(&redis.Options{
        Addr:     conf.Redis.ServiceAddr,
        Username: conf.Redis.Username,
        Password: conf.Redis.Password,
    })

    if err := rdb.Ping(context.Background()).Err(); err != nil {
        return fmt.Errorf("redis connection failed: %w", err)
    }

    conf.redisLimiter = redis_rate.NewLimiter(rdb)

    return nil
}

func (conf *config) initTokenStats() error {
    // TODO Add invalid input check
    const (
        DefaultWindowSize      = 1000
        DefaultMinSamples      = 10
        DefaultMaxRatio        = 4.0
        DefaultMaxTokensPerReq = 2000
        DefaultExceedFactor    = 1.5
    )

    cfg := conf.TokenStats
    if cfg == nil {
        cfg = &limitToken.TokenStatsConfig{}
    }

    windowSize := int(cfg.WindowSize)
    if windowSize == 0 {
        windowSize = DefaultWindowSize
    }

    minSamples := int(cfg.MinSamples)
    if minSamples == 0 {
        minSamples = DefaultMinSamples
    }

    maxRatio := float64(cfg.MaxRatio)
    if maxRatio == 0 {
        maxRatio = DefaultMaxRatio
    }

    maxTokensPerReq := int(cfg.MaxTokensPerReq)
    if maxTokensPerReq == 0 {
        maxTokensPerReq = DefaultMaxTokensPerReq
    }

    exceedFactor := float64(cfg.ExceedFactor)
    if exceedFactor == 0 {
        exceedFactor = DefaultExceedFactor
    }

    conf.tokenStat = &TokenStats{
        windowSize:      windowSize,
        data:            make([]TokenPair, 0, windowSize),
        minSamples:      minSamples,
        maxRatio:        maxRatio,
        maxTokensPerReq: maxTokensPerReq,
        exceedFactor:    exceedFactor,
    }

    return nil
}

type TokenPair struct {
    Prompt     int
    Completion int
}

type TokenStats struct {
    windowSize      int
    data            []TokenPair
    index           int
    full            bool
    mu              sync.Mutex
    minSamples      int
    maxRatio        float64
    maxTokensPerReq int
    exceedFactor    float64
}

func (s *TokenStats) Add(promptTokens, completionTokens int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if len(s.data) < s.windowSize {
        s.data = append(s.data, TokenPair{Prompt: promptTokens, Completion: completionTokens})
    } else {
        s.data[s.index] = TokenPair{Prompt: promptTokens, Completion: completionTokens}
        s.index = (s.index + 1) % s.windowSize
        s.full = true
    }
}

func (s *TokenStats) IsExceeded(promptTokens int) bool {
    s.mu.Lock()
    defer s.mu.Unlock()

    // code start
    if len(s.data) < s.minSamples {
        return float64(promptTokens)*s.maxRatio > float64(s.maxTokensPerReq)
    }

    var ratios []float64
    var completions []int
    for _, pair := range s.data {
        if pair.Prompt > 0 {
            ratios = append(ratios, float64(pair.Completion)/float64(pair.Prompt))
        }
        completions = append(completions, pair.Completion)
    }
    sort.Float64s(ratios)
    sort.Ints(completions)

    posRatio := int(0.95 * float64(len(ratios)))
    if posRatio >= len(ratios) {
        posRatio = len(ratios) - 1
    }
    expectedCompletion := float64(promptTokens) * ratios[posRatio]

    posCompletion := int(0.95 * float64(len(completions)))
    if posCompletion >= len(completions) {
        posCompletion = len(completions) - 1
    }
    completionP95 := float64(completions[posCompletion])

    return expectedCompletion < completionP95*s.exceedFactor
}
