package httpapi

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

type RuntimeOrphanReason string

const (
	RuntimeOrphanMissingSandboxRecord  RuntimeOrphanReason = "missing-sandbox-record"
	RuntimeOrphanCleanupPending        RuntimeOrphanReason = "cleanup-pending"
	RuntimeOrphanRuntimeRefMismatch    RuntimeOrphanReason = "runtime-ref-mismatch"
	RuntimeOrphanMissingTemplateRecord RuntimeOrphanReason = "missing-template-record"
	RuntimeOrphanUnlabeledOwner        RuntimeOrphanReason = "unlabeled-owner"
)

type RuntimeOrphan struct {
	Reason     RuntimeOrphanReason         `json:"reason"`
	Resource   mboxruntime.ManagedResource `json:"resource"`
	SandboxID  *uuid.UUID                  `json:"sandboxId,omitempty"`
	TemplateID *uuid.UUID                  `json:"templateId,omitempty"`
	ProjectID  *uuid.UUID                  `json:"projectId,omitempty"`
	RuntimeRef *domain.RuntimeRef          `json:"runtimeRef,omitempty"`
	Status     domain.SandboxStatus        `json:"status,omitempty"`
	DeletedAt  *time.Time                  `json:"deletedAt,omitempty"`
	Message    string                      `json:"message"`
	Evidence   []string                    `json:"evidence,omitempty"`
}

type RuntimeOrphanAudit struct {
	Adapter       string          `json:"adapter"`
	CheckedAt     time.Time       `json:"checkedAt"`
	Namespace     string          `json:"namespace,omitempty"`
	ResourceCount int             `json:"resourceCount"`
	OrphanCount   int             `json:"orphanCount"`
	ExpectedClean bool            `json:"expectedClean"`
	Items         []RuntimeOrphan `json:"items"`
}

type RuntimeOrphanCleanupRequest struct {
	Resource     mboxruntime.ManagedResourceRef `json:"resource"`
	Reason       RuntimeOrphanReason            `json:"reason"`
	Confirm      string                         `json:"confirm"`
	DeleteOrphan bool                           `json:"deleteOrphan"`
}

type RuntimeOrphanCleanupResult struct {
	Deleted  bool                           `json:"deleted"`
	Resource mboxruntime.ManagedResourceRef `json:"resource"`
	Reason   RuntimeOrphanReason            `json:"reason"`
	Message  string                         `json:"message"`
}

func (api *API) listRuntimeResources(w http.ResponseWriter, r *http.Request) {
	if api.auditor == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime auditor is not configured")
		return
	}
	managed, err := api.auditor.ListManagedResources(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if namespace != "" {
		managed.Items = filterManagedResourcesByNamespace(managed.Items, namespace)
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	if kind != "" {
		managed.Items = filterManagedResourcesByKind(managed.Items, kind)
	}
	writeJSON(w, http.StatusOK, mboxruntime.ManagedResourceList{
		Adapter:   managed.Adapter,
		CheckedAt: managed.CheckedAt,
		Summary:   summarizeManagedResources(managed.Items),
		Items:     managed.Items,
	})
}

func (api *API) listRuntimeOrphans(w http.ResponseWriter, r *http.Request) {
	if api.auditor == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime auditor is not configured")
		return
	}

	audit, err := api.runtimeOrphanAudit(r)
	if err != nil {
		writeStoreOrRuntimeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, audit)
}

