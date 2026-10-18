package trafficguard

import (
 "context"
 "crypto/sha256"
)

type VerifyPoWInput struct { Nonce string;Solution string;Difficulty int }
type VerifyPoWResult struct { Valid bool }

// VerifyPoW 只计算一次SHA256；求解循环由浏览器现有worker拥有。
func VerifyPoW(ctx context.Context,op OperationContext,in VerifyPoWInput)(VerifyPoWResult,error) {
 if len(in.Solution)==0||len(in.Solution)>64 { return VerifyPoWResult{Valid:false},nil }
 for _,b:=range []byte(in.Solution) { if b<33||b>126 { return VerifyPoWResult{Valid:false},nil } }
 digest:=sha256.Sum256([]byte(in.Nonce+":"+in.Solution))
 for bit:=0;bit<in.Difficulty;bit++ { if digest[bit/8]&(1<<uint(7-bit%8))!=0 { return VerifyPoWResult{Valid:false},nil } }
 return VerifyPoWResult{Valid:true},nil
}
