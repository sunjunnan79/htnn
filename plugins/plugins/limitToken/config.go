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
	limiter *redis_rate.Limiter
}

func (conf *config) Init(cb api.ConfigCallbackHandler) error {
	//初始化redis客户端
	conf.initRedisLimiter()

	return nil
}

func (conf *config) initRedisLimiter() {

	rdb := redis.NewClient(&redis.Options{
		Addr:     conf.Redis.ServiceAddr,
		Username: conf.Redis.Username,
		Password: conf.Redis.Password,
	})

	conf.limiter = redis_rate.NewLimiter(rdb)

}