func (api *API) cleanupRuntimeOrphan(w http.ResponseWriter, r *http.Request) {
	if api.auditor == nil || api.cleaner == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime cleaner is not configured")
		return
	}

	var input RuntimeOrphanCleanupRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Resource.Adapter = strings.TrimSpace(input.Resource.Adapter)
	input.Resource.Kind = strings.TrimSpace(input.Resource.Kind)
	input.Resource.Namespace = strings.TrimSpace(input.Resource.Namespace)
	input.Resource.Name = strings.TrimSpace(input.Resource.Name)
	input.Confirm = strings.TrimSpace(input.Confirm)
	if !input.DeleteOrphan || input.Confirm != "delete-orphan-runtime-resource" {
		writeError(w, http.StatusBadRequest, "cleanup requires deleteOrphan=true and confirm=\"delete-orphan-runtime-resource\"")
		return
	}
	if input.Resource.Adapter == "" || input.Resource.Kind == "" || input.Resource.Namespace == "" || input.Resource.Name == "" {
		writeError(w, http.StatusBadRequest, "resource adapter, kind, namespace, and name are required")
		return
	}
	if input.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason is required")
		return
	}

	audit, err := api.runtimeOrphanAudit(r)
	if err != nil {
		writeStoreOrRuntimeError(w, err)
		return
	}
	item, ok := findRuntimeOrphan(audit.Items, input.Resource)
	if !ok {
		writeError(w, http.StatusConflict, "runtime resource is not currently reported as an orphan")
		return
	}
	if item.Reason != input.Reason {
		writeError(w, http.StatusConflict, "runtime orphan reason changed from requested reason")
		return
	}

	if err := api.cleaner.DeleteManagedResource(r.Context(), input.Resource); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	result := RuntimeOrphanCleanupResult{
		Deleted:  true,
		Resource: input.Resource,
		Reason:   item.Reason,
		Message:  "orphan runtime resource deleted",
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    item.ProjectID,
		Action:       "runtime.orphan.deleted",
		ResourceType: strings.ToLower(input.Resource.Kind),
		ResourceName: input.Resource.Name,
		Metadata: auditMetadata(map[string]any{
			"adapter":   input.Resource.Adapter,
			"namespace": input.Resource.Namespace,
			"reason":    item.Reason,
		}),
	})
	writeJSON(w, http.StatusOK, result)
}

type runtimeAuditError struct {
	status  int
	message string
}

func (e runtimeAuditError) Error() string {
	return e.message
}

