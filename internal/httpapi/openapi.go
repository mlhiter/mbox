package httpapi

import (
	"net/http"
	"strings"
)

var knownAuditActions = []string{
	"project.created",
	"project.updated",
	"project.deleted",
	"project.policy.updated",
	"project.quota_policy.updated",
	"project.credential.created",
	"project.credential.deleted",
	"template.created",
	"template.updated",
	"template.deleted",
	"template.validation.started",
	"template.validation.decided",
	"sandbox.created",
	"sandbox.updated",
	"sandbox.deleted",
	"sandbox.stopped",
	"sandbox.started",
	"runtime.session.created",
	"runtime.session.ended",
	"execution.task.created",
	"execution.task.cancel.requested",
	"artifact.created",
	"artifact.content.captured",
	"artifact.content.uploaded",
	"runtime.orphan.deleted",
	"policy.denied",
}

var knownPolicyDeniedOperations = []string{
	"sandbox.launch",
	"template.validation",
	"artifact.content.capture",
	"artifact.content.upload",
}

type openAPIDocument map[string]any

func (api *API) getOpenAPI(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, buildOpenAPI(api.info))
}

func buildOpenAPI(info APIInfo) openAPIDocument {
	return openAPIDocument{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "mbox API",
			"version":     info.APIVersion,
			"description": "Product API for mbox Kubernetes execution-platform primitives.",
		},
		"servers": []map[string]any{
			{"url": "http://127.0.0.1:18080", "description": "Default local API server"},
		},
		"tags": []map[string]any{
			{"name": "system"},
			{"name": "runtime"},
			{"name": "audit"},
			{"name": "projects"},
			{"name": "templates"},
			{"name": "sandboxes"},
			{"name": "sessions"},
			{"name": "tasks"},
			{"name": "artifacts"},
			{"name": "credentials"},
		},
		"paths":      openAPIPaths(),
		"components": openAPIComponents(),
	}
}

type openAPIOperationOption func(map[string]any)

