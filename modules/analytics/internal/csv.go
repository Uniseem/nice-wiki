package analytics

import (
 "context"
 "strings"
)

// ValidateCSVCell 对危险首字符添加单引号，再按 CSV 规则转义。
func ValidateCSVCell(ctx context.Context, op OperationContext, in ValidateCSVCellInput) (ValidateCSVCellResult,error) {
 value:=in.Value
 if len(value)>0&&strings.ContainsRune("=+@-\t\r\n",rune(value[0])) {value="'"+value}
 if strings.ContainsAny(value,",\r\n\"") {value="\""+strings.ReplaceAll(value,"\"","\"\"")+"\""}
 return ValidateCSVCellResult{Escaped:value},nil
}
