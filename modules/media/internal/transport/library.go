package transport

import (
	"encoding/json"
	"net/http"

	"nice-wiki/libs/platform"
	media "nice-wiki/modules/media/internal"
)

func groupFilter(r *http.Request) *string {
	if !r.URL.Query().Has("groupId") {
		return nil
	}
	v := r.URL.Query().Get("groupId")
	return &v
}

func libraryHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := platform.OperationContextFrom(r.Context())
		var result any
		var err error
		switch name {
		case "listAssetGroups":
			result, err = media.ListAssetGroups(r.Context(), op, media.ListAssetGroupsInput{})
		case "createAssetGroup":
			var input media.CreateAssetGroupInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				err = platform.NewError(platform.CodeValidation, false, "分组名称不可读")
			} else {
				result, err = media.CreateAssetGroup(r.Context(), op, input)
			}
		case "deleteAssetGroup":
			result, err = media.DeleteAssetGroup(r.Context(), op, media.DeleteAssetGroupInput{GroupID: r.PathValue("groupId")})
		case "setAssetGroup":
			var input media.SetAssetGroupInput
			if json.NewDecoder(r.Body).Decode(&input) != nil || input.AssetID != r.PathValue("assetId") {
				err = platform.NewError(platform.CodeValidation, false, "资源分组路径与正文不一致")
			} else {
				result, err = media.SetAssetGroup(r.Context(), op, input)
			}
		}
		if err != nil {
			writeRequestProblem(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, result)
	})
}
