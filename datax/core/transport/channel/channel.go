// Copyright 2020 the go-etl Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package channel

import (
	"context"
	"sync"

	"github.com/Breeze0806/go-etl/config"
	coreconst "github.com/Breeze0806/go-etl/datax/common/config/core"
	"github.com/Breeze0806/go-etl/element"
	"golang.org/x/time/rate"
)

// Channel 通道
type Channel struct {
	limiter *rate.Limiter
	records *element.RecordChan
	ctx     context.Context
	stats   Stats
}

// Stats Channel的统计信息
type Stats struct {
	sync.RWMutex
	StatsJSON
}

// StatsJSON Channel的JSON统计信息
type StatsJSON struct {
	TotalByte   int64 `json:"totalByte"`
	TotalRecord int64 `json:"totalRecord"`
	Byte        int64 `json:"byte"`
	Record      int64 `json:"record"`
}

func (s *Stats) increase(b int64) {
	s.Lock()
	defer s.Unlock()
	s.TotalByte += b
	s.Byte += b
	s.TotalRecord++
	s.Record++
}

func (s *Stats) reduce(b int64) {
	s.Lock()
	defer s.Unlock()
	s.Byte -= b
	s.Record--
}

// statsJSON 返回json的机构体
func (s *Stats) statsJSON() StatsJSON {
	s.RLock()
	defer s.RUnlock()
	return s.StatsJSON
}

// NewChannel 创建通道
func NewChannel(ctx context.Context, conf *config.JSON) *Channel {
	r := -1
	b := -1.0
	if conf != nil {
		b = conf.GetFloat64OrDefaullt(coreconst.DataxCoreTransportChannelSpeedByte, -1.0)
		r = int(conf.GetInt64OrDefaullt(coreconst.DataxCoreTransportChannelSpeedRecord, -1))
	}
	var limiter *rate.Limiter
	if b > 0 {
		limiter = rate.NewLimiter(rate.Limit(b), int(b))
	}
	if r < 0 {
		r = 0
	}
	return &Channel{
		records: element.NewRecordChanBuffer(ctx, r),
		ctx:     ctx,
		limiter: limiter,
	}
}

// Size 通道记录大小
func (c *Channel) Size() int {
	return c.records.Buffered()
}

// IsEmpty 通道是否为空
func (c *Channel) IsEmpty() bool {
	return c.Size() == 0
}

// Push 将记录r加入通道
func (c *Channel) Push(r element.Record) (n int, err error) {
	if c.limiter != nil {
		if err = c.limiter.WaitN(c.ctx, int(r.ByteSize())); err != nil {
			return 0, err
		}
	}
	c.stats.increase(r.ByteSize())
	return c.records.PushBack(r), nil
}

// Pop 将记录弹出，当通道中不存在记录，就会返回false
func (c *Channel) Pop() (r element.Record, ok bool) {
	r, ok = c.records.PopFront()
	if r != nil {
		c.stats.reduce(r.ByteSize())
	}
	return
}

// PushAll 通过fetchRecord函数加入多条记录
func (c *Channel) PushAll(fetchRecord func() (element.Record, error)) error {
	return c.records.PushBackAll(fetchRecord)
}

// PopAll 通过onRecord函数弹出多条记录
func (c *Channel) PopAll(onRecord func(element.Record) error) error {
	return c.records.PopFrontAll(onRecord)
}

// Close 关闭
func (c *Channel) Close() {
	c.records.Close()
}

// PushTerminate 加入终止记录
func (c *Channel) PushTerminate() (n int) {
	n, _ = c.Push(element.GetTerminateRecord())
	return
}

// StatsJSON 返回Channel的统计信息
func (c *Channel) StatsJSON() StatsJSON {
	return c.stats.statsJSON()
}
