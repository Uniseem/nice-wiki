package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/identity/internal"
	"nice-wiki/modules/identity/internal/gen"
)

// v2Operation 把“请求体 + 路径/查询参数”组装成用例输入再执行：B 是请求体类型，I 是用例输入。
func v2Operation[B any, I any, O any](a *Application, execute func(context.Context, identity.OperationContext, I) (O, error), build func(B, *http.Request) I, prepare ...func(context.Context, *http.Request) context.Context) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := a.identityContext(r.Context())
		for _, p := range prepare {
			ctx = p(ctx, r)
		}
		op, _ := platform.OperationContextFrom(ctx)
		var body B
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "身份输入不可解码"))
				return
			}
		}
		result, err := execute(ctx, op, build(body, r))
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}

type none = struct{}

func queryPage(_ none, r *http.Request) identity.PageInput {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return identity.PageInput{Limit: limit, Cursor: r.URL.Query().Get("cursor")}
}

func withProvider(ctx context.Context, r *http.Request) context.Context {
	return identity.WithPathProvider(ctx, r.PathValue("provider"))
}

// registerV2 登记多用户平台新增的接口。
func (a *Application) registerV2(handlers map[string]http.Handler) {
	handlers["readAuthOptions"] = jsonOperation(a, identity.ReadAuthOptions)
	handlers["registerUser"] = jsonOperation(a, identity.RegisterUser)
	handlers["oauthStart"] = v2Operation(a, identity.OauthStart, func(b gen.OauthStartInput, _ *http.Request) gen.OauthStartInput { return b }, withProvider)
	handlers["listMyIdentities"] = jsonOperation(a, identity.ListMyIdentities)
	handlers["startIdentityLink"] = v2Operation(a, identity.StartIdentityLink, func(b gen.OauthStartInput, _ *http.Request) gen.OauthStartInput { return b }, withProvider)
	handlers["linkIdentity"] = v2Operation(a, identity.LinkIdentity, func(b gen.OauthLoginInput, _ *http.Request) gen.OauthLoginInput { return b }, withProvider)
	handlers["unlinkIdentity"] = v2Operation(a, identity.UnlinkIdentity, func(_ none, _ *http.Request) struct{} { return struct{}{} }, withProvider)
	handlers["oauthLogin"] = v2Operation(a, identity.OauthLogin, func(b gen.OauthLoginInput, _ *http.Request) gen.OauthLoginInput { return b }, withProvider)
	handlers["listMySites"] = jsonOperation(a, identity.ListMySites)
	handlers["createSite"] = jsonOperation(a, identity.CreateSite)
	site := func(_ none, r *http.Request) identity.SiteRef { return identity.SiteRef{SiteID: r.PathValue("siteId")} }
	handlers["readSite"] = v2Operation(a, identity.ReadSite, site)
	handlers["deleteSite"] = v2Operation(a, identity.DeleteSite, site)
	handlers["restoreSite"] = v2Operation(a, identity.RestoreSite, func(_ none, r *http.Request) identity.DeletedSiteRef {
		return identity.DeletedSiteRef{SiteID: r.PathValue("deletedSiteId")}
	})
	handlers["listSiteMembers"] = v2Operation(a, identity.ListSiteMembers, site)
	handlers["listSiteDomains"] = v2Operation(a, identity.ListSiteDomains, site)
	handlers["updateSite"] = v2Operation(a, identity.UpdateSite, func(b gen.UpdateSiteInput, r *http.Request) identity.UpdateSiteCmd {
		return identity.UpdateSiteCmd{SiteID: r.PathValue("siteId"), Body: b}
	})
	handlers["addSiteMember"] = v2Operation(a, identity.AddSiteMember, func(b gen.AddSiteMemberInput, r *http.Request) identity.AddSiteMemberCmd {
		return identity.AddSiteMemberCmd{SiteID: r.PathValue("siteId"), Body: b}
	})
	handlers["updateSiteMember"] = v2Operation(a, identity.UpdateSiteMember, func(b gen.UpdateSiteMemberInput, r *http.Request) identity.UpdateSiteMemberCmd {
		return identity.UpdateSiteMemberCmd{SiteID: r.PathValue("siteId"), UserID: r.PathValue("userId"), Body: b}
	})
	handlers["removeSiteMember"] = v2Operation(a, identity.RemoveSiteMember, func(_ none, r *http.Request) identity.SiteUserRef {
		return identity.SiteUserRef{SiteID: r.PathValue("siteId"), UserID: r.PathValue("userId")}
	})
	handlers["addSiteDomain"] = v2Operation(a, identity.AddSiteDomain, func(b gen.AddSiteDomainInput, r *http.Request) identity.AddSiteDomainCmd {
		return identity.AddSiteDomainCmd{SiteID: r.PathValue("siteId"), Body: b}
	})
	siteHost := func(_ none, r *http.Request) identity.SiteHostRef {
		return identity.SiteHostRef{SiteID: r.PathValue("siteId"), Host: r.PathValue("host")}
	}
	handlers["verifySiteDomain"] = v2Operation(a, identity.VerifySiteDomain, siteHost)
	handlers["setPrimaryDomain"] = v2Operation(a, identity.SetPrimaryDomain, siteHost)
	handlers["removeSiteDomain"] = v2Operation(a, identity.RemoveSiteDomain, siteHost)
	handlers["resolveHost"] = v2Operation(a, identity.ResolveHost, func(_ none, r *http.Request) identity.HostRef { return identity.HostRef{Host: r.PathValue("host")} })
	handlers["issuePrivateAccess"] = v2Operation(a, identity.IssuePrivateAccess, site)
	handlers["setSitePassword"] = v2Operation(a, identity.SetSitePassword, func(b gen.SetSitePasswordInput, r *http.Request) identity.SetSitePasswordCmd {
		return identity.SetSitePasswordCmd{SiteID: r.PathValue("siteId"), Body: b}
	})
	handlers["listShareLinks"] = v2Operation(a, identity.ListShareLinks, site)
	handlers["createShareLink"] = v2Operation(a, identity.CreateShareLink, func(b gen.CreateShareLinkInput, r *http.Request) identity.CreateShareLinkCmd {
		return identity.CreateShareLinkCmd{SiteID: r.PathValue("siteId"), Body: b}
	})
	handlers["revokeShareLink"] = v2Operation(a, identity.RevokeShareLink, func(_ none, r *http.Request) identity.ShareLinkRef {
		return identity.ShareLinkRef{SiteID: r.PathValue("siteId"), LinkID: r.PathValue("linkId")}
	})
	handlers["redeemPrivateAccess"] = v2Operation(a, identity.RedeemPrivateAccess, func(b gen.RedeemPrivateAccessInput, r *http.Request) identity.RedeemPrivateAccessCmd {
		return identity.RedeemPrivateAccessCmd{SiteID: r.PathValue("siteId"), Body: b}
	})
	handlers["allowTlsHost"] = v2Operation(a, identity.AllowTlsHost, func(_ none, r *http.Request) identity.HostRef { return identity.HostRef{Host: r.PathValue("host")} })
	handlers["startDriveAuth"] = v2Operation(a, identity.StartDriveAuth, func(b gen.StartDriveAuthInput, r *http.Request) identity.StartDriveAuthCmd {
		return identity.StartDriveAuthCmd{SiteID: r.PathValue("siteId"), Body: b}
	})
	handlers["completeDriveAuth"] = v2Operation(a, identity.CompleteDriveAuth, func(b gen.CompleteDriveAuthInput, r *http.Request) identity.CompleteDriveAuthCmd {
		return identity.CompleteDriveAuthCmd{SiteID: r.PathValue("siteId"), Body: b}
	})
	handlers["refreshDriveToken"] = jsonOperation(a, identity.RefreshDriveToken)
	handlers["readPlatformSettings"] = jsonOperation(a, identity.ReadPlatformSettings)
	handlers["updatePlatformSettings"] = jsonOperation(a, identity.UpdatePlatformSettings)
	handlers["createRegistrationInvite"] = jsonOperation(a, identity.CreateRegistrationInvite)
	handlers["listRegistrationInvites"] = jsonOperation(a, identity.ListRegistrationInvites)
	handlers["revokeRegistrationInvite"] = v2Operation(a, identity.RevokeRegistrationInvite, func(_ none, r *http.Request) identity.InviteRef {
		return identity.InviteRef{InviteID: r.PathValue("inviteId")}
	})
	handlers["listUsers"] = v2Operation(a, identity.ListUsers, queryPage)
	handlers["resetUserPassword"] = v2Operation(a, identity.ResetUserPassword, func(b gen.ResetUserPasswordInput, r *http.Request) identity.ResetUserPasswordCmd {
		return identity.ResetUserPasswordCmd{UserID: r.PathValue("userId"), Body: b}
	})
	handlers["setUserStatus"] = v2Operation(a, identity.SetUserStatus, func(b gen.SetUserStatusInput, r *http.Request) identity.SetUserStatusCmd {
		return identity.SetUserStatusCmd{UserID: r.PathValue("userId"), Body: b}
	})
	handlers["listAllSites"] = v2Operation(a, identity.ListAllSites, queryPage)
	handlers["setSiteStatus"] = v2Operation(a, identity.SetSiteStatus, func(b gen.SetSiteStatusInput, r *http.Request) identity.SetSiteStatusCmd {
		return identity.SetSiteStatusCmd{SiteID: r.PathValue("siteId"), Body: b}
	})
	handlers["createAiInvite"] = jsonOperation(a, identity.CreateAiInvite)
	handlers["listAiInvites"] = jsonOperation(a, identity.ListAiInvites)
	handlers["revokeAiInvite"] = v2Operation(a, identity.RevokeAiInvite, func(_ none, r *http.Request) identity.InviteRef {
		return identity.InviteRef{InviteID: r.PathValue("inviteId")}
	})
	handlers["redeemAiCode"] = jsonOperation(a, identity.RedeemAiCode)
	handlers["readMyAi"] = jsonOperation(a, identity.ReadMyAi)
	handlers["readSiteAiPolicy"] = v2Operation(a, identity.ReadSiteAiPolicy, site)
	handlers["readSiteStoragePolicy"] = v2Operation(a, identity.ReadSiteStoragePolicy, site)
	handlers["noteVisitorQaCap"] = v2Operation(a, identity.NoteVisitorQaCap, site)
	handlers["submitReport"] = jsonOperation(a, identity.SubmitReport)
	handlers["listReports"] = jsonOperation(a, identity.ListReports)
	handlers["resolveReport"] = v2Operation(a, identity.ResolveReport, func(b gen.ResolveReportInput, r *http.Request) identity.ResolveReportCmd {
		return identity.ResolveReportCmd{ReportID: r.PathValue("reportId"), Body: b}
	})
}
