package inbox

import (
	"context"
	"errors"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

// 订阅循环参数边界与重连退避。
const (
	maxEvents      = 64
	maxInFlightCap = 16
	maxSubscribers = 16
	subscribeRetry = 2 * time.Second
	holdPause      = time.Second
)

// RunConsumer 是事件订阅循环的通用实现：
// 以durable模式订阅jobs事件SSE并用Last-Event-ID续传；校验信封、
// payloadSchema与dataEpoch——旧代事件确认跳过，未换入的新代事件
// 不确认；按事件名调用登记处理函数，成功后确认，失败不确认等待重投；
// 同subject串行，不同subject并发≤maxInFlight；屏障draining时停止领取。
func RunConsumer(ctx context.Context, client platform.JobsClient, config platform.ConsumerConfig, handlers platform.EventHandlerRegistry) error {
	if config.ConsumerID == "" {
		return platform.NewError(platform.CodeValidation, false, "consumerId缺失")
	}
	if len(config.Events) > maxEvents || config.MaxInFlight < 1 || config.MaxInFlight > maxInFlightCap {
		return platform.NewError(platform.CodeValidation, false, "订阅配置超出允许范围")
	}
	subscribed := map[string]bool{}
	for _, event := range config.Events {
		if event == "" || strings.Count(event, "@") != 1 {
			return platform.NewError(platform.CodeValidation, false, "订阅事件必须为 名称@版本")
		}
		subscribed[event] = true
	}
	consumer := &consumerLoop{
		client:     client,
		config:     config,
		subscribed: subscribed,
		handlers:   handlers,
		queues:     make([]chan platform.EventEnvelope, config.MaxInFlight),
		dones:      make([]chan struct{}, config.MaxInFlight),
	}
	consumer.start(ctx)
	defer consumer.stop()
	var lastEventID string
	for {
		if ctx.Err() != nil {
			return nil
		}
		if view, ok := readRuntime(ctx, config.Runtime); ok && pauseFor(view) {
			if !sleep(ctx, holdPause) {
				return nil
			}
			continue
		}
		stream, err := client.SubscribeEvents(ctx, config.ConsumerID, lastEventID)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if isConfigRejected(err) {
				return err
			}
			if !sleep(ctx, subscribeRetry) {
				return nil
			}
			continue
		}
		held := consumer.dispatch(ctx, stream, &lastEventID)
		_ = stream.Close()
		if ctx.Err() != nil {
			return nil
		}
		if held {
			// 未换入的新代事件不确认：暂停后由jobs按原游标重投。
			if !sleep(ctx, holdPause) {
				return nil
			}
		}
	}
}

type consumerLoop struct {
	client     platform.JobsClient
	config     platform.ConsumerConfig
	subscribed map[string]bool
	handlers   platform.EventHandlerRegistry
	queues     []chan platform.EventEnvelope
	dones      []chan struct{}
}

// start 启动maxInFlight个工作协程；按subject哈希固定分配，
// 保证同一subject始终串行。
func (c *consumerLoop) start(ctx context.Context) {
	for i := range c.queues {
		c.queues[i] = make(chan platform.EventEnvelope, 1)
		c.dones[i] = make(chan struct{})
		go c.worker(ctx, i)
	}
}

func (c *consumerLoop) stop() {
	for i := range c.queues {
		close(c.queues[i])
		<-c.dones[i]
	}
}

func (c *consumerLoop) worker(ctx context.Context, index int) {
	defer close(c.dones[index])
	for envelope := range c.queues[index] {
		c.process(ctx, envelope)
	}
}

func (c *consumerLoop) queueFor(subject platform.Identifier) chan platform.EventEnvelope {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(subject))
	return c.queues[hasher.Sum32()%uint32(len(c.queues))]
}

