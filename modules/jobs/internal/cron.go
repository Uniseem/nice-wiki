package internal

import (
	"context"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // 把IANA时区数据库打进二进制，不依赖部署机器的区域信息。
	"unicode/utf8"

	"nice-wiki/libs/platform"
)

const (
	cronFieldCount     = 5
	maxCronMinutes     = 527040
	maxCronRunes       = 8192
	dstLookbackMinutes = 24 * 60
)

// ValidateCronInput 是严格定时表达式校验输入。
type ValidateCronInput struct {
	Expression string
	Timezone   string
}

// ValidateCronResult 只返回规范化后的五字段表达式。
type ValidateCronResult struct {
	Normalized string
}

// NextFireAtInput 从给定时刻之后计算下一次触发。
type NextFireAtInput struct {
	Expression string
	Timezone   string
	After      time.Time
}

// NextFireAtResult 是下一次触发的UTC时刻。
type NextFireAtResult struct {
	NextAt time.Time
}

type cronField struct {
	star  bool
	value int
}

type cronSchedule struct {
	minute cronField
	hour   cronField
	day    cronField
	month  cronField
	loc    *time.Location
}

type localMinute struct {
	year   int
	month  time.Month
	day    int
	hour   int
	minute int
}

// ValidateCron 只接受合同写明的五字段子集：分、时、日、月、周。
// 周只接受星号；步长、列表、范围、名称、秒字段和宏一律拒绝，不选用某种cron方言补全。
func ValidateCron(ctx context.Context, op OperationContext, in ValidateCronInput) (ValidateCronResult, error) {
	_, normalized, err := compileCron(in.Expression, in.Timezone)
	if err != nil {
		return ValidateCronResult{}, err
	}
	return ValidateCronResult{Normalized: normalized}, nil
}

// NextFireAt 从after之后的下一UTC分钟起，最多检查366天。
// 按请求时区的本地时间匹配；同一本地分钟的夏令时重复只承认第一次，缺失的本地时间不会被走到。
func NextFireAt(ctx context.Context, op OperationContext, in NextFireAtInput) (NextFireAtResult, error) {
	schedule, _, err := compileCron(in.Expression, in.Timezone)
	if err != nil {
		return NextFireAtResult{}, err
	}
	start := in.After.UTC().Truncate(time.Minute).Add(time.Minute)
	seen := primeLocalMinutes(schedule.loc, start)
	for i := 0; i < maxCronMinutes; i++ {
		candidate := start.Add(time.Duration(i) * time.Minute)
		if !observeLocalMinute(seen, schedule.loc, candidate) {
			continue
		}
		if schedule.match(candidate.In(schedule.loc)) {
			return NextFireAtResult{NextAt: candidate.UTC()}, nil
		}
	}
	return NextFireAtResult{}, platform.NewError(platform.CodeValidation, false, "未来366天内没有下一次触发")
}

func compileCron(expression, timezone string) (cronSchedule, string, error) {
	if expression == "" || !utf8.ValidString(expression) || utf8.RuneCountInString(expression) > maxCronRunes || strings.Contains(expression, "@") {
		return cronSchedule{}, "", invalidCron("定时表达式须为分、时、日、月、周五个字段，不接受秒字段或宏")
	}
	fields := strings.Fields(expression)
	if len(fields) != cronFieldCount {
		return cronSchedule{}, "", invalidCron("定时表达式须为分、时、日、月、周五个字段，不接受秒字段或宏")
	}
	for _, token := range fields {
		if token != "*" && strings.ContainsAny(token, "/,-") {
			return cronSchedule{}, "", invalidCron("步长、列表和范围没有合同句子")
		}
	}
	if fields[4] != "*" {
		return cronSchedule{}, "", invalidCron("周字段编号没有合同句子，只接受星号")
	}
	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return cronSchedule{}, "", err
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return cronSchedule{}, "", err
	}
	day, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return cronSchedule{}, "", err
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return cronSchedule{}, "", err
	}
	gap := 60
	if minute.star {
		gap = 1
	}
	if gap < 1 {
		return cronSchedule{}, "", invalidCron("分钟间隔小于1")
	}
	loc, err := loadIANA(timezone)
	if err != nil {
		return cronSchedule{}, "", err
	}
	normalized := strings.Join([]string{formatCronField(minute), formatCronField(hour), formatCronField(day), formatCronField(month), "*"}, " ")
	return cronSchedule{minute: minute, hour: hour, day: day, month: month, loc: loc}, normalized, nil
}

