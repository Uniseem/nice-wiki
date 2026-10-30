// Package app 装配 jobs 域的目录布局、服务身份信任锚、运行权威客户端
// 与 HTTP 容器；不承载任务、事件、死信、定时或屏障领域规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/queue"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/libs/platform/sitestate"
	jobs "nice-wiki/modules/jobs/internal"
	"nice-wiki/modules/jobs/internal/gen"
	"nice-wiki/modules/jobs/internal/transport"
	jobsmigrations "nice-wiki/modules/jobs/migrations"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "jobs"

// BuildApplication 验证本域启动前置条件，建立 active/control
// 目录布局，加载服务身份材料，读取运行权威。
//
// 任务、事件、管理和定时路由均交统一入口。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	if err := ensureDataLayout(config.DataRoot); err != nil {
		return nil, err
	}
	anchors, err := loadTrustAnchors(config.TrustAnchorRef)
	if err != nil {
		return nil, err
	}
	key, err := loadSigningKey(config.SigningKeyRef)
	if err != nil {
		return nil, err
	}
	if anchors.ServiceKeyID(ModuleSlug, key.KeyID) == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域服务身份未登记在信任锚中")
	}
	runtimeAuthority, err := newRuntimeClient(config, key)
	if err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtimeAuthority)
	if err != nil {
		return nil, err
	}
	openedDataEpoch := view.DataEpoch

	// 实际建立已完成管理映射，并由统一工厂核完整规格；缺其他handler
	// 仍拒绝启动，不能把已完成切片裸挂成可用服务。
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域内嵌接口规格不可读")
	}
	identityBase := config.Dependencies["identity"]
	if identityBase == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "任务管理缺少identity地址")
	}
	identity := gen.NewIdentityClient(identityBase, ModuleSlug, key)
	store, err := jobs.OpenDatabase(config.DataRoot)
	if err != nil {
		return nil, err
	}
	storeOwnedByApplication := false
	defer func() {
		if !storeOwnedByApplication {
			_ = store.Close()
		}
	}()
	siteValidator, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "QueueSiteConfigResult")
	if err != nil {
		return nil, err
	}
	siteClient := gen.NewSiteConfigClient(config.Dependencies["site-config"], ModuleSlug, key)
	siteID := func(ctx context.Context, epoch platform.Identifier) (string, error) {
		return ReadQueueSiteID(ctx, siteClient, string(epoch), siteValidator)
	}
	recovery, err := NewOwnerRecoveryChecker(config, key, siteID)
	if err != nil {
		return nil, err
	}
	runtimeProvider := func(work context.Context) (jobs.LeaseRuntimeView, error) {
		current, err := lifecycle.ReadRuntime(work, runtimeAuthority)
		if err != nil {
			return jobs.LeaseRuntimeView{}, err
		}
		return store.ReadLeaseRuntime(current)
	}
	jobs.InstallStore(store, runtimeProvider, identity, recovery, siteID)
	events, err := queue.ForConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	jobs.InstallEventQueue(events)
	migrations, err := jobsmigrations.All()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "任务迁移集不可读")
	}
	if len(migrations) == 0 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "任务迁移集为空")
	}
	admitBusiness := lifecycle.SimpleAdmission(runtimeAuthority.ReadRuntimeState, openedDataEpoch)
	assembleBackgroundTasks := func(admit platform.WorkAdmission) ([]platform.BackgroundTask, error) {
		if store == nil || admit == nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "jobs后台准入尚未完整装配")
		}
		if err := store.ConfigureBackgroundScope(admit); err != nil {
			return nil, err
		}
		runtimeView := func(work context.Context) (platform.RuntimeView, error) {
			view, err := runtimeProvider(work)
			return view.Runtime, err
		}
		writeWindow := func(work context.Context) (time.Time, error) {
			view, err := runtimeProvider(work)
			if err != nil {
				return time.Time{}, err
			}
			_ = view
			return time.Time{}, nil
		}
		operation := func(work context.Context, epoch platform.Identifier) (context.Context, platform.OperationContext, error) {
			if !platform.ValidIdentifier(epoch) {
				return nil, platform.OperationContext{}, platform.NewError(platform.CodeVersionMismatch, false, "后台任务数据代次不可用")
			}
			deadline, ok := work.Deadline()
			if !ok {
				deadline = time.Now().UTC().Add(30 * time.Second)
			}
			id := platform.NewIdentifier()
			expires := deadline.UTC().Format(time.RFC3339Nano)
			op := platform.OperationContext{
				RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug,
				DataEpoch: epoch, Deadline: expires, CostClass: platform.CostClassBackground,
				Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system", ExpiresAt: expires, Grants: []string{}},
			}
			return platform.WithOperationContext(work, op), op, nil
		}
		periodic := func(name string, pollMillis int, run func(context.Context, platform.OperationContext, jobs.LeaseRuntimeView) error) platform.BackgroundTask {
			return platform.BackgroundTask{Name: name, Run: func(ctx context.Context) error {
				return lifecycle.RunRecoveryLoop(ctx, platform.RecoveryLoopConfig{
					AdmitBatch: admit, PollMillis: pollMillis, Runtime: runtimeView, WriteWindow: writeWindow,
					Recover: func(work context.Context) error {
						view, err := runtimeProvider(work)
						if err != nil {
							return err
						}
						bound, op, err := operation(work, view.Runtime.DataEpoch)
						if err != nil {
							return err
						}
						return run(bound, op, view)
					},
				})
			}}
		}
		purgeNext := time.Time{}
		return []platform.BackgroundTask{
			periodic("jobs.RequeueDue", 1000, func(ctx context.Context, op platform.OperationContext, view jobs.LeaseRuntimeView) error {
				_, err := jobs.RequeueDue(ctx, op, jobs.RequeueDueInput{Now: time.Now().UTC(), BatchSize: 100, DataEpoch: view.Runtime.DataEpoch})
				return err
			}),
			periodic("jobs.ReapExpiredLeases", 5000, func(ctx context.Context, op platform.OperationContext, view jobs.LeaseRuntimeView) error {
				_, err := jobs.ReapExpiredLeases(ctx, op, jobs.ReapExpiredLeasesInput{Now: time.Now().UTC(), DataEpoch: view.Runtime.DataEpoch})
				return err
			}),
			periodic("jobs.TickSchedules", 1000, func(ctx context.Context, op platform.OperationContext, view jobs.LeaseRuntimeView) error {
				_, err := jobs.TickSchedules(ctx, op, jobs.TickSchedulesInput{Now: time.Now().UTC(), DataEpoch: view.Runtime.DataEpoch})
				return err
			}),
			periodic("jobs.PurgeExpiredHistory", 60000, func(ctx context.Context, op platform.OperationContext, view jobs.LeaseRuntimeView) error {
				now := time.Now().UTC()
				if !purgeNext.IsZero() && now.Before(purgeNext) {
					return nil
				}
				if _, err := jobs.PurgeExpiredHistory(ctx, op, jobs.PurgeExpiredHistoryInput{Now: now, BatchSize: 500}); err != nil {
					return err
				}
				purgeNext = now.Add(24 * time.Hour)
				return nil
			}),
		}, nil
	}
	backgroundTasks, err := assembleBackgroundTasks(admitBusiness)
	if err != nil {
		return nil, err
	}
	// 站点删除到期后清掉它的任务记录。
	backgroundTasks = append(backgroundTasks, sitestate.Task(sitestate.Config{Module: ModuleSlug, DB: store.DB(), Events: events.Consumer(), Runtime: runtimeAuthority.ReadRuntimeState, Purge: store.PurgeSiteJobs}))

	handlers := jobs.NewManagementHandlers(key)
	for id, handler := range jobs.NewEventHandlers() {
		handlers[id] = handler
	}
	for id, handler := range jobs.NewQueueHandlers() {
		handlers[id] = handler
	}
	for id, handler := range jobs.NewProgressHandlers() {
		handlers[id] = handler
	}
	for id, handler := range jobs.NewScheduleHandlers() {
		handlers[id] = handler
	}
	application := transport.NewApplication(config, anchors, runtimeAuthority, openedDataEpoch)
	for id, handler := range application.ControlHandlers() {
		handlers[id] = handler
	}
	queueCallers := make([]string, 0)
	queueOwners := make(map[string]bool)
	scheduleOwners := make([]string, 0)
	seenScheduleOwner := map[string]bool{}
	for _, definition := range gen.JobTypes {
		if !queueOwners[definition.Owner] {
			queueOwners[definition.Owner] = true
			queueCallers = append(queueCallers, definition.Owner)
		}
		if definition.Schedule != "" && !seenScheduleOwner[definition.Owner] {
			seenScheduleOwner[definition.Owner] = true
			scheduleOwners = append(scheduleOwners, definition.Owner)
		}
	}
	producerCallers := make([]string, 0)
	consumerCallers := make([]string, 0)
	seenProducer := map[string]bool{}
	seenConsumer := map[string]bool{}
	for _, event := range gen.EventTypes {
		if !seenProducer[event.Producer] {
			seenProducer[event.Producer] = true
			producerCallers = append(producerCallers, event.Producer)
		}
		for _, consumer := range event.Consumers {
			if !seenConsumer[consumer] {
				seenConsumer[consumer] = true
				consumerCallers = append(consumerCallers, consumer)
			}
		}
	}
	sort.Strings(queueCallers)
	sort.Strings(scheduleOwners)
	sort.Strings(producerCallers)
	sort.Strings(consumerCallers)
	readJobCallers := append([]string{"portal"}, queueCallers...)
	createScheduleCallers := append([]string{"portal"}, scheduleOwners...)
	router, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self:    ModuleSlug,
		Anchors: anchors,
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtimeAuthority)
		},
		AdmitBusiness:   admitBusiness,
		OpenedDataEpoch: openedDataEpoch,
		AllowedCallers: map[string][]string{
			"createJob":    queueCallers,
			"publishEvent": producerCallers, "acknowledgeEvent": consumerCallers, "subscribeEvents": consumerCallers,
			"listJobs":       {"portal"},
			"cancelJob":      {"portal", "memory", "qa"},
			"readJob":        readJobCallers,
			"listDeliveries": {"portal"},
			"replayDelivery": {"portal"},
			"retryJob":       {"portal"},
			"streamProgress": {"portal"},
			"createSchedule": createScheduleCallers,
			"listSchedules":  {"portal"},
			"updateSchedule": {"portal"},
		},
		ServiceActorCallers: map[string][]string{
			"cancelJob":      {"memory", "qa"},
			"createJob":      queueCallers,
			"readJob":        queueCallers,
			"createSchedule": scheduleOwners,
		},
		Authorize: apivalidate.NewIdentityAuthorizer(identity),
	}, handlers)
	if err != nil {
		return nil, err
	}
	readiness := func(work context.Context) (transport.ReadinessEvidence, error) {
		runtimeView, err := lifecycle.ReadRuntime(work, runtimeAuthority)
		if err != nil {
			return transport.ReadinessEvidence{}, err
		}
		if runtimeView.DataEpoch != openedDataEpoch {
			return transport.ReadinessEvidence{}, platform.NewError(platform.CodeVersionMismatch, true, "就绪读取发现数据代次变化")
		}
		deadline := time.Now().UTC().Add(1500 * time.Millisecond)
		if current, ok := work.Deadline(); ok && current.Before(deadline) {
			deadline = current
		}
		id := platform.NewIdentifier()
		op := platform.OperationContext{
			RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug,
			DataEpoch: runtimeView.DataEpoch, Deadline: deadline.UTC().Format(time.RFC3339Nano),
			Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system", ExpiresAt: deadline.UTC().Format(time.RFC3339Nano)},
		}
		raw, err := siteClient.ReadSiteConfig(platform.WithOperationContext(work, op))
		if err != nil {
			return transport.ReadinessEvidence{}, err
		}
		if err := siteValidator(raw); err != nil {
			return transport.ReadinessEvidence{}, platform.NewError(platform.CodeIntegrityError, false, "就绪读取的站点配置不符合合同")
		}
		var response struct {
			Config struct {
				Initialized bool `json:"initialized"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return transport.ReadinessEvidence{}, platform.NewError(platform.CodeIntegrityError, false, "就绪读取的站点配置不可解码")
		}
		journalRecovered, publicRequestsAllowed, err := store.ReadReadinessEvidence(response.Config.Initialized)
		if err != nil {
			return transport.ReadinessEvidence{}, err
		}
		return transport.ReadinessEvidence{
			Runtime: runtimeView, Initialized: response.Config.Initialized,
			JournalRecovered: journalRecovered, PublicRequestsAllowed: publicRequestsAllowed,
		}, nil
	}
	if err = application.Install(router, store, backgroundTasks, readiness); err != nil {
		return nil, err
	}
	storeOwnedByApplication = true
	return application, nil
}

// NewRuntimeClient 构造 site-config 运行权威客户端，供进程模板的运行
// 状态监视使用；不在构造阶段发起网络请求。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	key, err := loadSigningKey(config.SigningKeyRef)
	if err != nil {
		return nil, err
	}
	return newRuntimeClient(config, key)
}

func newRuntimeClient(config platform.ServiceConfig, key platform.SigningKey) (platform.SiteConfigClient, error) {
	baseURL := config.Dependencies["site-config"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

// ensureDataLayout 建立 active/control 两区。
func ensureDataLayout(dataRoot string) error {
	if dataRoot == "" || !filepath.IsAbs(dataRoot) {
		return platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	for _, name := range [...]string{"active", "control"} {
		if err := os.MkdirAll(filepath.Join(dataRoot, name), 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
	}
	return nil
}

// loadTrustAnchors 读取 nw-keys 生成的公钥表；不可用时按合同返回
// DEPENDENCY_UNAVAILABLE，启动失败交由进程守护重试。
func loadTrustAnchors(ref string) (platform.TrustAnchors, error) {
	raw, err := os.ReadFile(ref)
	if err != nil {
		return platform.TrustAnchors{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务身份信任锚不可读")
	}
	var anchors platform.TrustAnchors
	if err := json.Unmarshal(raw, &anchors); err != nil || len(anchors.Services) == 0 {
		return platform.TrustAnchors{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务身份信任锚不合法")
	}
	return anchors, nil
}

// loadSigningKey 读取密钥引用文件 {keyId, privateKeyRef} 并验证私钥可解析；
// 私钥字节只在平台鉴权缓存内，不写入日志或配置。
func loadSigningKey(ref string) (platform.SigningKey, error) {
	raw, err := os.ReadFile(ref)
	if err != nil {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不可读")
	}
	var key platform.SigningKey
	if err := json.Unmarshal(raw, &key); err != nil || key.KeyID == "" || !filepath.IsAbs(key.PrivateKeyRef) {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不合法")
	}
	if _, err := serviceauth.LoadSigningKey(key); err != nil {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名私钥不可用")
	}
	return key, nil
}
