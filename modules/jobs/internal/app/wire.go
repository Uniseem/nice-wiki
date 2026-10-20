// Package app 装配 jobs 域的目录布局、服务身份信任锚、运行权威客户端
// 与 HTTP 容器；不承载任务、事件、死信、定时或屏障领域规则。
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/libs/platform/serviceauth"
	jobs "nice-wiki/modules/jobs/internal"
	"nice-wiki/modules/jobs/internal/gen"
	jobsmigrations "nice-wiki/modules/jobs/migrations"
	"nice-wiki/modules/jobs/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "jobs"

type activationReceipt struct {
	Module          string `json:"module"`
	StageID         string `json:"stageId"`
	TargetDataEpoch string `json:"targetDataEpoch"`
	State           string `json:"state"`
	ManifestHash    string `json:"manifestHash"`
	RecordedAt      string `json:"recordedAt"`
	Signature       string `json:"signature"`
}

type registerActivationRequest struct {
	Receipt         activationReceipt `json:"receipt"`
	ExpectedRevision int64             `json:"expectedRevision"`
	RestoreID       string            `json:"restoreId"`
}

type activationRuntimeState struct {
	DataEpoch        string              `json:"dataEpoch"`
	TargetDataEpoch  *string             `json:"targetDataEpoch"`
	Phase            string              `json:"phase"`
	Maintenance      bool                `json:"maintenance"`
	RestoreID        *string             `json:"restoreId"`
	Revision         int64               `json:"revision"`
	ActivationReceipts []activationReceipt `json:"activationReceipts"`
}

type registerActivationResponse struct {
	State activationRuntimeState `json:"state"`
}