func parseCronField(token string, min, max int) (cronField, error) {
	if token == "*" {
		return cronField{star: true}, nil
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return cronField{}, invalidCron("定时表达式不接受秒、宏、随机或名称")
		}
	}
	value, err := strconv.Atoi(token)
	if err != nil || value < min || value > max {
		return cronField{}, invalidCron("定时字段超出允许范围")
	}
	return cronField{value: value}, nil
}

func formatCronField(field cronField) string {
	if field.star {
		return "*"
	}
	return strconv.Itoa(field.value)
}

func (schedule cronSchedule) match(local time.Time) bool {
	if !schedule.minute.star && schedule.minute.value != local.Minute() {
		return false
	}
	if !schedule.hour.star && schedule.hour.value != local.Hour() {
		return false
	}
	if !schedule.day.star && schedule.day.value != local.Day() {
		return false
	}
	if !schedule.month.star && schedule.month.value != int(local.Month()) {
		return false
	}
	return true
}

func loadIANA(name string) (*time.Location, error) {
	if name == "" || name == "Local" || name == "local" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > maxCronRunes {
		return nil, invalidTimezone()
	}
	loc, err := time.LoadLocation(name)
	if err != nil || loc == time.Local {
		return nil, invalidTimezone()
	}
	return loc, nil
}

func invalidCron(message string) error {
	return platform.NewError(platform.CodeValidation, false, "%s", message)
}

func invalidTimezone() error {
	return platform.NewError(platform.CodeValidation, false, "时区不是已存在的IANA时区")
}

// primeLocalMinutes 记录起点前24小时出现过的本地分钟。
// 合同只要求DST重复分钟取第一次；折返通常远小于一天，更长的偏移折返没有合同句子。
func primeLocalMinutes(loc *time.Location, start time.Time) map[localMinute]struct{} {
	seen := make(map[localMinute]struct{})
	primeFrom := start.Add(-time.Duration(dstLookbackMinutes) * time.Minute)
	for i := 0; i < dstLookbackMinutes; i++ {
		observeLocalMinute(seen, loc, primeFrom.Add(time.Duration(i)*time.Minute))
	}
	return seen
}

func observeLocalMinute(seen map[localMinute]struct{}, loc *time.Location, utc time.Time) bool {
	local := utc.In(loc)
	key := localMinute{year: local.Year(), month: local.Month(), day: local.Day(), hour: local.Hour(), minute: local.Minute()}
	if _, ok := seen[key]; ok {
		return false
	}
	seen[key] = struct{}{}
	return true
}

// latestCronMatch 在notBefore到now之间取最后一次匹配的UTC分钟，搜索不超过366天。
// 早于窗口、但本身就是下一次已存触发点的时刻，仍可回退为最后到期tick。
func latestCronMatch(expression, timezone string, notBefore, now time.Time) (time.Time, bool, error) {
	schedule, _, err := compileCron(expression, timezone)
	if err != nil {
		return time.Time{}, false, err
	}
	if notBefore.After(now) {
		return time.Time{}, false, nil
	}
	end := now.UTC().Truncate(time.Minute)
	lower := notBefore.UTC()
	start := end.Add(-time.Duration(maxCronMinutes-1) * time.Minute)
	seen := primeLocalMinutes(schedule.loc, start)
	var latest time.Time
	found := false
	for i := 0; i < maxCronMinutes; i++ {
		candidate := start.Add(time.Duration(i) * time.Minute)
		if candidate.After(end) || candidate.After(now) {
			break
		}
		first := observeLocalMinute(seen, schedule.loc, candidate)
		if !first || candidate.Before(lower) {
			continue
		}
		if schedule.match(candidate.In(schedule.loc)) {
			latest = candidate.UTC()
			found = true
		}
	}
	if found {
		return latest, true, nil
	}
	tick := lower.Truncate(time.Minute)
	if tick.Before(start) && !tick.After(now) && minuteIsFirstMatch(schedule, tick) {
		return tick, true, nil
	}
	return time.Time{}, false, nil
}

func minuteIsFirstMatch(schedule cronSchedule, instant time.Time) bool {
	instant = instant.UTC().Truncate(time.Minute)
	seen := primeLocalMinutes(schedule.loc, instant)
	if !observeLocalMinute(seen, schedule.loc, instant) {
		return false
	}
	return schedule.match(instant.In(schedule.loc))
}
