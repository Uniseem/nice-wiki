package trafficguard

import "context"

type RefillBucketInput struct {
 TokensMilli int64
 Capacity int64
 ElapsedMs int64
 RatePerMinute int64
}
type RefillBucketResult struct { TokensMilli int64 }

// RefillBucket 只做整数补桶；策略边界保证乘法在int64范围内。
func RefillBucket(ctx context.Context,op OperationContext,in RefillBucketInput)(RefillBucketResult,error) {
 elapsed:=in.ElapsedMs
 if elapsed<0 { elapsed=0 }
 if elapsed>60000 { elapsed=60000 }
 capacity:=in.Capacity*1000
 tokens:=in.TokensMilli
 if tokens>capacity { tokens=capacity }
 refill:=elapsed*in.RatePerMinute/60
 if refill>capacity-tokens { tokens=capacity } else { tokens+=refill }
 return RefillBucketResult{TokensMilli:tokens},nil
}
