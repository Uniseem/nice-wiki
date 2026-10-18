package analytics

import (
 "context"
 "time"
 "nice-wiki/libs/platform"
)

// ValidateDateRange 返回左闭右开 UTC 区间；空区间使用最近七天。
func ValidateDateRange(ctx context.Context, op OperationContext, in ValidateDateRangeInput) (ValidateDateRangeResult, error) {
 end:=time.Now().UTC()
 var err error
 if in.To!="" { end,err=time.Parse(time.RFC3339Nano,in.To);if err!=nil {return ValidateDateRangeResult{},platform.NewError(platform.CodeValidation,false,"查询结束时间不合法")} }
 begin:=end.AddDate(0,0,-7)
 if in.From!="" {begin,err=time.Parse(time.RFC3339Nano,in.From);if err!=nil {return ValidateDateRangeResult{},platform.NewError(platform.CodeValidation,false,"查询开始时间不合法")} }
 days:=in.MaxDays;if days==0 {days=366};if days<1||days>366{return ValidateDateRangeResult{},platform.NewError(platform.CodeValidation,false,"查询最大天数必须在1至366之间")}
 if !begin.Before(end)||end.Sub(begin)>time.Duration(days)*24*time.Hour {return ValidateDateRangeResult{},platform.NewError(platform.CodeValidation,false,"查询范围超过允许区间")}
 return ValidateDateRangeResult{FromUTC:begin.UTC().Format(time.RFC3339Nano),ToUTC:end.UTC().Format(time.RFC3339Nano)},nil
}

// pageInterval只选择完整小时或唯一部署报表时区的日历桶，避免把部分桶计数冒充任意精确区间。
func pageInterval(ctx context.Context,op OperationContext,rt *Runtime,in QueryPageStatisticsInput)(time.Time,time.Time,string,error){
 kind:=in.Granularity;if kind==""{kind="hour"};if kind!="hour"&&kind!="day"{return time.Time{},time.Time{},"",platform.NewError(platform.CodeValidation,false,"报表粒度不支持")};location:=time.UTC;if kind=="day"{if rt.CountingLocation==nil{return time.Time{},time.Time{},"",platform.NewError(platform.CodeDependencyUnavailable,true,"可信报表时区尚未配置")};location=rt.CountingLocation};now:=time.Now().In(location);end:=now.Truncate(time.Hour);if kind=="day"{end=time.Date(now.Year(),now.Month(),now.Day(),0,0,0,0,location)};var err error;if in.To!=""{end,err=time.Parse(time.RFC3339Nano,in.To);if err!=nil{return time.Time{},time.Time{},"",platform.NewError(platform.CodeValidation,false,"报表结束时间不合法")}};if kind=="day"{end=end.In(location)};begin:=end.AddDate(0,0,-7);if in.From!=""{begin,err=time.Parse(time.RFC3339Nano,in.From);if err!=nil{return time.Time{},time.Time{},"",platform.NewError(platform.CodeValidation,false,"报表开始时间不合法")}}
 if kind=="hour"{if !begin.Equal(begin.UTC().Truncate(time.Hour))||!end.Equal(end.UTC().Truncate(time.Hour)){return time.Time{},time.Time{},"",platform.NewError(platform.CodeValidation,false,"小时报表范围必须对齐UTC整点")};if _,err=ValidateDateRange(ctx,op,ValidateDateRangeInput{From:begin.Format(time.RFC3339Nano),To:end.Format(time.RFC3339Nano),MaxDays:366});err!=nil{return time.Time{},time.Time{},"",err}}else{begin=begin.In(location);end=end.In(location);if begin.Hour()!=0||begin.Minute()!=0||begin.Second()!=0||begin.Nanosecond()!=0||end.Hour()!=0||end.Minute()!=0||end.Second()!=0||end.Nanosecond()!=0||!begin.Before(end)||end.After(begin.AddDate(0,0,366)){return time.Time{},time.Time{},"",platform.NewError(platform.CodeValidation,false,"日报范围必须对齐报表时区日界且不超过366个日历日")}};return begin.UTC(),end.UTC(),kind,nil
}
