package inbox

import (
	"context"
	"errors"
	"hash/fnv"
	"nice-wiki/libs/platform"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	maxEvents      = 64
	maxInFlightCap = 16
	subscribeRetry = 2 * time.Second
	holdPause      = time.Second
)

// ErrEventSkipped由本域handler在已确认重复或无须应用时返回，仍由平台确认。
var ErrEventSkipped = errors.New("事件已处理或无须应用")

// RunConsumer保留只返回error的调用方式；无需实现无关worker端口。
func RunConsumer(ctx context.Context, client platform.EventConsumerClient, config platform.ConsumerConfig, handlers platform.EventHandlerRegistry) error {
	_, err := RunConsumerWithResult(ctx, client, config, handlers)
	return err
}

// RunConsumerWithResult退出时等待分片结束，再返回实际累计计数。
func RunConsumerWithResult(ctx context.Context, client platform.EventConsumerClient, config platform.ConsumerConfig, handlers platform.EventHandlerRegistry) (result platform.ConsumerRunResult, err error) {
	if client == nil || config.ConsumerID == "" || config.Runtime == nil || config.ValidatePayload == nil || config.AdmitBatch==nil || config.AdmitDelivery==nil || len(config.Events) < 1 || len(config.Events) > maxEvents || config.MaxInFlight < 1 || config.MaxInFlight > maxInFlightCap {
		return result, platform.NewError(platform.CodeValidation, false, "订阅配置或真实运行依赖不完整")
	}
	subscribed := map[string]bool{}
	for _, event := range config.Events {
		parts := strings.Split(event, "@")
		if len(parts) != 2 || parts[0] == "" {
			return result, platform.NewError(platform.CodeValidation, false, "订阅事件必须为名称@版本")
		}
		version, parseErr := strconv.Atoi(parts[1])
		if parseErr != nil || version < 1 || handlers.Handler(parts[0]) == nil || subscribed[event] {
			return result, platform.NewError(platform.CodeValidation, false, "订阅事件版本、handler或唯一性不合法")
		}
		subscribed[event] = true
	}
	c := &consumerLoop{client: client, config: config, subscribed: subscribed, handlers: handlers, queues: make([]chan consumerDelivery, config.MaxInFlight), dones: make([]chan struct{}, config.MaxInFlight)}
	c.start(ctx)
	defer func() {
		c.stop()
		result = platform.ConsumerRunResult{Acknowledged: c.acknowledged.Load(), Skipped: c.skipped.Load(), Failed: c.failed.Load()}
	}()
	lastID := ""
	for {
		if ctx.Err() != nil {
			return result, nil
		}
		view, ok := readRuntime(ctx, config.Runtime)
		if !ok || c.pauseFor(ctx,view) {
			if !sleep(ctx, holdPause) {
				return result, nil
			}
			continue
		}
		call,cancel:=c.operation(ctx,view.DataEpoch,config.ConsumerID)
 batch,releaseBatch,scopeErr:=config.AdmitBatch(call);if scopeErr!=nil{cancel();if !sleep(ctx,holdPause){return result,nil};continue}
 call=batch
		stream, openErr := client.SubscribeEvents(call, config.ConsumerID, lastID)
		if openErr != nil {
			cancel()
 releaseBatch()
			if ctx.Err() != nil {
				return result, nil
			}
			if isConfigRejected(openErr) {
				return result, openErr
			}
			if !sleep(ctx, subscribeRetry) {
				return result, nil
			}
			continue
		}
		held := c.dispatch(ctx, call, stream, &lastID)
		_ = stream.Close()
		cancel()
 releaseBatch()
		if ctx.Err() != nil {
			return result, nil
		}
		if held && !sleep(ctx, holdPause) {
			return result, nil
		}
	}
}

type consumerDelivery struct{event platform.EventEnvelope;ctx context.Context;done func()}

type consumerLoop struct {
	client       platform.EventConsumerClient
	config       platform.ConsumerConfig
	subscribed   map[string]bool
	handlers     platform.EventHandlerRegistry
	queues       []chan consumerDelivery
	dones        []chan struct{}
	acknowledged atomic.Int64
	skipped      atomic.Int64
	failed       atomic.Int64
}

