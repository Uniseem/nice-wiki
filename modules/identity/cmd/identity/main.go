// identity 服务可执行入口：只装配本域参数，进程模板、信号与退出码
// 语义由 libs/platform/lifecycle.RunService 统一实现。
//
// 合同中的 `accounts` 运维子命令依赖 identity.ReadServerAccountIndex
// （server_recovery.go，后续业务工作包），随该函数一并接线。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	identity "nice-wiki/modules/identity/internal"
	"nice-wiki/modules/identity/internal/app"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "accounts" {
		os.Exit(runAccounts(os.Args[2:]))
	}
	os.Exit(lifecycle.RunService(platform.ServiceSpec{
		Module: "identity",
		BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
			return app.BuildApplication(ctx, config)
		},
		RuntimeClient: app.NewRuntimeClient,
	}))
}

// runAccounts 执行本机账号查询，不启动HTTP服务；配置、运行代次和
// 服务签名依赖仍按正常服务边界读取，避免在未知运行状态下放行查询。
func runAccounts(args []string) int {
	config, err := lifecycle.LoadServiceConfig(args, os.Environ())
	if err != nil {
		return reportAccountsError(err)
	}
	client, err := app.NewRuntimeClient(config)
	if err != nil {
		return reportAccountsError(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	view, err := lifecycle.ReadRuntime(ctx, client)
	if err != nil {
		return reportAccountsError(err)
	}
	ctx = identity.WithServerAccountIndexContext(ctx, config.DataRoot, view.DataEpoch)
	deadline := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)
	result, err := identity.ReadServerAccountIndex(ctx, platform.OperationContext{
		CallerService: "identity",
		Deadline:      deadline,
		DataEpoch:     view.DataEpoch,
	}, identity.ReadServerAccountIndexInput{})
	if err != nil {
		return reportAccountsError(err)
	}
	for _, account := range result.Index.Accounts {
		fmt.Printf("%s\t%s\n", account.AccountName, account.Status)
	}
	return 0
}

func reportAccountsError(err error) int {
	code := platform.CodeInternal
	var platformErr *platform.Error
	if errors.As(err, &platformErr) {
		code = platformErr.Code
	}
	fmt.Fprintf(os.Stderr, "accounts失败，errorCode=%s\n", code)
	return 1
}