func openAPIPaths() map[string]any {
	return map[string]any{
		"/healthz": map[string]any{
			"get": operation("system", "Health check", nil, nil, schemaRef("Health"), false, publicOperation()),
		},
		"/v1/info": map[string]any{
			"get": operation("system", "Get API capability manifest", nil, nil, schemaRef("APIInfo"), false, publicOperation()),
		},
		"/v1/openapi.json": map[string]any{
			"get": operation("system", "Get OpenAPI contract", nil, nil, map[string]any{"type": "object"}, false),
		},
		"/v1/runtime/resources": map[string]any{
			"get": operation("runtime", "List managed runtime resources", []map[string]any{
				queryParam("namespace", "string"),
				queryParam("kind", "string"),
			}, nil, schemaRef("RuntimeResourceList"), false),
		},
		"/v1/runtime/orphans": map[string]any{
			"get": operation("runtime", "List runtime orphan audit", []map[string]any{
				queryParam("namespace", "string"),
				queryParam("kind", "string"),
			}, nil, schemaRef("RuntimeOrphanAudit"), false),
		},
		"/v1/runtime/orphans/cleanup": map[string]any{
			"post": operation("runtime", "Cleanup one reported runtime orphan", nil, schemaRef("RuntimeOrphanCleanupRequest"), schemaRef("RuntimeOrphanCleanupResult"), false),
		},
		"/v1/audit-events": map[string]any{
			"get": operation("audit", "List audit events", auditQueryParams(true), nil, listSchema("AuditEvent"), false),
		},
		"/v1/projects": map[string]any{
			"get":  operation("projects", "List projects", nil, nil, listSchema("Project"), false),
			"post": operation("projects", "Create project", nil, schemaRef("ProjectCreate"), schemaRef("Project"), true),
		},
		"/v1/projects/{projectID}": map[string]any{
			"get":    operation("projects", "Get project", pathParams("projectID"), nil, schemaRef("Project"), false),
			"patch":  operation("projects", "Update project", pathParams("projectID"), schemaRef("ProjectUpdate"), schemaRef("Project"), false),
			"delete": operationNoContent("projects", "Delete project", pathParams("projectID")),
		},
		"/v1/projects/{projectID}/policy": map[string]any{
			"get": operation("projects", "Get project launch policy", pathParams("projectID"), nil, schemaRef("ProjectPolicy"), false),
			"put": operation("projects", "Upsert project launch policy", pathParams("projectID"), schemaRef("ProjectPolicyUpsert"), schemaRef("ProjectPolicy"), false),
		},
		"/v1/projects/{projectID}/quota-policy": map[string]any{
			"get": operation("projects", "Get project quota policy", pathParams("projectID"), nil, schemaRef("ProjectQuotaPolicy"), false),
			"put": operation("projects", "Upsert project quota policy", pathParams("projectID"), schemaRef("ProjectQuotaPolicyUpsert"), schemaRef("ProjectQuotaPolicy"), false),
		},
		"/v1/projects/{projectID}/credentials": map[string]any{
			"get":  operation("projects", "List project credential references", pathParams("projectID"), nil, listSchema("ProjectCredential"), false),
			"post": operation("projects", "Create project credential reference", pathParams("projectID"), schemaRef("ProjectCredentialCreate"), schemaRef("ProjectCredential"), true),
		},
		"/v1/projects/{projectID}/usage": map[string]any{
			"get": operation("projects", "Get project product-record usage", pathParams("projectID"), nil, schemaRef("ProjectUsage"), false),
		},
		"/v1/projects/{projectID}/audit-events": map[string]any{
			"get": operation("projects", "List project audit events", append(pathParams("projectID"), auditQueryParams(false)...), nil, listSchema("AuditEvent"), false),
		},
		"/v1/credentials/{credentialID}": map[string]any{
			"get":    operation("credentials", "Get credential reference", pathParams("credentialID"), nil, schemaRef("ProjectCredential"), false),
			"delete": operationNoContent("credentials", "Delete credential reference", pathParams("credentialID")),
		},
		"/v1/templates": map[string]any{
			"get": operation("templates", "List templates", []map[string]any{
				queryParam("projectId", "string"),
			}, nil, listSchema("EnvironmentTemplate"), false),
			"post": operation("templates", "Create template", nil, schemaRef("TemplateCreate"), schemaRef("EnvironmentTemplate"), true),
		},
		"/v1/templates/{templateID}": map[string]any{
			"get":    operation("templates", "Get template", pathParams("templateID"), nil, schemaRef("EnvironmentTemplate"), false),
			"patch":  operation("templates", "Update template", pathParams("templateID"), schemaRef("TemplateUpdate"), schemaRef("EnvironmentTemplate"), false),
			"delete": operationNoContent("templates", "Delete template", pathParams("templateID")),
		},
		"/v1/templates/{templateID}/boundary": map[string]any{
			"get": operation("templates", "Get template boundary summary", append(pathParams("templateID"), queryParam("projectId", "string")), nil, schemaRef("BoundarySummary"), false),
		},
		"/v1/templates/{templateID}/validation-runs": map[string]any{
			"post": operation("templates", "Create template validation run", pathParams("templateID"), schemaRef("TemplateValidationRunCreate"), schemaRef("TemplateValidationRun"), true),
		},
		"/v1/templates/{templateID}/validation-runs/{sandboxID}/decision": map[string]any{
			"post": operation("templates", "Decide template validation run", append(pathParams("templateID"), pathParam("sandboxID")), schemaRef("TemplateValidationRunDecision"), schemaRef("TemplateValidationRun"), false),
		},
		"/v1/sandboxes": map[string]any{
			"get": operation("sandboxes", "List sandboxes", []map[string]any{
				queryParam("projectId", "string"),
			}, nil, listSchema("Sandbox"), false),
			"post": operation("sandboxes", "Create sandbox", nil, schemaRef("SandboxCreate"), schemaRef("Sandbox"), true),
		},
		"/v1/sandboxes/{sandboxID}": map[string]any{
			"get":    operation("sandboxes", "Get sandbox", pathParams("sandboxID"), nil, schemaRef("Sandbox"), false),
			"patch":  operation("sandboxes", "Update sandbox", pathParams("sandboxID"), schemaRef("SandboxUpdate"), schemaRef("Sandbox"), false),
			"delete": operationNoContent("sandboxes", "Delete sandbox", pathParams("sandboxID")),
		},
		"/v1/sandboxes/{sandboxID}/boundary": map[string]any{
			"get": operation("sandboxes", "Get sandbox boundary summary", pathParams("sandboxID"), nil, schemaRef("BoundarySummary"), false),
		},
		"/v1/sandboxes/{sandboxID}/start": map[string]any{
			"post": operation("sandboxes", "Start sandbox", pathParams("sandboxID"), nil, schemaRef("Sandbox"), false),
		},
		"/v1/sandboxes/{sandboxID}/stop": map[string]any{
			"post": operation("sandboxes", "Stop sandbox", pathParams("sandboxID"), nil, schemaRef("Sandbox"), false),
		},
		"/v1/sandboxes/{sandboxID}/runtime": map[string]any{
			"get": operation("runtime", "Resolve sandbox runtime target", pathParams("sandboxID"), nil, schemaRef("RuntimeTarget"), false),
		},
		"/v1/sandboxes/{sandboxID}/logs": map[string]any{
			"get": operation("runtime", "Read sandbox runtime logs", append(pathParams("sandboxID"), queryParam("tailLines", "integer")), nil, schemaRef("LogResult"), false),
		},
		"/v1/sandboxes/{sandboxID}/events": map[string]any{
			"get": operation("runtime", "List sandbox runtime events", pathParams("sandboxID"), nil, listSchema("RuntimeEvent"), false),
		},
		"/v1/sandboxes/{sandboxID}/ports": map[string]any{
			"get": operation("runtime", "List sandbox preview ports", pathParams("sandboxID"), nil, schemaRef("PreviewPortsResult"), false),
		},
		"/v1/sandboxes/{sandboxID}/ports/{port}/proxy/": map[string]any{
			"get": operation("runtime", "Proxy sandbox preview port", append(pathParams("sandboxID"), pathParam("port")), nil, map[string]any{"type": "string", "format": "binary"}, false),
		},
		"/v1/sandboxes/{sandboxID}/terminal": map[string]any{
			"get": websocketOperation("runtime", "Connect sandbox terminal", append(pathParams("sandboxID"), queryParam("shell", "string"))),
		},
		"/v1/sandboxes/{sandboxID}/sessions": map[string]any{
			"get":  operation("sessions", "List sandbox runtime sessions", pathParams("sandboxID"), nil, listSchema("RuntimeSession"), false),
			"post": operation("sessions", "Create runtime session record", pathParams("sandboxID"), schemaRef("RuntimeSessionCreate"), schemaRef("RuntimeSession"), true),
		},
		"/v1/sandboxes/{sandboxID}/tasks": map[string]any{
			"get":  operation("tasks", "List sandbox execution tasks", pathParams("sandboxID"), nil, listSchema("ExecutionTask"), false),
			"post": operation("tasks", "Create execution task", pathParams("sandboxID"), schemaRef("ExecutionTaskCreate"), schemaRef("ExecutionTask"), true),
		},
		"/v1/sandboxes/{sandboxID}/artifacts": map[string]any{
			"get":  operation("artifacts", "List sandbox artifacts", pathParams("sandboxID"), nil, listSchema("Artifact"), false),
			"post": operation("artifacts", "Create sandbox artifact reference", pathParams("sandboxID"), schemaRef("ArtifactCreate"), schemaRef("Artifact"), true),
		},
		"/v1/sessions/{sessionID}": map[string]any{
			"get": operation("sessions", "Get runtime session", pathParams("sessionID"), nil, schemaRef("RuntimeSession"), false),
		},
		"/v1/sessions/{sessionID}/end": map[string]any{
			"post": operation("sessions", "End runtime session", pathParams("sessionID"), nil, schemaRef("RuntimeSession"), false),
		},
		"/v1/tasks/{taskID}": map[string]any{
			"get": operation("tasks", "Get execution task", pathParams("taskID"), nil, schemaRef("ExecutionTask"), false),
		},
		"/v1/tasks/{taskID}/events": map[string]any{
			"get": ndjsonOperation("tasks", "Watch execution task events", pathParams("taskID")),
		},
		"/v1/tasks/{taskID}/cancel": map[string]any{
			"post": operation("tasks", "Cancel execution task", pathParams("taskID"), nil, schemaRef("ExecutionTask"), false),
		},
		"/v1/tasks/{taskID}/artifacts": map[string]any{
			"get": operation("artifacts", "List task artifacts", pathParams("taskID"), nil, listSchema("Artifact"), false),
		},
		"/v1/artifacts/{artifactID}": map[string]any{
			"get": operation("artifacts", "Get artifact", pathParams("artifactID"), nil, schemaRef("Artifact"), false),
		},
		"/v1/artifacts/{artifactID}/capture": map[string]any{
			"post": operation("artifacts", "Capture workspace artifact content", pathParams("artifactID"), nil, schemaRef("Artifact"), false),
		},
		"/v1/artifacts/{artifactID}/content": map[string]any{
			"get": operation("artifacts", "Read artifact content", pathParams("artifactID"), nil, map[string]any{"type": "string", "format": "binary"}, false),
			"put": operation("artifacts", "Upload retained artifact content", pathParams("artifactID"), binaryBody(), schemaRef("Artifact"), false),
		},
	}
}

