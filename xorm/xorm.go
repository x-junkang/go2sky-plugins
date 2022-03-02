//
// Copyright 2022 SkyAPM org
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
//

package xorm

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"context"

	"github.com/SkyAPM/go2sky"
	v3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	"xorm.io/xorm"
	"xorm.io/xorm/contexts"
)

const (
	ComponentIDGoXorm int32 = 5010
)

type xormHookSpan struct{}

var xormHookSpanKey = &xormHookSpan{}

type GormHook struct {
	tracer *go2sky.Tracer
	opts   *options
}

func New(tracer *go2sky.Tracer, opts ...Option) *GormHook {
	option := &options{
		dbType:      UNKNOWN,
		componentID: componentIDUnknown,
		peer:        "unknown",
		reportQuery: false,
		reportParam: false,
	}
	for _, opt := range opts {
		opt(option)
	}
	hook := &GormHook{
		tracer: tracer,
		opts:   option,
	}
	return hook
}

// func WrapEngine(e *xorm.Engine, dataSourceName string) {
// 	e.AddHook(NewGormHook(ketrace_api.GetTracer(), dataSourceName))
// }

// func WrapEngineWithTracer(e *xorm.Engine, tracer *ketrace.Tracer, dataSourceName string) {
// 	e.AddHook(NewGormHook(tracer, dataSourceName))
// }

func (h *GormHook) Initialize(e *xorm.Engine) error {
	if h == nil {
		return errors.New("hook is nil")
	}
	e.AddHook(h)
	return nil
}

func (h *GormHook) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	peer := h.opts.peer
	if h.tracer == nil {
		return context.Background(), nil
	}
	span, err := h.tracer.CreateExitSpan(c.Ctx, c.SQL, peer, func(header, value string) error {
		// https://github.com/apache/skywalking/blob/master/docs/en/protocols/Skywalking-Cross-Process-Propagation-Headers-Protocol-v3.md
		return nil
	})
	// create span fail, use original ctx
	if err != nil {
		return c.Ctx, nil
	}

	span.SetComponent(h.opts.componentID)

	if h.opts.reportQuery {
		span.Tag("sql", c.SQL)
	}
	if h.opts.reportParam {
		span.Tag("args", argsToString(c.Args))
	}
	span.SetSpanLayer(v3.SpanLayer_Database)
	ctx := context.WithValue(c.Ctx, xormHookSpanKey, span)
	return ctx, nil
}

func (h *GormHook) AfterProcess(c *contexts.ContextHook) error {
	span, ok := c.Ctx.Value(xormHookSpanKey).(go2sky.Span)
	if !ok {
		return nil
	}
	if c.ExecuteTime > 0 {
		span.Tag("execute_time_ms", c.ExecuteTime.String())
	}
	if c.Err != nil {
		timeNow := time.Now()
		span.Error(timeNow, c.Err.Error())
	}
	span.End()
	return nil
}

func argsToString(args []interface{}) string {
	sb := strings.Builder{}

	switch len(args) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%v", args[0])
	}

	sb.WriteString(fmt.Sprintf("%v", args[0]))
	for _, arg := range args[1:] {
		sb.WriteString(fmt.Sprintf(", %v", arg))
	}
	return sb.String()
}