func (api *API) runtimeOrphanAudit(r *http.Request) (RuntimeOrphanAudit, error) {
	managed, err := api.auditor.ListManagedResources(r.Context())
	if err != nil {
		return RuntimeOrphanAudit{}, runtimeAuditError{status: http.StatusBadGateway, message: err.Error()}
	}
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if namespace != "" {
		managed.Items = filterManagedResourcesByNamespace(managed.Items, namespace)
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	if kind != "" {
		managed.Items = filterManagedResourcesByKind(managed.Items, kind)
	}
	sandboxes, err := api.store.ListAllSandboxes(r.Context())
	if err != nil {
		return RuntimeOrphanAudit{}, err
	}
	templates, err := api.store.ListAllTemplates(r.Context())
	if err != nil {
		return RuntimeOrphanAudit{}, err
	}

	audit := buildRuntimeOrphanAudit(managed, sandboxes, templates)
	audit.Namespace = namespace
	return audit, nil
}

func writeStoreOrRuntimeError(w http.ResponseWriter, err error) {
	var runtimeErr runtimeAuditError
	if errors.As(err, &runtimeErr) {
		writeError(w, runtimeErr.status, runtimeErr.message)
		return
	}
	writeStoreError(w, err)
}

func filterManagedResourcesByNamespace(resources []mboxruntime.ManagedResource, namespace string) []mboxruntime.ManagedResource {
	filtered := make([]mboxruntime.ManagedResource, 0, len(resources))
	for _, resource := range resources {
		if resource.Namespace == namespace {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

func filterManagedResourcesByKind(resources []mboxruntime.ManagedResource, kind string) []mboxruntime.ManagedResource {
	filtered := make([]mboxruntime.ManagedResource, 0, len(resources))
	for _, resource := range resources {
		if resource.Kind == kind {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

func summarizeManagedResources(resources []mboxruntime.ManagedResource) mboxruntime.ManagedResourceSummary {
	byKind := make(map[string]int)
	byNamespace := make(map[string]int)
	byOwner := make(map[string]int)
	for _, resource := range resources {
		byKind[resource.Kind]++
		if resource.Namespace != "" {
			byNamespace[resource.Namespace]++
		}
		if key := managedResourceOwnerKey(resource.Owner); key != "" {
			byOwner[key]++
		}
	}
	return mboxruntime.ManagedResourceSummary{
		Total:       len(resources),
		ByKind:      managedResourceCounts(byKind),
		ByNamespace: managedResourceCounts(byNamespace),
		ByOwner:     managedResourceCounts(byOwner),
	}
}

func managedResourceOwnerKey(owner *mboxruntime.ManagedResourceOwner) string {
	if owner == nil {
		return ""
	}
	switch owner.Kind {
	case "sandbox":
		if owner.SandboxID == "" {
			return ""
		}
		if owner.ProjectID != "" {
			return "project/" + owner.ProjectID + "/sandbox/" + owner.SandboxID
		}
		return "sandbox/" + owner.SandboxID
	case "template":
		if owner.TemplateID == "" {
			return ""
		}
		return "template/" + owner.TemplateID
	default:
		return ""
	}
}

func managedResourceCounts(counts map[string]int) []mboxruntime.ManagedResourceCount {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]mboxruntime.ManagedResourceCount, 0, len(names))
	for _, name := range names {
		items = append(items, mboxruntime.ManagedResourceCount{
			Name:  name,
			Count: counts[name],
		})
	}
	return items
}

func findRuntimeOrphan(items []RuntimeOrphan, ref mboxruntime.ManagedResourceRef) (RuntimeOrphan, bool) {
	for _, item := range items {
		resourceRef := mboxruntime.ManagedResourceRef{
			Adapter:   item.Resource.Adapter,
			Kind:      item.Resource.Kind,
			Namespace: item.Resource.Namespace,
			Name:      item.Resource.Name,
		}
		if sameManagedResourceRef(resourceRef, ref) {
			return item, true
		}
	}
	return RuntimeOrphan{}, false
}

func sameManagedResourceRef(actual mboxruntime.ManagedResourceRef, expected mboxruntime.ManagedResourceRef) bool {
	return actual.Adapter == expected.Adapter &&
		actual.Kind == expected.Kind &&
		actual.Namespace == expected.Namespace &&
		actual.Name == expected.Name
}

func buildRuntimeOrphanAudit(managed mboxruntime.ManagedResourceList, sandboxes []domain.Sandbox, templates []domain.EnvironmentTemplate) RuntimeOrphanAudit {
	sandboxByID := make(map[uuid.UUID]domain.Sandbox, len(sandboxes))
	for _, sandbox := range sandboxes {
		sandboxByID[sandbox.ID] = sandbox
	}
	templateByID := make(map[uuid.UUID]domain.EnvironmentTemplate, len(templates))
	for _, template := range templates {
		templateByID[template.ID] = template
	}

	items := []RuntimeOrphan{}
	for _, resource := range managed.Items {
		switch resource.Kind {
		case "SandboxClaim":
			items = append(items, classifySandboxClaim(resource, sandboxByID)...)
		case "SandboxTemplate":
			items = append(items, classifySandboxTemplate(resource, templateByID)...)
		}
	}

	return RuntimeOrphanAudit{
		Adapter:       managed.Adapter,
		CheckedAt:     managed.CheckedAt,
		ResourceCount: len(managed.Items),
		OrphanCount:   len(items),
		ExpectedClean: len(items) == 0,
		Items:         items,
	}
}

func classifySandboxClaim(resource mboxruntime.ManagedResource, sandboxByID map[uuid.UUID]domain.Sandbox) []RuntimeOrphan {
	rawID := resource.Labels["mbox.dev/sandbox-id"]
	sandboxID, err := uuid.Parse(rawID)
	if rawID == "" || err != nil {
		return []RuntimeOrphan{{
			Reason:   RuntimeOrphanUnlabeledOwner,
			Resource: resource,
			Message:  "managed SandboxClaim is missing a valid mbox.dev/sandbox-id label",
			Evidence: []string{"kind=SandboxClaim"},
		}}
	}

	sandbox, ok := sandboxByID[sandboxID]
	if !ok {
		return []RuntimeOrphan{{
			Reason:    RuntimeOrphanMissingSandboxRecord,
			Resource:  resource,
			SandboxID: &sandboxID,
			Message:   "managed SandboxClaim has no matching sandbox product record",
			Evidence:  []string{"label mbox.dev/sandbox-id=" + sandboxID.String()},
		}}
	}

	expected := domain.RuntimeRef{
		Adapter:   resource.Adapter,
		Kind:      "SandboxClaim",
		Namespace: resource.Namespace,
		Name:      resource.Name,
	}
	if sandbox.DeletedAt != nil {
		return []RuntimeOrphan{{
			Reason:     RuntimeOrphanCleanupPending,
			Resource:   resource,
			SandboxID:  &sandbox.ID,
			ProjectID:  &sandbox.ProjectID,
			RuntimeRef: sandbox.RuntimeRef,
			Status:     sandbox.Status,
			DeletedAt:  sandbox.DeletedAt,
			Message:    "sandbox is soft-deleted while its managed runtime resource is still present",
			Evidence:   []string{"sandbox.status=" + string(sandbox.Status), "runtimeRef cleanup is pending"},
		}}
	}
	if sandbox.RuntimeRef == nil || !sameRuntimeRef(*sandbox.RuntimeRef, expected) {
		return []RuntimeOrphan{{
			Reason:     RuntimeOrphanRuntimeRefMismatch,
			Resource:   resource,
			SandboxID:  &sandbox.ID,
			ProjectID:  &sandbox.ProjectID,
			RuntimeRef: sandbox.RuntimeRef,
			Status:     sandbox.Status,
			Message:    "managed SandboxClaim does not match the sandbox product runtimeRef",
			Evidence:   runtimeRefMismatchEvidence(sandbox.RuntimeRef, expected),
		}}
	}
	return nil
}

func classifySandboxTemplate(resource mboxruntime.ManagedResource, templateByID map[uuid.UUID]domain.EnvironmentTemplate) []RuntimeOrphan {
	rawID := resource.Labels["mbox.dev/template-id"]
	templateID, err := uuid.Parse(rawID)
	if rawID == "" || err != nil {
		return []RuntimeOrphan{{
			Reason:   RuntimeOrphanUnlabeledOwner,
			Resource: resource,
			Message:  "managed SandboxTemplate is missing a valid mbox.dev/template-id label",
			Evidence: []string{"kind=SandboxTemplate"},
		}}
	}
	if _, ok := templateByID[templateID]; !ok {
		return []RuntimeOrphan{{
			Reason:     RuntimeOrphanMissingTemplateRecord,
			Resource:   resource,
			TemplateID: &templateID,
			Message:    "managed SandboxTemplate has no matching template product record",
			Evidence:   []string{"label mbox.dev/template-id=" + templateID.String()},
		}}
	}
	return nil
}

func sameRuntimeRef(actual domain.RuntimeRef, expected domain.RuntimeRef) bool {
	return actual.Adapter == expected.Adapter &&
		actual.Kind == expected.Kind &&
		actual.Namespace == expected.Namespace &&
		actual.Name == expected.Name
}

func runtimeRefMismatchEvidence(actual *domain.RuntimeRef, expected domain.RuntimeRef) []string {
	evidence := []string{"expected=" + runtimeRefString(expected)}
	if actual == nil {
		return append(evidence, "actual=<nil>")
	}
	return append(evidence, "actual="+runtimeRefString(*actual))
}

func runtimeRefString(ref domain.RuntimeRef) string {
	return ref.Adapter + "/" + ref.Kind + "/" + ref.Namespace + "/" + ref.Name
}
