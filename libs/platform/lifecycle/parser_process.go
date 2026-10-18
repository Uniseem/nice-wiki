package lifecycle

import (
 "bytes"
 "context"
 "errors"
 "io"
 "os"
 "os/exec"
 "strings"
 "time"

 "nice-wiki/libs/platform"
)

const parserArgument = "--nw-isolated-parser="
const parserInputLimit = 50 << 20
const parserOutputLimit = 16 << 20

// RunBoundedParser 只启动当前二进制固定子入口，输入输出均为管道，不继承服务配置与密钥环境。
func RunBoundedParser(ctx context.Context, kind string, input []byte) ([]byte, error) {
 if !platform.ValidIdentifier(kind) || len(input)>parserInputLimit { return nil,platform.NewError(platform.CodeValidation,false,"隔离解析请求超出限制") }
 executable,err:=os.Executable()
 if err!=nil { return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"无法定位隔离解析入口") }
 childCtx,cancel:=context.WithTimeout(ctx,60*time.Second)
 defer cancel()
 cmd:=exec.CommandContext(childCtx,executable,parserArgument+kind)
 cmd.Env=[]string{"GOMAXPROCS=1","GOMEMLIMIT=384MiB","LANG=C.UTF-8","TZ=UTC"}
 cmd.Stdin=bytes.NewReader(input)
 output:=&parserOutput{cancel:cancel}
 cmd.Stdout=output
 cmd.Stderr=io.Discard
 err=cmd.Run()
 if output.exceeded { return nil,platform.NewError(platform.CodeValidation,false,"隔离解析输出超过预算") }
 if err!=nil {
  if ctx.Err()!=nil { return nil,ctx.Err() }
  if childCtx.Err()!=nil { return nil,platform.NewError(platform.CodeDeadlineExceeded,false,"隔离解析达到截止") }
  return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"隔离解析失败或不受支持")
 }
 return output.buffer.Bytes(),nil
}

type parserOutput struct {
 buffer bytes.Buffer
 cancel context.CancelFunc
 exceeded bool
}

func (w *parserOutput) Write(value []byte) (int,error) {
 if len(value)>parserOutputLimit-w.buffer.Len() { w.exceeded=true;w.cancel();return 0,errors.New("parser output limit") }
 return w.buffer.Write(value)
}

// RunParserChild 必须在读取部署配置、打开数据库或服务密钥前调用；普通启动返回false。
// 子进程回调只能解析已限量的内存输入并返回内存结果，不能打开文件或访问网络。
func RunParserChild(kind string, parse func([]byte)([]byte,error)) (bool,error) {
 if len(os.Args)!=2 || !strings.HasPrefix(os.Args[1],parserArgument) { return false,nil }
 if os.Args[1]!=parserArgument+kind || parse==nil { return true,platform.NewError(platform.CodeValidation,false,"未知隔离解析入口") }
 if err:=EnterParserSandbox();err!=nil { return true,err }
 input,err:=io.ReadAll(io.LimitReader(os.Stdin,parserInputLimit+1))
 if err!=nil || len(input)>parserInputLimit { return true,platform.NewError(platform.CodeValidation,false,"隔离解析输入超过预算") }
 result,err:=parse(input)
 if err!=nil { return true,platform.NewError(platform.CodeValidation,false,"隔离解析内容不受支持") }
 if len(result)>parserOutputLimit { return true,platform.NewError(platform.CodeValidation,false,"隔离解析输出超过预算") }
 _,err=os.Stdout.Write(result)
 return true,err
}