// BuildApplication 验证本域启动前置条件，建立 active/control/restore
// 目录布局，加载服务身份材料，读取运行权威并执行恢复换目录。
//
// 任务、事件、管理、定时、屏障和快照路由均交统一入口；跨域封口proof仍由
// data-protection协调器提供，缺少真实proof时只在seal调用失败关闭。
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
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		return nil, err
	}
	openedDataEpoch := view.DataEpoch
	var swapEvidence lifecycle.SnapshotSwapEvidence
	if swap.Action != platform.SwapNone {
		swapEvidence, err = lifecycle.ReadRestoreSwapState(config.DataRoot, string(swap.RestoreID))
		if err != nil {
			return nil, err
		}
		openedDataEpoch, err = restoreOpenedDataEpoch(view, swap, swapEvidence)
		if err != nil {
			return nil, err
		}
	}

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
	if swap.Action != platform.SwapNone {
		if err := registerActivation(ctx, siteClient, key, view, swap, swapEvidence); err != nil {
			return nil, err
		}
	}
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
	if base := config.Dependencies["data-protection"]; base != "" {
		store.ConfigureDataProtectionClient(gen.NewDataProtectionClient(base, ModuleSlug, key))
	}
	// self publisher按配置可选；缺失时保留outbox，不让Relay任务错误终止整个进程。
	var publisher platform.EventPublisher
	if selfBase := config.Dependencies[ModuleSlug]; selfBase != "" {
		publisher = outbox.NewEventPublisher(gen.NewJobsClient(selfBase, ModuleSlug, key))
	}
	migrations, err := jobsmigrations.All()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "任务迁移集不可读")
	}
	schemaVersion := len(migrations)
	manager, err := lifecycle.NewSnapshotManager(lifecycle.SnapshotConfig{
		Owner: ModuleSlug, DataRoot: config.DataRoot, OpenedDataEpoch: openedDataEpoch,
		Runtime: func(work context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(work, runtimeAuthority)
		},
		ValidateManifest: jobs.ValidateJobsSnapshotManifest,
		BeginDrain: func(work context.Context, _ *lifecycle.SnapshotCleanup) error {
			return jobs.DrainJobs(work, store)
		},
		Drain: func(work context.Context, _ *lifecycle.SnapshotCleanup) error {
			return jobs.DrainJobs(work, store)
		},
		Watermark:   store.SnapshotWatermark,
		ObserveWork: store.SnapshotObserveWork,
		SealWork:    store.SnapshotSealWork,
		ResumeWork:  store.SnapshotResumeWork,
		VerifyGlobalSeal: func(context.Context, lifecycle.SnapshotSealProof, string) error {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "全域封口证明尚未由协调器提供")
		},
		Capture: func(work context.Context, cut lifecycle.SnapshotSafeCut, workspace string, input lifecycle.SnapshotExportInput) (lifecycle.DomainSnapshotManifest, error) {
			if input.IncludeSecrets {
				return lifecycle.DomainSnapshotManifest{}, platform.NewError(platform.CodeUnsupportedCapability, false, "jobs域没有可导出的secret库存")
			}
			db, err := store.SnapshotDatabase()
			if err != nil {
				return lifecycle.DomainSnapshotManifest{}, err
			}
			files, err := lifecycle.CaptureSnapshotFiles(work, filepath.Join(config.DataRoot, "active"), workspace, map[string]*sql.DB{"jobs.db": db}, lifecycle.SnapshotCaptureExclude{})
			if err != nil {
				return lifecycle.DomainSnapshotManifest{}, err
			}
			return lifecycle.DomainSnapshotManifest{Format: "nice-wiki.domain-snapshot", SchemaVersion: 1, Owner: ModuleSlug, BackupID: input.BackupID, BarrierID: cut.BarrierID, SnapshotID: input.SnapshotID, SourceEpoch: cut.Epoch, DataSchemaVersion: schemaVersion, RevisionWatermark: cut.Watermark, ContainsSecrets: false, Files: files, Objects: []lifecycle.DomainSnapshotObject{}, Publications: []lifecycle.SnapshotPublishedRevisionRef{}, References: []lifecycle.DomainSnapshotReference{}, Assets: []platform.AssetVersionRef{}, Invocations: []lifecycle.SnapshotInvocationRef{}}, nil
		},
		MigrateStage:     jobs.MigrateJobsStage,
		ValidateStaged:   jobs.ValidateJobsStaged,
		StageBudget:      jobs.JobsStageBudget,
	})
	if err != nil {
		return nil, err
	}
	if err := store.ConfigureSnapshotManager(manager); err != nil {
		return nil, err
	}
	admitBusiness := manager.AdmitBusiness
	bindEventScope := manager.BindOutboxEventScope
	assembleBackgroundTasks := func(admit platform.WorkAdmission, bindEventScope func(context.Context, platform.OutboxRow) (context.Context, func(), error)) ([]platform.BackgroundTask, error) {
		if store == nil || admit == nil || bindEventScope == nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "jobs后台准入或事件scope尚未完整装配")
		}
		if err := store.ConfigureBackgroundScope(admit, bindEventScope); err != nil {
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
			if view.SafePointRequested {
				if view.BarrierDeadline.IsZero() {
					return time.Time{}, platform.NewError(platform.CodeDependencyUnavailable, true, "后台写窗口缺少control屏障截止")
				}
				return view.BarrierDeadline, nil
			}
			if view.Runtime.Maintenance {
				return time.Time{}, platform.NewError(platform.CodeLocked, true, "维护期间未开放后台写窗口")
			}
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
			periodic("jobs.RequeueExpiredDeliveries", 5000, func(ctx context.Context, op platform.OperationContext, view jobs.LeaseRuntimeView) error {
				_, err := jobs.RequeueExpiredDeliveries(ctx, op, jobs.RequeueExpiredDeliveriesInput{Limit: 100, DataEpoch: view.Runtime.DataEpoch})
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
			{Name: "jobs.RelayOutbox", Run: func(ctx context.Context) error {
				if publisher == nil {
					<-ctx.Done()
					return nil
				}
				view, err := runtimeProvider(ctx)
				if err != nil {
					return err
				}
				bound, op, err := operation(ctx, view.Runtime.DataEpoch)
				if err != nil {
					return err
				}
				return jobs.RelayOutbox(bound, op, publisher)
			}},
		}, nil
	}
	backgroundTasks, err := assembleBackgroundTasks(admitBusiness, bindEventScope)
	if err != nil {
		return nil, err
	}

	handlers := jobs.NewManagementHandlers(key)
	for id, handler := range jobs.NewEventHandlers() {
		handlers[id] = handler
	}
	for id, handler := range jobs.NewLeaseHandlers() {
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
	for id, handler := range jobs.NewBarrierHandlers() {
		handlers[id] = handler
	}
	for id, handler := range jobs.NewSnapshotHandlers() {
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
	barrierCallers := []string{"analytics", "content", "data-protection", "identity", "knowledge", "localization", "media", "memory", "model-gateway", "public-site", "qa", "site-config", "traffic-guard"}
	router, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self:    ModuleSlug,
		Anchors: anchors,
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtimeAuthority)
		},
		AdmitBusiness: admitBusiness,
		ControllerOperations: map[string]bool{
			"coordinateBarrier": true, "acquireBarrier": true, "releaseBarrier": true,
			"exportSnapshot": true, "stageSnapshot": true, "discardSnapshot": true, "readSnapshotArchive": true,
		},
			OpenedDataEpoch: openedDataEpoch,
		AllowedCallers: map[string][]string{
			"createJob": queueCallers,
			"claimJob":  queueCallers, "startJob": queueCallers, "heartbeatJob": queueCallers, "checkLease": queueCallers, "completeJob": queueCallers, "failJob": queueCallers,
			"publishEvent": producerCallers, "acknowledgeEvent": consumerCallers, "subscribeEvents": consumerCallers,
			"listJobs":       {"portal"},
			"cancelJob":      {"portal"},
			"readJob":        readJobCallers,
			"listDeliveries": {"portal"},
			"replayDelivery": {"portal"},
			"retryJob":       {"portal"},
			"streamProgress": {"portal"},
			"createSchedule": createScheduleCallers,
			"listSchedules":  {"portal"},
			"updateSchedule": {"portal"},
			"coordinateBarrier": barrierCallers, "acquireBarrier": {"data-protection"}, "releaseBarrier": {"data-protection"},
			"exportSnapshot": {"data-protection"}, "stageSnapshot": {"data-protection"}, "discardSnapshot": {"data-protection"},
			"readSnapshotArchive": {"data-protection"},
		},
		ServiceActorCallers: map[string][]string{
			"createJob": queueCallers,
			"readJob":   queueCallers,
			"claimJob":  queueCallers, "startJob": queueCallers, "heartbeatJob": queueCallers, "checkLease": queueCallers, "completeJob": queueCallers, "failJob": queueCallers,
			"createSchedule": scheduleOwners,
			"coordinateBarrier": barrierCallers,
		},
		MaintenanceOperations: map[string]bool{"heartbeatJob": true, "checkLease": true, "completeJob": true, "failJob": true, "coordinateBarrier": true, "acquireBarrier": true, "releaseBarrier": true, "exportSnapshot": true, "stageSnapshot": true, "discardSnapshot": true, "readSnapshotArchive": true},
		Authorize:             apivalidate.NewIdentityAuthorizer(identity),
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
		journalRecovered, safePoint, publicRequestsAllowed, err := store.ReadReadinessEvidence(runtimeView, response.Config.Initialized)
		if err != nil {
			return transport.ReadinessEvidence{}, err
		}
		return transport.ReadinessEvidence{
			Runtime: runtimeView, Initialized: response.Config.Initialized,
			JournalRecovered: journalRecovered, SafePoint: safePoint, PublicRequestsAllowed: publicRequestsAllowed,
		}, nil
	}
	if err = application.Install(router, store, backgroundTasks, readiness); err != nil {
		return nil, err
	}
	storeOwnedByApplication = true
	return application, nil
}

