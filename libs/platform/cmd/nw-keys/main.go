// nw-keys 生成各服务Ed25519密钥对与公钥表的运维工具。
//
// 读取deploy/config/services.json中的服务清单，为每个服务与portal
// 委托各生成一对Ed25519密钥；私钥写入各自secret目录（0600），
// 已存在的密钥不覆盖；汇总公钥写deploy/config/trust-anchors.json。
// --rotate时新增keyId并保留旧公钥一个过渡期。
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// servicesFile deploy/config/services.json中nw-keys使用的子集；
// 其余字段（URL、端口、数据卷、依赖）原样忽略。
type servicesFile struct {
	Version  int            `json:"version"`
	Services []serviceEntry `json:"services"`
}

type serviceEntry struct {
	Slug       string `json:"slug"`
	SecretsDir string `json:"secretsDir"`
}

// trustAnchorsFile 公钥表；不含私钥。
type trustAnchorsFile struct {
	Services             []anchorEntry `json:"services"`
	PortalDelegationKeys []anchorKey   `json:"portalDelegationKeys"`
}

type anchorEntry struct {
	Slug      string `json:"slug"`
	KeyID     string `json:"keyId"`
	PublicKey string `json:"publicKey"`
}

type anchorKey struct {
	KeyID     string `json:"keyId"`
	PublicKey string `json:"publicKey"`
}

// portal委托公钥数量上限，与TrustAnchors约束一致。
const (
	maxDelegationKeys = 4
	keyFilePrefix     = "signing-"
	delegationPrefix  = "delegation-"
	anchorsFileMode   = 0o644
	privateFileMode   = 0o600
	privateDirMode    = 0o700
	failExitCode      = 1
)

func main() {
	servicesPath := flag.String("services", "deploy/config/services.json", "服务清单路径")
	anchorsPath := flag.String("out", "deploy/config/trust-anchors.json", "公钥表输出路径")
	budgetDir := flag.String("budget-dir", "", "匿名预算密钥目录；给出时确保其中有一把预算密钥")
	rotate := flag.Bool("rotate", false, "生成新keyId并保留旧公钥一个过渡期")
	delegationKeys := flag.Int("delegation-keys", 1, "portal委托密钥对数量")
	flag.Parse()
	if err := run(*servicesPath, *anchorsPath, *rotate, *delegationKeys, *budgetDir); err != nil {
		fmt.Fprintf(os.Stderr, "nw-keys: %s\n", err)
		os.Exit(failExitCode)
	}
}

func run(servicesPath, anchorsPath string, rotate bool, delegationCount int, budgetDir string) error {
	if delegationCount < 1 || delegationCount > maxDelegationKeys {
		return usageError("portal委托密钥对数量必须在1到4之间")
	}
	listing, err := readServices(servicesPath)
	if err != nil {
		return err
	}
	anchors := trustAnchorsFile{Services: []anchorEntry{}, PortalDelegationKeys: []anchorKey{}}
	if existing, err := os.ReadFile(anchorsPath); err == nil {
		if err := json.Unmarshal(existing, &anchors); err != nil {
			return usageError("既有公钥表不可解析")
		}
	} else if !os.IsNotExist(err) {
		return usageError("公钥表不可读")
	}
	for _, service := range listing.Services {
		if err := ensureServiceKeys(service, rotate, &anchors); err != nil {
			return err
		}
	}
	if err := ensureDelegationKeys(listing, rotate, delegationCount, &anchors); err != nil {
		return err
	}
	if budgetDir != "" {
		if err := os.MkdirAll(budgetDir, privateDirMode); err != nil {
			return usageError("创建预算密钥目录失败")
		}
		if len(existingKeyIDs(budgetDir, "budget-")) == 0 {
			if _, err := writeKeyPair(filepath.Join(budgetDir, "budget-"+newKeyID("budget")+".pem")); err != nil {
				return err
			}
		}
	}
	sortAnchors(&anchors)
	return writeAnchors(anchorsPath, anchors)
}

// ensureServiceKeys 扫描服务secret目录；已存在signing-*.pem不覆盖，
// 从未生成或--rotate时新增keyId；公钥全部登记，旧公钥保留过渡期。
func ensureServiceKeys(service serviceEntry, rotate bool, anchors *trustAnchorsFile) error {
	if service.Slug == "" || service.SecretsDir == "" {
		return usageError("服务清单条目缺少slug或secretsDir")
	}
	if err := os.MkdirAll(service.SecretsDir, privateDirMode); err != nil {
		return usageError("创建secret目录失败")
	}
	existing := existingKeyIDs(service.SecretsDir, keyFilePrefix)
	if len(existing) == 0 || rotate {
		keyID := newKeyID(service.Slug)
		if _, err := writeKeyPair(filepath.Join(service.SecretsDir, keyFilePrefix+keyID+".pem")); err != nil {
			return err
		}
		existing = append(existing, keyID)
	}
	anchors.Services = dropSlug(anchors.Services, service.Slug)
	for _, keyID := range existing {
		publicKey, err := readPublicKey(filepath.Join(service.SecretsDir, keyFilePrefix+keyID+".pem"))
		if err != nil {
			return err
		}
		anchors.Services = append(anchors.Services, anchorEntry{Slug: service.Slug, KeyID: keyID, PublicKey: publicKey})
	}
	return nil
}