func (c *consumerLoop) start(ctx context.Context) {
	for i := range c.queues {
		c.queues[i] = make(chan consumerDelivery, 1)
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
	for delivery := range c.queues[index] {
 c.process(delivery.ctx,delivery.event)
 delivery.done()
	}
}
func (c *consumerLoop) queueFor(subject string) chan consumerDelivery {
	h := fnv.New32a()
	_, _ = h.Write([]byte(subject))
	return c.queues[h.Sum32()%uint32(len(c.queues))]
}

// operation每次IO建立新截止与当前代次；不转发启动时的委托凭证。
func (c *consumerLoop) operation(ctx context.Context, epoch, root string) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(platform.WithPortalProof(ctx, ""), 30*time.Second)
	deadline, _ := work.Deadline()
	id := platform.NewIdentifier()
	if root == "" {
		root = id
	}
	site := c.config.SiteID
	if site == "" {
		site = "system"
	}
	until := deadline.UTC().Format(time.RFC3339Nano)
	op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: root, CallerService: c.config.ConsumerID, DataEpoch: epoch, Deadline: until, CostClass: platform.CostClassBackground, Actor: platform.AuthContext{Kind: "service", SubjectID: c.config.ConsumerID, SiteID: site, ExpiresAt: until, Grants: []string{}}}
	return platform.WithOperationContext(work, op), cancel
}
func (c *consumerLoop) dispatch(parent context.Context, ctx context.Context, stream platform.EventStream, lastID *string) bool {
	for {
		event, err := stream.Next(ctx)
		if err != nil {
			return false
		}
		*lastID = stream.LastID()
		decision:=c.classify(ctx,event)
 if decision==outcomeHold{return true};if decision==outcomeFailed{c.failed.Add(1);continue}
 work,cancelDelivery:=c.operation(parent,event.DataEpoch,event.EventID)
 scoped,release,scopeErr:=c.config.AdmitDelivery(work)
 if scopeErr!=nil{cancelDelivery();return true}
 delivery:=consumerDelivery{event:event,ctx:scoped,done:func(){cancelDelivery();release()}}
 if decision==outcomeSkipped{c.acknowledge(scoped,event,true);delivery.done();continue}
 select{case c.queueFor(event.SubjectID)<-delivery:case<-ctx.Done():delivery.done();return false}
	}
}
func (c *consumerLoop) acknowledge(ctx context.Context, event platform.EventEnvelope, skipped bool) {
	view, ok := readRuntime(ctx, c.config.Runtime)
	if !ok || c.pauseFor(ctx,view) {
		c.failed.Add(1)
		return
	}
	work, cancel := c.operation(ctx, view.DataEpoch, event.EventID)
	defer cancel()
	if c.client.AcknowledgeEvent(work, c.config.ConsumerID, event.EventID) != nil {
		c.failed.Add(1)
		return
	}
	if skipped {
		c.skipped.Add(1)
	} else {
		c.acknowledged.Add(1)
	}
}
func (c *consumerLoop) process(ctx context.Context, event platform.EventEnvelope) {
	view, ok := readRuntime(ctx, c.config.Runtime)
	if !ok || c.pauseFor(ctx,view) || event.DataEpoch != view.DataEpoch {
		c.failed.Add(1)
		return
	}
	work, cancel := c.operation(ctx, view.DataEpoch, event.EventID)
	err := c.handlers.Handler(event.EventType)(work, platform.EventDelivery{Envelope: event})
	cancel()
	if err != nil && !errors.Is(err, ErrEventSkipped) {
		c.failed.Add(1)
		return
	}
	c.acknowledge(ctx, event, errors.Is(err, ErrEventSkipped))
}

type outcome int

const (
	outcomeProcess outcome = iota
	outcomeSkipped
	outcomeHold
	outcomeFailed
)

func (c *consumerLoop) classify(ctx context.Context, event platform.EventEnvelope) outcome {
	if validateEnvelope(event) != nil || !c.subscribed[eventKey(event.EventType, event.EventVersion)] {
		return outcomeFailed
	}
	view, ok := readRuntime(ctx, c.config.Runtime)
	if !ok || c.pauseFor(ctx,view) {
		return outcomeHold
	}
	if event.DataEpoch!=view.DataEpoch{return outcomeHold}
	if c.config.ValidatePayload(event.EventType, event.EventVersion, event.Payload) != nil {
		return outcomeFailed
	}
	return outcomeProcess
}
func eventKey(name string, version int) string { return name + "@" + strconv.Itoa(version) }
func(c *consumerLoop)pauseFor(ctx context.Context,view platform.RuntimeView)bool{if !view.Maintenance&&view.Phase==platform.PhaseRunning{return false};if c.config.AllowDrain==nil{return true};allowed,err:=c.config.AllowDrain(ctx,view);return err!=nil||!allowed}
func readRuntime(ctx context.Context, provider func(context.Context) (platform.RuntimeView, error)) (platform.RuntimeView, bool) {
	if provider == nil {
		return platform.RuntimeView{}, false
	}
	view, err := provider(ctx)
	return view, err == nil && view.DataEpoch != "" && view.Phase != ""
}
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
func isConfigRejected(err error) bool {
	var problem *platform.Error
	return errors.As(err, &problem) && (problem.Code == platform.CodeValidation || problem.Code == platform.CodeForbidden || problem.Code == platform.CodeNotFound || problem.Code == platform.CodeUnauthenticated)
}
