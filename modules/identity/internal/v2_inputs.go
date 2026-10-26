package identity

import "nice-wiki/modules/identity/internal/gen"

// v2 接口的输入：路径参数和请求体合在一起交给用例函数。

type SiteRef struct{ SiteID string }
type SiteUserRef struct{ SiteID, UserID string }
type SiteHostRef struct{ SiteID, Host string }
type HostRef struct{ Host string }
type InviteRef struct{ InviteID string }
type PageInput struct {
	Limit  int
	Cursor string
}

type UpdateSiteCmd struct {
	SiteID string
	Body   gen.UpdateSiteInput
}
type AddSiteMemberCmd struct {
	SiteID string
	Body   gen.AddSiteMemberInput
}
type UpdateSiteMemberCmd struct {
	SiteID, UserID string
	Body           gen.UpdateSiteMemberInput
}
type AddSiteDomainCmd struct {
	SiteID string
	Body   gen.AddSiteDomainInput
}
type SetUserStatusCmd struct {
	UserID string
	Body   gen.SetUserStatusInput
}
type SetSiteStatusCmd struct {
	SiteID string
	Body   gen.SetSiteStatusInput
}
type ResolveReportCmd struct {
	ReportID string
	Body     gen.ResolveReportInput
}
type RedeemPrivateAccessCmd struct {
	SiteID string
	Body   gen.RedeemPrivateAccessInput
}