// ensureDelegationKeys 维护portal委托密钥；数量达到配置后不再生成，
// --rotate新增keyId；既有公钥保留一个过渡期，总数不超过上限。
func ensureDelegationKeys(listing servicesFile, rotate bool, delegationCount int, anchors *trustAnchorsFile) error {
	portalDir := ""
	for _, service := range listing.Services {
		if service.Slug == "portal" {
			portalDir = service.SecretsDir
		}
	}
	if portalDir == "" {
		return usageError("服务清单缺少portal条目")
	}
	if err := os.MkdirAll(portalDir, privateDirMode); err != nil {
		return usageError("创建portal secret目录失败")
	}
	existing := existingKeyIDs(portalDir, delegationPrefix)
	if len(existing) < delegationCount || rotate {
		keyID := newKeyID("portal-delegation")
		if _, err := writeKeyPair(filepath.Join(portalDir, delegationPrefix+keyID+".pem")); err != nil {
			return err
		}
		existing = append(existing, keyID)
	}
	registered := map[string]bool{}
	kept := make([]anchorKey, 0, len(anchors.PortalDelegationKeys)+1)
	for _, key := range anchors.PortalDelegationKeys {
		if registered[key.KeyID] {
			continue
		}
		registered[key.KeyID] = true
		kept = append(kept, key)
	}
	for _, keyID := range existing {
		if registered[keyID] {
			continue
		}
		publicKey, err := readPublicKey(filepath.Join(portalDir, delegationPrefix+keyID+".pem"))
		if err != nil {
			return err
		}
		registered[keyID] = true
		kept = append(kept, anchorKey{KeyID: keyID, PublicKey: publicKey})
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].KeyID > kept[j].KeyID })
	if len(kept) > maxDelegationKeys {
		kept = kept[:maxDelegationKeys]
	}
	anchors.PortalDelegationKeys = kept
	return nil
}

// existingKeyIDs 列出目录中既有密钥的keyId；已存在密钥不覆盖。
func existingKeyIDs(dir, prefix string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var ids []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".pem") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".pem"))
	}
	sort.Strings(ids)
	return ids
}

// writeKeyPair 生成Ed25519密钥对，私钥以PKCS8 PEM写入（0600），
// 返回公钥base64url。
func writeKeyPair(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		return "", usageError("密钥文件已存在，拒绝覆盖")
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", usageError("生成密钥对失败")
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", usageError("编码私钥失败")
	}
	raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, raw, privateFileMode); err != nil {
		return "", usageError("写入私钥文件失败")
	}
	return base64.RawURLEncoding.EncodeToString(publicKey), nil
}

// readPublicKey 从私钥PEM导出公钥base64url；已存在密钥不重生成。
func readPublicKey(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", usageError("读取私钥文件失败")
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return "", usageError("私钥文件格式不合法")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", usageError("解析私钥失败")
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return "", usageError("私钥不是Ed25519")
	}
	publicKey, ok := key.Public().(ed25519.PublicKey)
	if !ok {
		return "", usageError("导出公钥失败")
	}
	return base64.RawURLEncoding.EncodeToString(publicKey), nil
}

// newKeyID 生成形如slug-20261014153059-xxx的密钥ID。
func newKeyID(slug string) string {
	random := make([]byte, 2)
	_, _ = rand.Read(random)
	return slug + "-" + time.Now().UTC().Format("20060102150405") + "-" + base64.RawURLEncoding.EncodeToString(random)[:3]
}

// sortAnchors 输出顺序确定，保证公钥表条目可比较。
func sortAnchors(anchors *trustAnchorsFile) {
	sort.Slice(anchors.Services, func(i, j int) bool {
		if anchors.Services[i].Slug != anchors.Services[j].Slug {
			return anchors.Services[i].Slug < anchors.Services[j].Slug
		}
		return anchors.Services[i].KeyID < anchors.Services[j].KeyID
	})
	sort.Slice(anchors.PortalDelegationKeys, func(i, j int) bool {
		return anchors.PortalDelegationKeys[i].KeyID < anchors.PortalDelegationKeys[j].KeyID
	})
}

func writeAnchors(path string, anchors trustAnchorsFile) error {
	raw, err := json.MarshalIndent(anchors, "", "  ")
	if err != nil {
		return usageError("编码公钥表失败")
	}
	if err := os.WriteFile(path, append(raw, '\n'), anchorsFileMode); err != nil {
		return usageError("写入公钥表失败")
	}
	return nil
}

// dropSlug 移除某slug的旧登记；重新登记后按实际文件排序。
func dropSlug(entries []anchorEntry, slug string) []anchorEntry {
	kept := entries[:0]
	for _, entry := range entries {
		if entry.Slug != slug {
			kept = append(kept, entry)
		}
	}
	return kept
}

func readServices(path string) (servicesFile, error) {
	var listing servicesFile
	raw, err := os.ReadFile(path)
	if err != nil {
		return listing, usageError("服务清单不可读")
	}
	if err := json.Unmarshal(raw, &listing); err != nil {
		return listing, usageError("服务清单不合法")
	}
	if len(listing.Services) == 0 {
		return listing, usageError("服务清单为空")
	}
	return listing, nil
}

func usageError(message string) error {
	return fmt.Errorf("VALIDATION: %s", message)
}