// dispatch 分发流内事件直到流结束、取消或遇到未换入的新代事件；
// 返回是否因新代事件暂停。确认后的游标写入lastEventID。
func (c *consumerLoop) dispatch(ctx context.Context, stream platform.EventStream, lastEventID *string) bool {
	for {
		envelope, err := stream.Next(ctx)
		if err != nil {
			return false
		}
		*lastEventID = stream.LastID()
		outcome := c.classify(ctx, envelope)
		switch outcome {
		case outcomeHold:
			return true
		case outcomeSkipped:
			if ackErr := c.client.AcknowledgeEvent(ctx, c.config.ConsumerID, envelope.EventID); ackErr != nil && ctx.Err() != nil {
				return false
			}
		case outcomeProcess:
			queue := c.queueFor(envelope.SubjectID)
			select {
			case queue <- envelope:
			case <-ctx.Done():
				return false
			}
		}
	}
}

// process 由工作协程调用：调用登记处理函数，成功后确认。
// 失败不确认，等待jobs在确认租约到期后重投。
func (c *consumerLoop) process(ctx context.Context, envelope platform.EventEnvelope) {
	handler := c.handlers.Handler(envelope.EventType)
	if handler == nil {
		_ = c.client.AcknowledgeEvent(ctx, c.config.ConsumerID, envelope.EventID)
		return
	}
	if err := handler(ctx, platform.EventDelivery{Envelope: envelope}); err != nil {
		return
	}
	_ = c.client.AcknowledgeEvent(ctx, c.config.ConsumerID, envelope.EventID)
}

type outcome int

const (
	outcomeProcess outcome = iota
	outcomeSkipped
	outcomeHold
)

// classify 校验信封、订阅范围、payloadSchema与dataEpoch。
func (c *consumerLoop) classify(ctx context.Context, envelope platform.EventEnvelope) outcome {
	if validateEnvelope(envelope) != nil {
		return outcomeSkipped
	}
	if !c.subscribed[eventKey(envelope.EventType, envelope.EventVersion)] {
		return outcomeSkipped
	}
	view, ok := readRuntime(ctx, c.config.Runtime)
	if ok && view.DataEpoch != "" {
		switch envelope.DataEpoch {
		case view.DataEpoch:
			// 当前代事件照常处理。
		case view.TargetDataEpoch:
			// 未换入的新代事件不确认，等待本服务换入。
			return outcomeHold
		default:
			// 旧代事件确认跳过。
			return outcomeSkipped
		}
	}
	if c.config.ValidatePayload != nil {
		if err := c.config.ValidatePayload(envelope.EventType, envelope.EventVersion, envelope.Payload); err != nil {
			return outcomeSkipped
		}
	}
	return outcomeProcess
}

func eventKey(eventType string, version int) string {
	return eventType + "@" + strconv.Itoa(version)
}

// pauseFor 屏障draining与恢复切换期间暂停领取。
func pauseFor(view platform.RuntimeView) bool {
	switch view.Phase {
	case platform.PhaseBackupBarrier, platform.PhaseRestoreSwitching, platform.PhaseRestoreRollingBack, platform.PhaseRestoreBlocked:
		return true
	}
	return false
}

// readRuntime 读取运行状态；未提供或失败按running处理。
func readRuntime(ctx context.Context, provider func(context.Context) (platform.RuntimeView, error)) (platform.RuntimeView, bool) {
	if provider == nil {
		return platform.RuntimeView{Phase: platform.PhaseRunning}, false
	}
	view, err := provider(ctx)
	if err != nil {
		return platform.RuntimeView{Phase: platform.PhaseRunning}, false
	}
	return view, true
}

// sleep 等待d或ctx取消；返回false表示已取消。
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// isConfigRejected 订阅未登记等永久性拒绝直接上抛，不无限重试。
func isConfigRejected(err error) bool {
	var libErr *platform.Error
	if errors.As(err, &libErr) {
		return libErr.Code == platform.CodeValidation || libErr.Code == platform.CodeForbidden || libErr.Code == platform.CodeNotFound
	}
	return false
}