func operation(tag string, summary string, parameters []map[string]any, requestSchema any, responseSchema any, created bool, options ...openAPIOperationOption) map[string]any {
	status := "200"
	if created {
		status = "201"
	}
	op := map[string]any{
		"tags":      []string{tag},
		"summary":   summary,
		"security":  bearerSecurity(),
		"responses": responses(status, responseSchema),
	}
	params := appendCommonHeaderParams(parameters)
	if len(params) > 0 {
		op["parameters"] = params
	}
	if requestSchema != nil {
		op["requestBody"] = requestBody(requestSchema)
	}
	for _, option := range options {
		option(op)
	}
	return op
}

func operationNoContent(tag string, summary string, parameters []map[string]any) map[string]any {
	return map[string]any{
		"tags":       []string{tag},
		"summary":    summary,
		"security":   bearerSecurity(),
		"parameters": appendCommonHeaderParams(parameters),
		"responses": map[string]any{
			"204": map[string]any{"description": "Deleted"},
			"401": unauthorizedResponse(),
			"default": map[string]any{
				"description": "Error",
				"content":     jsonContent(schemaRef("Error")),
			},
		},
	}
}

func ndjsonOperation(tag string, summary string, parameters []map[string]any) map[string]any {
	return map[string]any{
		"tags":       []string{tag},
		"summary":    summary,
		"security":   bearerSecurity(),
		"parameters": appendCommonHeaderParams(parameters),
		"responses": map[string]any{
			"200": map[string]any{
				"description": "Newline-delimited task events",
				"content": map[string]any{
					"application/x-ndjson": map[string]any{
						"schema": schemaRef("ExecutionTaskEvent"),
					},
				},
			},
			"401": unauthorizedResponse(),
			"default": map[string]any{
				"description": "Error",
				"content":     jsonContent(schemaRef("Error")),
			},
		},
	}
}

func websocketOperation(tag string, summary string, parameters []map[string]any) map[string]any {
	return map[string]any{
		"tags":       []string{tag},
		"summary":    summary,
		"security":   bearerSecurity(),
		"parameters": appendCommonHeaderParams(parameters),
		"responses": map[string]any{
			"101": map[string]any{"description": "WebSocket upgrade"},
			"401": unauthorizedResponse(),
			"default": map[string]any{
				"description": "Error",
				"content":     jsonContent(schemaRef("Error")),
			},
		},
	}
}

func responses(status string, responseSchema any) map[string]any {
	return map[string]any{
		status: map[string]any{
			"description": "OK",
			"content":     jsonContent(responseSchema),
		},
		"401": unauthorizedResponse(),
		"default": map[string]any{
			"description": "Error",
			"content":     jsonContent(schemaRef("Error")),
		},
	}
}

func publicOperation() openAPIOperationOption {
	return func(op map[string]any) {
		op["security"] = []map[string]any{}
		responses, ok := op["responses"].(map[string]any)
		if ok {
			delete(responses, "401")
		}
	}
}

func bearerSecurity() []map[string]any {
	return []map[string]any{{"bearerAuth": []string{}}}
}

func unauthorizedResponse() map[string]any {
	return map[string]any{
		"description": "Unauthorized",
		"content":     jsonContent(schemaRef("Error")),
	}
}

func requestBody(schema any) map[string]any {
	if body, ok := schema.(map[string]any); ok {
		if _, hasContent := body["content"]; hasContent {
			return body
		}
	}
	return map[string]any{
		"required": true,
		"content":  jsonContent(schema),
	}
}

func jsonContent(schema any) map[string]any {
	return map[string]any{
		"application/json": map[string]any{"schema": schema},
	}
}

func binaryBody() map[string]any {
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/octet-stream": map[string]any{
				"schema": map[string]any{"type": "string", "format": "binary"},
			},
			"text/plain": map[string]any{
				"schema": map[string]any{"type": "string", "format": "binary"},
			},
		},
	}
}

func schemaRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func listSchema(item string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{
				"type":  "array",
				"items": schemaRef(item),
			},
		},
		"required": []string{"items"},
	}
}

func pathParams(names ...string) []map[string]any {
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		out = append(out, pathParam(name))
	}
	return out
}

func pathParam(name string) map[string]any {
	return map[string]any{
		"name":     name,
		"in":       "path",
		"required": true,
		"schema":   map[string]any{"type": "string"},
	}
}

func queryParam(name string, typ string) map[string]any {
	return map[string]any{
		"name":   name,
		"in":     "query",
		"schema": map[string]any{"type": typ},
	}
}

func headerParam(name string) map[string]any {
	description := "Optional client-supplied request correlation id. The server echoes this header and records it in audit event metadata when an audit event is written."
	maxLength := maxRequestIDRunes
	if name == auditActorHeader || name == auditSourceHeader {
		description = "Optional client-supplied audit attribution label. This is not an authentication claim."
		maxLength = maxAuditAttributionRunes
	}
	return map[string]any{
		"name":        name,
		"in":          "header",
		"description": description,
		"schema":      map[string]any{"type": "string", "maxLength": maxLength},
	}
}