// restoreOpenedDataEpoch核对平台完成的换目录事实与当前控制runtime，返回
// 本进程实际打开的active数据代次；不读取control私有文件或放空绕过epoch。
func restoreOpenedDataEpoch(control platform.RuntimeView, swap platform.SwapResult, evidence lifecycle.SnapshotSwapEvidence) (platform.Identifier, error) {
	if swap.Action != platform.SwapSwitched && swap.Action != platform.SwapRolledBack {
		return "", platform.NewError(platform.CodeValidation, false, "恢复换目录动作未知")
	}
	if !platform.ValidIdentifier(swap.RestoreID) || !evidence.Known || evidence.Aborted ||
		!platform.ValidIdentifier(evidence.StageID) || !platform.ValidHash(evidence.ManifestHash) ||
		!platform.ValidHash(evidence.HostManifestHash) || !platform.ValidIdentifier(evidence.HostEpoch) ||
		!platform.ValidIdentifier(evidence.TargetEpoch) || evidence.HostEpoch == evidence.TargetEpoch {
		return "", platform.NewError(platform.CodeIntegrityError, false, "恢复换目录证据不完整")
	}
	if control.Revision < 1 || !control.Maintenance || control.RestoreID != swap.RestoreID ||
		control.DataEpoch != evidence.HostEpoch || control.TargetDataEpoch != evidence.TargetEpoch {
		return "", platform.NewError(platform.CodeVersionMismatch, false, "恢复换目录事实与控制runtime不一致")
	}
	switch swap.Action {
	case platform.SwapSwitched:
		if control.Phase != platform.PhaseRestoreSwitching || !evidence.Switched || evidence.RolledBack {
			return "", platform.NewError(platform.CodeIntegrityError, false, "恢复切换终态与控制runtime不一致")
		}
		return platform.Identifier(evidence.TargetEpoch), nil
	case platform.SwapRolledBack:
		if control.Phase != platform.PhaseRestoreRollingBack || !evidence.RolledBack || evidence.Switched {
			return "", platform.NewError(platform.CodeIntegrityError, false, "恢复换回终态与控制runtime不一致")
		}
		return platform.Identifier(evidence.HostEpoch), nil
	default:
		return "", platform.NewError(platform.CodeValidation, false, "恢复换目录动作未知")
	}
}

