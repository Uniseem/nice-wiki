package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var email = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
var personalPath = regexp.MustCompile(`(?:/Users/|/home/)[A-Za-z0-9._\-]+(?:/|\b)|[A-Za-z]:\\Users\\[^\s\\]+`)
var ip = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b|(?:[A-Fa-f0-9]{1,4}:){2,}[A-Fa-f0-9:]*`)
var key = regexp.MustCompile(`(?:sk-[A-Za-z0-9_\-]{20,}|gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[A-Z0-9]{16}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----)`)
var credentialLiteral = regexp.MustCompile(`(?i)(?:api[_-]?key|access[_-]?token|client[_-]?secret|password)\s*[:=]\s*["\']([A-Za-z0-9_+/=-]{24,})["\']`)
var jwtLiteral = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{12,}\.[A-Za-z0-9_-]{12,}\.[A-Za-z0-9_-]{12,}`)
var identity = regexp.MustCompile(`^[0-9]+\+[A-Za-z0-9-]+@users\.noreply\.github\.com$`)
var conventional = regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([A-Za-z0-9._/-]+\))?!?: .+`)

type scanner struct{ count int }

func git(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s 失败: %w", args[0], err)
	}
	return output, nil
}
func allowedIP(raw string) bool {
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return true
	}
	if addr.IsLoopback() || addr.IsUnspecified() {
		return true
	}
	for _, block := range []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"} {
		if netip.MustParsePrefix(block).Contains(addr) {
			return true
		}
	}
	return false
}
func (s *scanner) report(source string, line int, rule string) {
	s.count++
	// 不回显可能含凭据的命中内容。
	fmt.Fprintf(os.Stderr, "%s:%d: %s\n", source, line, rule)
}
func (s *scanner) scan(source string, data []byte) error {
	if bytes.IndexByte(data, 0) >= 0 {
		return errors.New("二进制文件需人工隐私检查: " + source)
	}
	for index, line := range strings.Split(string(data), "\n") {
		for _, value := range email.FindAllString(line, -1) {
			if !identity.MatchString(value) && value != "noreply@openai.com" && value != "noreply@anthropic.com" && !strings.HasSuffix(value, "@example.com") {
				s.report(source, index+1, "非代用邮箱")
			}
		}
		for _, path := range personalPath.FindAllString(line, -1) {
			// D42 文档里的用户名占位符是规则说明，不能豁免同一行的真实路径。
			if strings.Contains(path, "<") && strings.Contains(path, ">") {
				continue
			}
			s.report(source, index+1, "本机用户路径")
		}
		if key.MatchString(line) || jwtLiteral.MatchString(line) {
			s.report(source, index+1, "疑似密钥")
		}
		for _, match := range credentialLiteral.FindAllStringSubmatch(line, -1) {
			value := strings.ToLower(match[1])
			if !strings.Contains(value, "example") && !strings.Contains(value, "placeholder") {
				s.report(source, index+1, "疑似硬编码凭据")
			}
		}
		for _, value := range ip.FindAllString(line, -1) {
			if !allowedIP(value) {
				s.report(source, index+1, "非文档地址")
			}
		}
	}
	return nil
}
func (s *scanner) files(staged bool) error {
	args := []string{"ls-files", "--cached", "--others", "--exclude-standard", "-z"}
	if staged {
		args = []string{"diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z"}
	}
	output, err := git(args...)
	if err != nil {
		return err
	}
	for _, name := range strings.Split(string(output), "\x00") {
		if name == "" {
			continue
		}
		if name == "vps.txt" || strings.HasPrefix(name, "data/") || name == ".env" || strings.HasPrefix(name, ".env.") {
			s.report(name, 0, "私有配置或运行数据禁止入库")
			continue
		}
		if err := s.scan(name+":filename", []byte(name)); err != nil {
			return err
		}

		var data []byte
		if staged {
			data, err = git("show", ":"+name)
		} else {
			info, statErr := os.Lstat(name)
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			if statErr != nil {
				return statErr
			}
			if !info.Mode().IsRegular() {
				return errors.New("扫描范围包含非普通文件: " + name)
			}
			data, err = os.ReadFile(name)
		}
		if err != nil {
			return err
		}
		if err := s.scan(name, data); err != nil {
			return err
		}
	}
	return nil
}
func (s *scanner) commit(ref string) error {
	data, err := git("show", "-s", "--format=%an%n%ae%n%cn%n%ce%n%B", ref)
	if err != nil {
		return err
	}
	lines := strings.SplitN(string(data), "\n", 5)
	if len(lines) < 5 {
		return errors.New("提交身份输出不完整")
	}
	for _, index := range []int{1, 3} {
		if !identity.MatchString(lines[index]) {
			s.report(ref, 0, "作者或提交者邮箱不是 GitHub 代用地址")
		}
	}
	namePattern := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
	for _, index := range []int{0, 2} {
		if !namePattern.MatchString(lines[index]) {
			s.report(ref, 0, "作者或提交者名称需确认为公开账号")
		}
	}
	if err := s.scan(ref+":message", []byte(lines[4])); err != nil {
		return err
	}
	output, err := git("diff-tree", "--root", "--no-commit-id", "--name-only", "--diff-filter=ACMR", "-m", "-r", "-z", ref)
	if err != nil {
		return err
	}
	for _, name := range strings.Split(string(output), "\x00") {
		if name == "" {
			continue
		}
		if name == "vps.txt" || strings.HasPrefix(name, "data/") || name == ".env" || strings.HasPrefix(name, ".env.") {
			s.report(name, 0, "私有配置或运行数据禁止入库")
			continue
		}
		if err := s.scan(name+":filename", []byte(name)); err != nil {
			return err
		}

		data, err := git("show", ref+":"+name)
		if err != nil {
			return err
		}
		if err := s.scan(ref+":"+name, data); err != nil {
			return err
		}
	}
	return nil
}
func (s *scanner) rangeScan(base, head string) error {
	args := []string{"rev-list", head}
	if strings.Trim(base, "0") != "" {
		args = append(args, "^"+base)
	}
	output, err := git(args...)
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(string(output)) {
		if err := s.commit(ref); err != nil {
			return err
		}
	}
	return nil
}
func (s *scanner) prePush() error {
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		fields := strings.Fields(input.Text())
		if len(fields) != 4 {
			return errors.New("pre-push 输入无效")
		}
		if strings.Trim(fields[1], "0") == "" {
			continue
		}
		if err := s.rangeScan(fields[3], fields[1]); err != nil {
			return err
		}
	}
	return input.Err()
}
func main() {
	mode := flag.String("mode", "working", "working、staged、range、message、pre-push")
	base := flag.String("base", "", "范围起点")
	head := flag.String("head", "HEAD", "范围终点")
	file := flag.String("file", "", "提交信息文件")
	flag.Parse()
	root, err := git("rev-parse", "--show-toplevel")
	if err == nil {
		err = os.Chdir(strings.TrimSpace(string(root)))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	s := &scanner{}
	switch *mode {
	case "working":
		err = s.files(false)
	case "staged":
		err = s.currentIdentities()
		if err == nil {
			err = s.files(true)
		}
	case "range":
		err = s.rangeScan(*base, *head)
	case "pre-push":
		err = s.prePush()
	case "message":
		var data []byte
		data, err = os.ReadFile(*file)
		if err == nil {
			if !conventional.MatchString(strings.SplitN(string(data), "\n", 2)[0]) {
				err = errors.New("提交信息不符合 Conventional Commits")
			} else {
				err = s.scan("commit-message", data)
			}
		}
	default:
		err = errors.New("未知扫描模式")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if s.count != 0 {
		os.Exit(1)
	}
	fmt.Println("隐私扫描通过")
}

// 暂存扫描还核对即将产生的提交身份，不能由助手自动修改 git 配置。
func (s *scanner) currentIdentities() error {
	for _, kind := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"} {
		data, err := git("var", kind)
		if err != nil {
			return err
		}
		raw := string(data)
		start := strings.LastIndex(raw, " <")
		end := strings.Index(raw, ">")
		if start < 0 || end <= start+2 {
			return errors.New("git 身份格式无效")
		}
		address := raw[start+2 : end]
		if !identity.MatchString(address) {
			s.report("git-identity", 0, "提交身份必须使用 GitHub 代用邮箱")
			continue
		}
		plus := strings.Index(address, "+")
		at := strings.Index(address, "@")
		username := address[plus+1 : at]
		name := strings.TrimSpace(raw[:start])
		if !strings.EqualFold(name, username) {
			s.report("git-identity", 0, "提交名称与公开 GitHub 账号不一致，需确认")
		}
	}
	return nil
}