func appendCommonHeaderParams(parameters []map[string]any) []map[string]any {
	out := append([]map[string]any{}, parameters...)
	out = append(out, headerParam(requestIDHeader), headerParam(auditActorHeader), headerParam(auditSourceHeader))
	return out
}

func auditQueryParams(includeProject bool) []map[string]any {
	params := []map[string]any{}
	if includeProject {
		params = append(params, queryParam("projectId", "string"))
	}
	params = append(params,
		queryParam("action", "string"),
		queryParam("resourceType", "string"),
		queryParam("resourceId", "string"),
		queryParam("actor", "string"),
		queryParam("source", "string"),
		queryParam("requestId", "string"),
		queryParam("operation", "string"),
		queryParam("since", "string"),
		queryParam("until", "string"),
		queryParam("limit", "integer"),
	)
	return params
}

func openAPIComponents() map[string]any {
	return map[string]any{
		"securitySchemes": map[string]any{
			"bearerAuth": map[string]any{
				"type":        "http",
				"scheme":      "bearer",
				"description": "Shared automation token configured with MBOX_API_TOKEN.",
			},
		},
		"schemas": map[string]any{
			"Health":                        objectSchema(requiredProps("status"), prop("status", stringSchema())),
			"Error":                         objectSchema(requiredProps("error"), prop("error", stringSchema())),
			"APIInfo":                       objectSchema(requiredProps("name", "apiVersion", "serverVersion", "runtimeController", "runtimeAccess", "artifactContent", "capabilities", "compatibility", "authenticationRequired"), prop("name", stringSchema()), prop("apiVersion", stringSchema()), prop("serverVersion", stringSchema()), prop("runtimeController", schemaRef("RuntimeInfo")), prop("runtimeAccess", schemaRef("RuntimeInfo")), prop("artifactContent", schemaRef("ArtifactInfo")), prop("capabilities", arraySchema(stringSchema())), prop("compatibility", schemaRef("Compatibility")), prop("authenticationRequired", boolSchema())),
			"RuntimeInfo":                   objectSchema(requiredProps("enabled"), prop("enabled", boolSchema()), prop("adapter", stringSchema())),
			"ArtifactInfo":                  objectSchema(requiredProps("retainedContentEnabled", "storageProvider", "maxBytes"), prop("retainedContentEnabled", boolSchema()), prop("storageProvider", stringSchema()), prop("maxBytes", integerSchema())),
			"Compatibility":                 objectSchema(requiredProps("minimumCliApiVersion", "minimumSdkApiVersion"), prop("minimumCliApiVersion", stringSchema()), prop("minimumSdkApiVersion", stringSchema())),
			"Project":                       projectSchema(false),
			"ProjectCreate":                 projectSchema(true),
			"ProjectUpdate":                 projectUpdateSchema(),
			"ProjectPolicy":                 projectPolicySchema(false),
			"ProjectPolicyUpsert":           projectPolicySchema(true),
			"ProjectQuotaPolicy":            projectQuotaPolicySchema(false),
			"ProjectQuotaPolicyUpsert":      projectQuotaPolicySchema(true),
			"ProjectCredential":             projectCredentialSchema(false),
			"ProjectCredentialCreate":       projectCredentialSchema(true),
			"ProjectUsage":                  projectUsageSchema(),
			"ProjectSandboxUsage":           projectSandboxUsageSchema(),
			"SandboxResourceRequestUsage":   sandboxResourceRequestUsageSchema(),
			"ResourceQuantityUsage":         resourceQuantityUsageSchema(),
			"ProjectSessionUsage":           projectSessionUsageSchema(),
			"ProjectTaskUsage":              projectTaskUsageSchema(),
			"ProjectArtifactUsage":          projectArtifactUsageSchema(),
			"ProjectTemplateUsage":          projectTemplateUsageSchema(),
			"ResourceUsageValue":            resourceUsageValueSchema(),
			"ProjectCredentialUsage":        projectCredentialUsageSchema(),
			"AuditEvent":                    auditEventSchema(),
			"AuditEventAction":              auditEventActionSchema(),
			"AuditEventMetadata":            auditEventMetadataSchema(),
			"PolicyDeniedAuditMetadata":     policyDeniedAuditMetadataSchema(),
			"EnvironmentTemplate":           templateSchema(false, false),
			"TemplateCreate":                templateSchema(true, false),
			"TemplateUpdate":                templateSchema(false, true),
			"TemplatePort":                  objectSchema(requiredProps("name", "port", "protocol"), prop("name", stringSchema()), prop("port", integerSchema()), prop("protocol", stringSchema())),
			"SecretRef":                     objectSchema(requiredProps("name"), prop("name", stringSchema()), prop("key", stringSchema())),
			"TemplateValidationRunCreate":   objectSchema(nil, prop("projectId", stringSchema()), prop("name", stringSchema()), prop("metadata", objectAnySchema())),
			"TemplateValidationRunDecision": objectSchema(requiredProps("status"), prop("status", enumSchema("passed", "failed"))),
			"TemplateValidationRun":         objectSchema(requiredProps("template", "sandbox"), prop("template", schemaRef("EnvironmentTemplate")), prop("sandbox", schemaRef("Sandbox"))),
			"BoundarySummary":               looseObjectSchema("Read-only runtime boundary summary."),
			"Sandbox":                       sandboxSchema(false),
			"SandboxCreate":                 sandboxSchema(true),
			"SandboxUpdate":                 sandboxUpdateSchema(),
			"SandboxPort":                   objectSchema(requiredProps("name", "port", "protocol"), prop("name", stringSchema()), prop("port", integerSchema()), prop("protocol", stringSchema()), prop("previewUrl", stringSchema())),
			"RuntimeRef":                    objectSchema(requiredProps("kind", "namespace", "name"), prop("adapter", stringSchema()), prop("kind", stringSchema()), prop("namespace", stringSchema()), prop("name", stringSchema())),
			"RuntimeTarget":                 looseObjectSchema("Resolved runtime Pod target."),
			"LogResult":                     objectSchema(requiredProps("target", "logs"), prop("target", schemaRef("RuntimeTarget")), prop("logs", stringSchema())),
			"RuntimeEvent":                  looseObjectSchema("Kubernetes event summary."),
			"PreviewPortsResult":            objectSchema(requiredProps("target", "items"), prop("target", schemaRef("RuntimeTarget")), prop("items", arraySchema(schemaRef("PreviewPort")))),
			"PreviewPort":                   objectSchema(requiredProps("name", "port", "protocol", "available"), prop("name", stringSchema()), prop("port", integerSchema()), prop("protocol", stringSchema()), prop("previewUrl", stringSchema()), prop("available", boolSchema()), prop("message", stringSchema())),
			"RuntimeSession":                runtimeSessionSchema(false),
			"RuntimeSessionCreate":          runtimeSessionSchema(true),
			"ExecutionTask":                 executionTaskSchema(false),
			"ExecutionTaskCreate":           executionTaskSchema(true),
			"ExecutionTaskEvent":            looseObjectSchema("Task watch event. Each line is one JSON object."),
			"Artifact":                      artifactSchema(false),
			"ArtifactCreate":                artifactSchema(true),
			"ArtifactContent":               artifactContentSchema(),
			"RuntimeResourceList":           runtimeResourceListSchema(),
			"RuntimeResourceSummary":        runtimeResourceSummarySchema(),
			"RuntimeResourceCount":          objectSchema(requiredProps("name", "count"), prop("name", stringSchema()), prop("count", integerSchema())),
			"RuntimeResource":               runtimeResourceSchema(),
			"RuntimeResourceOwner":          runtimeResourceOwnerSchema(),
			"RuntimeOrphanAudit":            runtimeOrphanAuditSchema(),
			"RuntimeOrphan":                 runtimeOrphanSchema(),
			"RuntimeOrphanReason":           enumSchema("missing-sandbox-record", "cleanup-pending", "runtime-ref-mismatch", "missing-template-record", "unlabeled-owner"),
			"RuntimeOrphanCleanupRequest":   runtimeOrphanCleanupRequestSchema(),
			"RuntimeOrphanCleanupResult":    runtimeOrphanCleanupResultSchema(),
			"ManagedResourceRef":            managedResourceRefSchema(),
		},
	}
}