// registerActivation登记真实换入/换回回执。先读当前控制状态复用已登记的
// 相同回执，避免重启时续签或覆盖同一恢复决定；请求始终使用service身份与
// 当前控制host epoch，ControllerOperations负责host/target期间的HTTP边界。
func registerActivation(ctx context.Context, client *gen.SiteConfigClient, key platform.SigningKey, control platform.RuntimeView, swap platform.SwapResult, evidence lifecycle.SnapshotSwapEvidence) error {
	if client == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "site-config客户端未装配")
	}
	work, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	deadline, ok := work.Deadline()
	if !ok {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "激活回执缺少操作截止")
	}
	id := platform.NewIdentifier()
	op := platform.OperationContext{
		RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug,
		DataEpoch: control.DataEpoch, Deadline: deadline.UTC().Format(time.RFC3339Nano),
		Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, ExpiresAt: deadline.UTC().Format(time.RFC3339Nano)},
	}
	bound := platform.WithOperationContext(work, op)
	raw, err := client.ReadRuntimeState(bound)
	if err != nil {
		return err
	}
	var current registerActivationResponse
	if err := json.Unmarshal(raw, &current); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "激活回执读取的控制状态不可解码")
	}
	state := current.State
	if state.DataEpoch != control.DataEpoch || state.Phase != control.Phase || state.Maintenance != control.Maintenance || state.Revision < control.Revision || state.RestoreID == nil || *state.RestoreID != string(swap.RestoreID) || state.TargetDataEpoch == nil || *state.TargetDataEpoch != evidence.TargetEpoch {
		return platform.NewError(platform.CodeVersionMismatch, true, "激活回执读取的控制runtime已变化")
	}
	foundState := false
	for _, existing := range state.ActivationReceipts {
		if existing.Module != ModuleSlug || existing.State != swap.Action {
			continue
		}
		foundState = true
		if existing.StageID == evidence.StageID && existing.TargetDataEpoch == evidence.TargetEpoch && existing.ManifestHash == evidence.ManifestHash && existing.RecordedAt != "" && existing.Signature != "" {
			return nil
		}
	}
	if foundState {
		return platform.NewError(platform.CodeStaleResult, false, "同一恢复已有不同jobs激活回执")
	}
	recordedAt := platform.Now()
	claims := serviceauth.ActivationReceiptClaims{
		Version:         1,
		RestoreID:       platform.Identifier(swap.RestoreID),
		Module:          ModuleSlug,
		StageID:         platform.Identifier(evidence.StageID),
		TargetDataEpoch: platform.Identifier(evidence.TargetEpoch),
		State:           swap.Action,
		ManifestHash:    platform.Hash(evidence.ManifestHash),
		RecordedAt:      recordedAt,
	}
	signature, err := serviceauth.SignActivationReceipt(key, claims)
	if err != nil {
		return err
	}
	request := registerActivationRequest{
		Receipt: activationReceipt{
			Module: ModuleSlug, StageID: evidence.StageID, TargetDataEpoch: evidence.TargetEpoch,
			State: swap.Action, ManifestHash: evidence.ManifestHash, RecordedAt: string(recordedAt), Signature: signature,
		},
		ExpectedRevision: state.Revision,
		RestoreID:        string(swap.RestoreID),
	}
	responseRaw, err := client.RegisterActivation(bound, request)
	if err != nil {
		return err
	}
	var response registerActivationResponse
	if err := json.Unmarshal(responseRaw, &response); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "激活回执登记响应不可解码")
	}
	if response.State.DataEpoch != state.DataEpoch || response.State.Phase != state.Phase || response.State.Maintenance != state.Maintenance || response.State.Revision != state.Revision+1 || response.State.RestoreID == nil || *response.State.RestoreID != string(swap.RestoreID) || response.State.TargetDataEpoch == nil || *response.State.TargetDataEpoch != evidence.TargetEpoch {
		return platform.NewError(platform.CodeIntegrityError, false, "激活回执登记响应与控制runtime不一致")
	}
	stored := false
	for _, existing := range response.State.ActivationReceipts {
		if existing.Module != request.Receipt.Module || existing.State != request.Receipt.State {
			continue
		}
		if existing != request.Receipt {
			return platform.NewError(platform.CodeIntegrityError, false, "激活回执登记响应与请求不一致")
		}
		stored = true
		break
	}
	if !stored {
		return platform.NewError(platform.CodeIntegrityError, false, "激活回执登记响应缺少本域回执")
	}
	return nil
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

// ensureDataLayout 建立 active/control/restore 三区；control 区不随
// 业务恢复替换，restore 供 ADR0003 换目录协议使用。
func ensureDataLayout(dataRoot string) error {
	if dataRoot == "" || !filepath.IsAbs(dataRoot) {
		return platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	for _, name := range [...]string{"active", "control", "restore"} {
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
