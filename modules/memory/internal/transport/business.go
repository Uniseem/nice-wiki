package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/assistant"
	memorycontext "nice-wiki/modules/memory/internal/context"
	"nice-wiki/modules/memory/internal/entry"
	"nice-wiki/modules/memory/internal/mapping"
	"nice-wiki/modules/memory/internal/proposal"
	"strconv"
)

// operationHandlers 只登记已实现用例；统一工厂仍拒绝缺少其他操作的整个启动。
func (a *Application) operationHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"health": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			v, err := Health(r.Context(), a)
			respond(w, r, v, err, http.StatusOK)
		}),
		"ready": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			v, err := Ready(r.Context(), a)
			respond(w, r, v, err, http.StatusOK)
		}),
		"createAssistantRun": jsonOperation(assistant.CreateAssistantRun, http.StatusAccepted),
		"cancelAssistantRun": jsonOperation(assistant.CancelAssistantRun, http.StatusOK, func(i *assistant.CancelAssistantRunInput, r *http.Request) error {
			return matchPath(i.RunId, r.PathValue("runId"))
		}),
		"approveProposal": jsonOperation(proposal.ApproveProposal, http.StatusAccepted, func(i *proposal.ApproveProposalInput, r *http.Request) error {
			return matchPath(i.ProposalId, r.PathValue("proposalId"))
		}),
		"deleteAssistantConversation": jsonOperation(assistant.DeleteAssistantConversation, http.StatusOK, func(i *assistant.DeleteAssistantConversationInput, r *http.Request) error {
			i.ConversationId = r.PathValue("conversationId")
			i.IdempotencyKey = r.Header.Get("Idempotency-Key")
			return nil
		}),
		"createEntry": jsonOperation(entry.CreateEntry, http.StatusCreated),
		"updateEntry": jsonOperation(entry.UpdateEntry, http.StatusOK, func(i *entry.UpdateEntryInput, r *http.Request) error {
			return matchPath(i.Entry.EntryId, r.PathValue("entryId"))
		}),
		"archiveEntry": jsonOperation(entry.ArchiveEntry, http.StatusOK, func(i *entry.ArchiveEntryInput, r *http.Request) error {
			return matchPath(i.EntryId, r.PathValue("entryId"))
		}),
		"restoreEntry": jsonOperation(entry.RestoreEntry, http.StatusOK, func(i *entry.RestoreEntryInput, r *http.Request) error {
			return matchPath(i.EntryId, r.PathValue("entryId"))
		}),
		"readEntry": jsonOperation(entry.ReadEntry, http.StatusOK, func(i *entry.ReadEntryInput, r *http.Request) error {
			i.EntryId = r.PathValue("entryId")
			i.Version = optionalInt(r, "version")
			return nil
		}),
		"listEntries": jsonOperation(entry.ListEntries, http.StatusOK, func(i *entry.ListEntriesInput, r *http.Request) error {
			i.State = optionalString(r, "state")
			i.Language = optionalString(r, "language")
			i.Cursor = optionalString(r, "cursor")
			i.Limit = queryLimit(r)
			return nil
		}),
		"listEntryRevisions": jsonOperation(entry.ListEntryRevisions, http.StatusOK, func(i *entry.ListEntryRevisionsInput, r *http.Request) error {
			i.EntryId = r.PathValue("entryId")
			i.BeforeVersion = optionalInt(r, "beforeVersion")
			i.Limit = queryLimit(r)
			return nil
		}),
		"putMapping": jsonOperation(mapping.PutMapping, http.StatusOK, func(i *mapping.PutMappingInput, r *http.Request) error {
			return matchPath(i.Mapping.MappingId, r.PathValue("mappingId"))
		}),
		"removeMapping": jsonOperation(mapping.RemoveMapping, http.StatusOK, func(i *mapping.RemoveMappingInput, r *http.Request) error {
			i.MappingId = r.PathValue("mappingId")
			i.ExpectedRevision, _ = strconv.Atoi(r.URL.Query().Get("expectedRevision"))
			i.IdempotencyKey = r.Header.Get("Idempotency-Key")
			return nil
		}),
		"listMappings": jsonOperation(mapping.ListMappings, http.StatusOK, func(i *mapping.ListMappingsInput, r *http.Request) error {
			i.TaskId = r.URL.Query().Get("taskId")
			i.Language = optionalString(r, "language")
			return nil
		}),
		"reorderMappings": jsonOperation(mapping.ReorderMappings, http.StatusOK),
		"putTaskBudget": jsonOperation(memorycontext.PutTaskBudget, http.StatusOK, func(i *memorycontext.PutTaskBudgetInput, r *http.Request) error {
			return matchPath(i.Budget.TaskId, r.PathValue("taskId"))
		}),
		"listTaskBudgets": jsonOperation(memorycontext.ListTaskBudgets, http.StatusOK),
		"resolveContext":  jsonOperation(memorycontext.ResolveContext, http.StatusOK),
		"readContextManifest": jsonOperation(memorycontext.ReadContextManifest, http.StatusOK, func(i *memorycontext.ReadContextManifestInput, r *http.Request) error {
			i.VersionHash = r.PathValue("versionHash")
			return nil
		}),
		"appendAssistantMessage": jsonOperation(assistant.AppendAssistantMessage, http.StatusCreated),
		"readAssistantConversation": jsonOperation(assistant.ReadAssistantConversation, http.StatusOK, func(i *assistant.ReadAssistantConversationInput, r *http.Request) error {
			i.ConversationId = r.PathValue("conversationId")
			i.Cursor = optionalString(r, "cursor")
			i.Limit = queryLimit(r)
			return nil
		}),
		"readAssistantRun": jsonOperation(assistant.ReadAssistantRun, http.StatusOK, func(i *assistant.ReadAssistantRunInput, r *http.Request) error {
			i.RunId = r.PathValue("runId")
			return nil
		}),
		"rejectProposal": jsonOperation(proposal.RejectProposal, http.StatusOK, func(i *proposal.RejectProposalInput, r *http.Request) error {
			return matchPath(i.ProposalId, r.PathValue("proposalId"))
		}),
	}
}
func jsonOperation[I, R any](fn func(context.Context, platform.OperationContext, I) (R, error), status int, bind ...func(*I, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input I
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "记忆请求无法转换"))
				return
			}
		}
		for _, bindInput := range bind {
			if err := bindInput(&input, r); err != nil {
				writeProblem(w, err)
				return
			}
		}
		op, _ := platform.OperationContextFrom(r.Context())
		result, err := fn(r.Context(), op, input)
		respond(w, r, result, err, status)
	})
}
func respond(w http.ResponseWriter, r *http.Request, result any, err error, status int) {
	if err != nil {
		writeProblem(w, err)
		return
	}
	writeJSON(w, status, result)
}
func matchPath(body, path string) error {
	if body != path {
		return platform.NewError(platform.CodeValidation, false, "路径与请求对象不一致")
	}
	return nil
}
func optionalString(r *http.Request, name string) *string {
	if !r.URL.Query().Has(name) {
		return nil
	}
	v := r.URL.Query().Get(name)
	return &v
}
func optionalInt(r *http.Request, name string) *int {
	if !r.URL.Query().Has(name) {
		return nil
	}
	v, _ := strconv.Atoi(r.URL.Query().Get(name))
	return &v
}
func queryLimit(r *http.Request) int {
	if !r.URL.Query().Has("limit") {
		return 20
	}
	v, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return v
}