type schemaProp struct {
	name   string
	schema any
}

func prop(name string, schema any) schemaProp {
	return schemaProp{name: name, schema: schema}
}

func objectSchema(required []string, props ...schemaProp) map[string]any {
	properties := map[string]any{}
	for _, item := range props {
		properties[item.name] = item.schema
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func looseObjectSchema(description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          description,
		"additionalProperties": true,
	}
}

func objectAnySchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
	}
}

func requiredProps(names ...string) []string {
	return names
}

func stringSchema() map[string]any {
	return map[string]any{"type": "string"}
}

func boolSchema() map[string]any {
	return map[string]any{"type": "boolean"}
}

func integerSchema() map[string]any {
	return map[string]any{"type": "integer"}
}

func numberSchema() map[string]any {
	return map[string]any{"type": "number"}
}

func arraySchema(items any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func enumSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

func dateTimeSchema() map[string]any {
	return map[string]any{"type": "string", "format": "date-time"}
}

func nullable(schema map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range schema {
		out[key] = value
	}
	out["nullable"] = true
	return out
}

func projectSchema(create bool) map[string]any {
	required := requiredProps("name", "defaultNamespace")
	props := []schemaProp{
		prop("name", stringSchema()),
		prop("slug", stringSchema()),
		prop("repositoryUrl", stringSchema()),
		prop("defaultNamespace", stringSchema()),
		prop("defaultTemplateId", stringSchema()),
		prop("metadata", objectAnySchema()),
	}
	if !create {
		required = requiredProps("id", "name", "slug", "defaultNamespace")
		props = append([]schemaProp{prop("id", stringSchema())}, props...)
		props = append(props, prop("createdAt", dateTimeSchema()), prop("updatedAt", dateTimeSchema()))
	}
	return objectSchema(required, props...)
}

func projectUpdateSchema() map[string]any {
	return objectSchema(nil,
		prop("name", stringSchema()),
		prop("repositoryUrl", stringSchema()),
		prop("defaultNamespace", stringSchema()),
		prop("defaultTemplateId", nullable(stringSchema())),
		prop("metadata", objectAnySchema()),
	)
}

func projectPolicySchema(upsert bool) map[string]any {
	props := []schemaProp{
		prop("enforcement", enumSchema("disabled", "enforced")),
		prop("allowedImagePrefixes", arraySchema(stringSchema())),
		prop("allowedServiceAccounts", arraySchema(stringSchema())),
		prop("allowedSecretRefs", arraySchema(stringSchema())),
	}
	if !upsert {
		props = append([]schemaProp{prop("projectId", stringSchema())}, props...)
		props = append(props, prop("createdAt", dateTimeSchema()), prop("updatedAt", dateTimeSchema()))
	}
	return objectSchema(requiredProps("enforcement"), props...)
}

func projectQuotaPolicySchema(upsert bool) map[string]any {
	props := []schemaProp{
		prop("enforcement", enumSchema("disabled", "enforced")),
		prop("maxActiveSandboxes", integerSchema()),
		prop("maxRetainedArtifactBytes", integerSchema()),
	}
	if !upsert {
		props = append([]schemaProp{prop("projectId", stringSchema())}, props...)
		props = append(props, prop("createdAt", dateTimeSchema()), prop("updatedAt", dateTimeSchema()))
	}
	return objectSchema(requiredProps("enforcement"), props...)
}

func projectCredentialSchema(create bool) map[string]any {
	required := requiredProps("name", "type", "secretRef")
	props := []schemaProp{
		prop("name", stringSchema()),
		prop("slug", stringSchema()),
		prop("type", enumSchema("git", "registry", "kubernetes", "ssh", "generic")),
		prop("target", stringSchema()),
		prop("secretRef", schemaRef("SecretRef")),
		prop("usage", arraySchema(stringSchema())),
		prop("metadata", objectAnySchema()),
	}
	if !create {
		required = requiredProps("id", "projectId", "name", "slug", "type", "secretRef")
		props = append([]schemaProp{prop("id", stringSchema()), prop("projectId", stringSchema())}, props...)
		props = append(props, prop("createdAt", dateTimeSchema()), prop("updatedAt", dateTimeSchema()))
	}
	return objectSchema(required, props...)
}

func projectUsageSchema() map[string]any {
	schema := objectSchema(requiredProps("projectId", "generatedAt", "sandboxes", "runtimeSessions", "executionTasks", "artifacts", "templates", "credentials"),
		prop("projectId", stringSchema()),
		prop("generatedAt", dateTimeSchema()),
		prop("sandboxes", schemaRef("ProjectSandboxUsage")),
		prop("runtimeSessions", schemaRef("ProjectSessionUsage")),
		prop("executionTasks", schemaRef("ProjectTaskUsage")),
		prop("artifacts", schemaRef("ProjectArtifactUsage")),
		prop("templates", schemaRef("ProjectTemplateUsage")),
		prop("credentials", schemaRef("ProjectCredentialUsage")),
		prop("notes", arraySchema(stringSchema())),
	)
	schema["description"] = "Read-only product-record usage summary. Sandbox request totals are declared template requests from product records, not live Kubernetes metrics."
	return schema
}

func projectSandboxUsageSchema() map[string]any {
	return objectSchema(requiredProps("total", "active", "pending", "running", "stopped", "failed", "deleted", "cleanupPending", "activeRequests", "runningRequests"),
		prop("total", integerSchema()),
		prop("active", integerSchema()),
		prop("pending", integerSchema()),
		prop("running", integerSchema()),
		prop("stopped", integerSchema()),
		prop("failed", integerSchema()),
		prop("deleted", integerSchema()),
		prop("cleanupPending", integerSchema()),
		prop("activeRequests", schemaRef("SandboxResourceRequestUsage")),
		prop("runningRequests", schemaRef("SandboxResourceRequestUsage")),
	)
}

func sandboxResourceRequestUsageSchema() map[string]any {
	schema := objectSchema(requiredProps("count", "cpu", "memory", "storage"),
		prop("count", integerSchema()),
		prop("cpu", schemaRef("ResourceQuantityUsage")),
		prop("memory", schemaRef("ResourceQuantityUsage")),
		prop("storage", schemaRef("ResourceQuantityUsage")),
	)
	schema["description"] = "Declared resource requests summed from sandbox template product records."
	return schema
}

func resourceQuantityUsageSchema() map[string]any {
	return objectSchema(requiredProps("declared", "missing", "invalid"),
		prop("total", stringSchema()),
		prop("declared", integerSchema()),
		prop("missing", integerSchema()),
		prop("invalid", integerSchema()),
	)
}

func projectSessionUsageSchema() map[string]any {
	return objectSchema(requiredProps("total", "active", "ended", "failed", "terminal", "ide", "notebook", "browser", "command", "custom"),
		prop("total", integerSchema()),
		prop("active", integerSchema()),
		prop("ended", integerSchema()),
		prop("failed", integerSchema()),
		prop("terminal", integerSchema()),
		prop("ide", integerSchema()),
		prop("notebook", integerSchema()),
		prop("browser", integerSchema()),
		prop("command", integerSchema()),
		prop("custom", integerSchema()),
	)
}

func projectTaskUsageSchema() map[string]any {
	return objectSchema(requiredProps("total", "queued", "running", "succeeded", "failed", "canceled", "timedOut"),
		prop("total", integerSchema()),
		prop("queued", integerSchema()),
		prop("running", integerSchema()),
		prop("succeeded", integerSchema()),
		prop("failed", integerSchema()),
		prop("canceled", integerSchema()),
		prop("timedOut", integerSchema()),
	)
}

func projectArtifactUsageSchema() map[string]any {
	return objectSchema(requiredProps("total", "retainedContent", "referencedBytes", "retainedBytes", "file", "directory", "log", "report", "screenshot", "image", "link", "other"),
		prop("total", integerSchema()),
		prop("retainedContent", integerSchema()),
		prop("referencedBytes", integerSchema()),
		prop("retainedBytes", integerSchema()),
		prop("file", integerSchema()),
		prop("directory", integerSchema()),
		prop("log", integerSchema()),
		prop("report", integerSchema()),
		prop("screenshot", integerSchema()),
		prop("image", integerSchema()),
		prop("link", integerSchema()),
		prop("other", integerSchema()),
	)
}

func projectTemplateUsageSchema() map[string]any {
	return objectSchema(requiredProps("projectScoped", "globalVisible"),
		prop("projectScoped", integerSchema()),
		prop("globalVisible", integerSchema()),
		prop("cpuRequests", arraySchema(schemaRef("ResourceUsageValue"))),
		prop("memoryRequests", arraySchema(schemaRef("ResourceUsageValue"))),
		prop("storageRequests", arraySchema(schemaRef("ResourceUsageValue"))),
	)
}

func resourceUsageValueSchema() map[string]any {
	return objectSchema(requiredProps("value", "count"),
		prop("value", stringSchema()),
		prop("count", integerSchema()),
	)
}

func projectCredentialUsageSchema() map[string]any {
	return objectSchema(requiredProps("total", "git", "registry", "kubernetes", "ssh", "generic"),
		prop("total", integerSchema()),
		prop("git", integerSchema()),
		prop("registry", integerSchema()),
		prop("kubernetes", integerSchema()),
		prop("ssh", integerSchema()),
		prop("generic", integerSchema()),
	)
}

func runtimeResourceListSchema() map[string]any {
	schema := objectSchema(requiredProps("adapter", "checkedAt", "summary", "items"),
		prop("adapter", stringSchema()),
		prop("checkedAt", dateTimeSchema()),
		prop("summary", schemaRef("RuntimeResourceSummary")),
		prop("items", arraySchema(schemaRef("RuntimeResource"))),
	)
	schema["description"] = "Read-only inventory of mbox-managed runtime resources reported by the runtime auditor."
	return schema
}

func runtimeResourceSummarySchema() map[string]any {
	return objectSchema(requiredProps("total", "byKind", "byNamespace", "byOwner"),
		prop("total", integerSchema()),
		prop("byKind", arraySchema(schemaRef("RuntimeResourceCount"))),
		prop("byNamespace", arraySchema(schemaRef("RuntimeResourceCount"))),
		prop("byOwner", arraySchema(schemaRef("RuntimeResourceCount"))),
	)
}

func runtimeResourceSchema() map[string]any {
	return objectSchema(requiredProps("adapter", "kind", "name"),
		prop("adapter", stringSchema()),
		prop("kind", stringSchema()),
		prop("namespace", stringSchema()),
		prop("name", stringSchema()),
		prop("owner", schemaRef("RuntimeResourceOwner")),
		prop("labels", objectAnySchema()),
		prop("createdAt", dateTimeSchema()),
	)
}

func runtimeResourceOwnerSchema() map[string]any {
	return objectSchema(requiredProps("kind"),
		prop("kind", enumSchema("sandbox", "template")),
		prop("projectId", stringSchema()),
		prop("sandboxId", stringSchema()),
		prop("templateId", stringSchema()),
	)
}

func runtimeOrphanAuditSchema() map[string]any {
	schema := objectSchema(requiredProps("adapter", "checkedAt", "resourceCount", "orphanCount", "expectedClean", "items"),
		prop("adapter", stringSchema()),
		prop("checkedAt", dateTimeSchema()),
		prop("namespace", stringSchema()),
		prop("resourceCount", integerSchema()),
		prop("orphanCount", integerSchema()),
		prop("expectedClean", boolSchema()),
		prop("items", arraySchema(schemaRef("RuntimeOrphan"))),
	)
	schema["description"] = "Read-only runtime orphan audit result comparing managed runtime resources with product records."
	return schema
}

func runtimeOrphanSchema() map[string]any {
	return objectSchema(requiredProps("reason", "resource", "message"),
		prop("reason", schemaRef("RuntimeOrphanReason")),
		prop("resource", schemaRef("RuntimeResource")),
		prop("sandboxId", stringSchema()),
		prop("templateId", stringSchema()),
		prop("projectId", stringSchema()),
		prop("runtimeRef", schemaRef("RuntimeRef")),
		prop("status", enumSchema("pending", "running", "stopped", "failed", "deleted")),
		prop("deletedAt", dateTimeSchema()),
		prop("message", stringSchema()),
		prop("evidence", arraySchema(stringSchema())),
	)
}

func managedResourceRefSchema() map[string]any {
	return objectSchema(requiredProps("adapter", "kind", "namespace", "name"),
		prop("adapter", stringSchema()),
		prop("kind", stringSchema()),
		prop("namespace", stringSchema()),
		prop("name", stringSchema()),
	)
}

func runtimeOrphanCleanupRequestSchema() map[string]any {
	return objectSchema(requiredProps("resource", "reason", "confirm", "deleteOrphan"),
		prop("resource", schemaRef("ManagedResourceRef")),
		prop("reason", schemaRef("RuntimeOrphanReason")),
		prop("confirm", enumSchema("delete-orphan-runtime-resource")),
		prop("deleteOrphan", boolSchema()),
	)
}

func runtimeOrphanCleanupResultSchema() map[string]any {
	return objectSchema(requiredProps("deleted", "resource", "reason", "message"),
		prop("deleted", boolSchema()),
		prop("resource", schemaRef("ManagedResourceRef")),
		prop("reason", schemaRef("RuntimeOrphanReason")),
		prop("message", stringSchema()),
	)
}

func auditEventSchema() map[string]any {
	schema := objectSchema(requiredProps("id", "action", "resourceType", "createdAt"),
		prop("id", stringSchema()),
		prop("projectId", stringSchema()),
		prop("action", schemaRef("AuditEventAction")),
		prop("resourceType", stringSchema()),
		prop("resourceId", stringSchema()),
		prop("resourceName", stringSchema()),
		prop("actor", stringSchema()),
		prop("source", stringSchema()),
		prop("metadata", schemaRef("AuditEventMetadata")),
		prop("createdAt", dateTimeSchema()),
	)
	schema["description"] = "Best-effort product audit event. Successful API mutations and selected policy/quota denials are recorded for operator visibility; events are not a strong transactional audit log or authentication identity proof."
	return schema
}

func auditEventActionSchema() map[string]any {
	schema := enumSchema(knownAuditActions...)
	schema["description"] = "Known mbox product audit action values. Clients should tolerate additional action strings in future API versions. Current values: " + strings.Join(knownAuditActions, ", ") + "."
	return schema
}

func auditEventMetadataSchema() map[string]any {
	return map[string]any{
		"anyOf": []map[string]any{
			schemaRef("PolicyDeniedAuditMetadata"),
			objectAnySchema(),
		},
		"description": "Action-specific metadata. For action=policy.denied, metadata follows PolicyDeniedAuditMetadata.",
	}
}

func policyDeniedAuditMetadataSchema() map[string]any {
	schema := objectSchema(requiredProps("operation", "reason"),
		prop("operation", enumSchema(knownPolicyDeniedOperations...)),
		prop("reason", stringSchema()),
		prop("requestId", stringSchema()),
		prop("templateId", stringSchema()),
		prop("templateName", stringSchema()),
		prop("image", stringSchema()),
		prop("serviceAccountName", stringSchema()),
		prop("sandboxId", stringSchema()),
		prop("artifactKind", stringSchema()),
		prop("incomingBytes", integerSchema()),
	)
	schema["description"] = "Metadata shape for action=policy.denied. Current coverage is intentionally narrow: sandbox launch policy/quota, template validation launch policy, and retained artifact byte quota denials."
	schema["additionalProperties"] = true
	return schema
}

func templateSchema(create bool, update bool) map[string]any {
	required := []string(nil)
	if create {
		required = requiredProps("name", "image")
	}
	props := []schemaProp{
		prop("projectId", stringSchema()),
		prop("name", stringSchema()),
		prop("slug", stringSchema()),
		prop("image", stringSchema()),
		prop("startupCommand", arraySchema(stringSchema())),
		prop("workingDir", stringSchema()),
		prop("cpuRequest", stringSchema()),
		prop("memoryRequest", stringSchema()),
		prop("storageRequest", stringSchema()),
		prop("exposedPorts", arraySchema(schemaRef("TemplatePort"))),
		prop("env", objectAnySchema()),
		prop("secretRefs", arraySchema(schemaRef("SecretRef"))),
		prop("networkPolicy", stringSchema()),
		prop("lifecyclePolicy", objectAnySchema()),
		prop("metadata", objectAnySchema()),
	}
	if !create && !update {
		required = requiredProps("id", "name", "slug", "image")
		props = append([]schemaProp{prop("id", stringSchema())}, props...)
		props = append(props, prop("createdAt", dateTimeSchema()), prop("updatedAt", dateTimeSchema()))
	}
	return objectSchema(required, props...)
}

func sandboxSchema(create bool) map[string]any {
	required := requiredProps("projectId", "name")
	props := []schemaProp{
		prop("projectId", stringSchema()),
		prop("templateId", stringSchema()),
		prop("name", stringSchema()),
		prop("slug", stringSchema()),
		prop("namespace", stringSchema()),
		prop("serviceAccountName", stringSchema()),
		prop("metadata", objectAnySchema()),
	}
	if !create {
		required = requiredProps("id", "projectId", "name", "slug", "status", "namespace", "serviceAccountName")
		props = append([]schemaProp{prop("id", stringSchema())}, props...)
		props = append(props,
			prop("status", enumSchema("pending", "running", "stopped", "failed", "deleted")),
			prop("runtimeRef", schemaRef("RuntimeRef")),
			prop("ports", arraySchema(schemaRef("SandboxPort"))),
			prop("createdAt", dateTimeSchema()),
			prop("updatedAt", dateTimeSchema()),
			prop("deletedAt", dateTimeSchema()),
		)
	}
	return objectSchema(required, props...)
}

func sandboxUpdateSchema() map[string]any {
	return objectSchema(nil,
		prop("name", stringSchema()),
		prop("status", enumSchema("pending", "running", "stopped", "failed", "deleted")),
		prop("namespace", stringSchema()),
		prop("serviceAccountName", stringSchema()),
		prop("runtimeRef", nullable(schemaRef("RuntimeRef"))),
		prop("ports", arraySchema(schemaRef("SandboxPort"))),
		prop("metadata", objectAnySchema()),
	)
}

func runtimeSessionSchema(create bool) map[string]any {
	props := []schemaProp{
		prop("type", enumSchema("terminal", "ide", "notebook", "browser", "command", "custom")),
		prop("client", stringSchema()),
		prop("metadata", objectAnySchema()),
	}
	required := requiredProps("type")
	if !create {
		required = requiredProps("id", "projectId", "sandboxId", "type", "status", "startedAt")
		props = append([]schemaProp{prop("id", stringSchema()), prop("projectId", stringSchema()), prop("sandboxId", stringSchema())}, props...)
		props = append(props,
			prop("status", enumSchema("active", "ended", "failed")),
			prop("userAgent", stringSchema()),
			prop("runtimeRef", schemaRef("RuntimeRef")),
			prop("startedAt", dateTimeSchema()),
			prop("endedAt", dateTimeSchema()),
			prop("createdAt", dateTimeSchema()),
			prop("updatedAt", dateTimeSchema()),
		)
	}
	return objectSchema(required, props...)
}

func executionTaskSchema(create bool) map[string]any {
	if create {
		return objectSchema(requiredProps("command"),
			prop("command", arraySchema(stringSchema())),
			prop("timeoutSeconds", integerSchema()),
			prop("metadata", objectAnySchema()),
		)
	}
	return objectSchema(requiredProps("id", "projectId", "sandboxId", "status", "command", "timeoutSeconds"),
		prop("id", stringSchema()),
		prop("projectId", stringSchema()),
		prop("sandboxId", stringSchema()),
		prop("status", enumSchema("queued", "running", "succeeded", "failed", "canceled", "timed_out")),
		prop("command", arraySchema(stringSchema())),
		prop("timeoutSeconds", integerSchema()),
		prop("exitCode", integerSchema()),
		prop("stdout", stringSchema()),
		prop("stderr", stringSchema()),
		prop("outputTruncated", boolSchema()),
		prop("error", stringSchema()),
		prop("runtimeRef", schemaRef("RuntimeRef")),
		prop("metadata", objectAnySchema()),
		prop("startedAt", dateTimeSchema()),
		prop("finishedAt", dateTimeSchema()),
		prop("createdAt", dateTimeSchema()),
		prop("updatedAt", dateTimeSchema()),
	)
}

func artifactSchema(create bool) map[string]any {
	required := requiredProps("kind", "name", "uri")
	props := []schemaProp{
		prop("taskId", stringSchema()),
		prop("kind", enumSchema("file", "directory", "log", "report", "screenshot", "image", "link", "other")),
		prop("name", stringSchema()),
		prop("uri", stringSchema()),
		prop("contentType", stringSchema()),
		prop("sizeBytes", numberSchema()),
		prop("metadata", objectAnySchema()),
	}
	if !create {
		required = requiredProps("id", "projectId", "sandboxId", "kind", "name", "uri")
		props = append([]schemaProp{prop("id", stringSchema()), prop("projectId", stringSchema()), prop("sandboxId", stringSchema())}, props...)
		props = append(props, prop("retainedContent", schemaRef("ArtifactContent")), prop("createdAt", dateTimeSchema()), prop("updatedAt", dateTimeSchema()))
	}
	return objectSchema(required, props...)
}

func artifactContentSchema() map[string]any {
	return objectSchema(requiredProps("artifactId", "sizeBytes", "sha256", "sourceUri", "storageProvider", "capturedAt"),
		prop("artifactId", stringSchema()),
		prop("contentType", stringSchema()),
		prop("sizeBytes", numberSchema()),
		prop("sha256", stringSchema()),
		prop("sourceUri", stringSchema()),
		prop("storageProvider", enumSchema("postgres", "filesystem", "s3")),
		prop("storageKey", stringSchema()),
		prop("capturedAt", dateTimeSchema()),
	)
}
