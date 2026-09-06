// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	protolib "google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/types"
)

// sanitizeUTF8 ensures a string contains only valid UTF-8 characters.
// Invalid UTF-8 sequences are replaced with the Unicode replacement character (U+FFFD).
// This is necessary because protobuf string fields require valid UTF-8.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	// Build a new string, replacing invalid sequences
	var builder strings.Builder
	builder.Grow(len(s))

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8 byte, replace with replacement character
			builder.WriteRune(utf8.RuneError)
			i++
		} else {
			builder.WriteRune(r)
			i += size
		}
	}

	return builder.String()
}

// ToTypedValue converts any Go value to a proto TypedValue.
// Supports primitives, arrays, maps, and JSON-serializable types.
func ToTypedValue(v any) *commonpb.TypedValue {
	if v == nil {
		return &commonpb.TypedValue{
			Kind: &commonpb.TypedValue_NullValue{
				NullValue: commonpb.NullValue_NULL_VALUE_UNSPECIFIED,
			},
		}
	}

	// Use reflection to handle various types
	val := reflect.ValueOf(v)

	// Dereference pointers
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return &commonpb.TypedValue{
				Kind: &commonpb.TypedValue_NullValue{
					NullValue: commonpb.NullValue_NULL_VALUE_UNSPECIFIED,
				},
			}
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.String:
		// Sanitize string to ensure valid UTF-8 for protobuf
		return &commonpb.TypedValue{
			Kind: &commonpb.TypedValue_StringValue{
				StringValue: sanitizeUTF8(val.String()),
			},
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &commonpb.TypedValue{
			Kind: &commonpb.TypedValue_IntValue{
				IntValue: val.Int(),
			},
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &commonpb.TypedValue{
			Kind: &commonpb.TypedValue_IntValue{
				IntValue: int64(val.Uint()),
			},
		}

	case reflect.Float32, reflect.Float64:
		return &commonpb.TypedValue{
			Kind: &commonpb.TypedValue_DoubleValue{
				DoubleValue: val.Float(),
			},
		}

	case reflect.Bool:
		return &commonpb.TypedValue{
			Kind: &commonpb.TypedValue_BoolValue{
				BoolValue: val.Bool(),
			},
		}

	case reflect.Slice, reflect.Array:
		// Handle byte slices specially
		if val.Type().Elem().Kind() == reflect.Uint8 {
			bytes := val.Bytes()
			return &commonpb.TypedValue{
				Kind: &commonpb.TypedValue_BytesValue{
					BytesValue: bytes,
				},
			}
		}

		// Convert to TypedArray
		items := make([]*commonpb.TypedValue, val.Len())
		for i := 0; i < val.Len(); i++ {
			items[i] = ToTypedValue(val.Index(i).Interface())
		}
		return &commonpb.TypedValue{
			Kind: &commonpb.TypedValue_ArrayValue{
				ArrayValue: &commonpb.TypedArray{
					Items: items,
				},
			},
		}

	case reflect.Map:
		// Convert to TypedMap
		entries := make(map[string]*commonpb.TypedValue)
		iter := val.MapRange()
		for iter.Next() {
			key := fmt.Sprintf("%v", iter.Key().Interface())
			entries[key] = ToTypedValue(iter.Value().Interface())
		}
		return &commonpb.TypedValue{
			Kind: &commonpb.TypedValue_MapValue{
				MapValue: &commonpb.TypedMap{
					Entries: entries,
				},
			},
		}

	default:
		// For structs and other complex types, serialize to JSON and store as string
		// This is a fallback for types that don't fit the TypedValue model
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			// If JSON encoding fails, return a sanitized string representation
			return &commonpb.TypedValue{
				Kind: &commonpb.TypedValue_StringValue{
					StringValue: sanitizeUTF8(fmt.Sprintf("%v", v)),
				},
			}
		}
		// JSON output is always valid UTF-8, no sanitization needed
		return &commonpb.TypedValue{
			Kind: &commonpb.TypedValue_StringValue{
				StringValue: string(jsonBytes),
			},
		}
	}
}

// FromTypedValue converts a proto TypedValue back to a Go value.
func FromTypedValue(tv *commonpb.TypedValue) any {
	if tv == nil {
		return nil
	}

	switch v := tv.Kind.(type) {
	case *commonpb.TypedValue_NullValue:
		return nil
	case *commonpb.TypedValue_StringValue:
		return v.StringValue
	case *commonpb.TypedValue_IntValue:
		return v.IntValue
	case *commonpb.TypedValue_DoubleValue:
		return v.DoubleValue
	case *commonpb.TypedValue_BoolValue:
		return v.BoolValue
	case *commonpb.TypedValue_BytesValue:
		return v.BytesValue
	case *commonpb.TypedValue_ArrayValue:
		if v.ArrayValue == nil {
			return []any{}
		}
		result := make([]any, len(v.ArrayValue.Items))
		for i, item := range v.ArrayValue.Items {
			result[i] = FromTypedValue(item)
		}
		return result
	case *commonpb.TypedValue_MapValue:
		if v.MapValue == nil {
			return map[string]any{}
		}
		result := make(map[string]any)
		for key, val := range v.MapValue.Entries {
			result[key] = FromTypedValue(val)
		}
		return result
	default:
		return nil
	}
}

// ToTypedMap converts map[string]any to map[string]*TypedValue.
func ToTypedMap(m map[string]any) map[string]*commonpb.TypedValue {
	if m == nil {
		return nil
	}

	result := make(map[string]*commonpb.TypedValue)
	for k, v := range m {
		result[k] = ToTypedValue(v)
	}

	return result
}

// FromTypedMap converts map[string]*TypedValue to map[string]any.
func FromTypedMap(m map[string]*commonpb.TypedValue) map[string]any {
	if m == nil {
		return make(map[string]any)
	}

	result := make(map[string]any)
	for k, v := range m {
		result[k] = FromTypedValue(v)
	}

	return result
}

// FindingToProto converts an SDK finding to a proto Finding.
func FindingToProto(f *finding.Finding) *typespb.Finding {
	if f == nil {
		return nil
	}

	protoFinding := &typespb.Finding{
		Id:            f.ID,
		MissionId:     f.MissionID,
		AgentName:     f.AgentName,
		DelegatedFrom: f.DelegatedFrom,
		Title:         f.Title,
		Description:   f.Description,
		Category:      string(f.Category),
		Subcategory:   f.Subcategory,
		Severity:      severityToProto(f.Severity),
		Confidence:    f.Confidence,
		Status:        statusToProto(f.Status),
		Remediation:   f.Remediation,
		References:    f.References,
		TargetId:      f.TargetID,
		Technique:     f.Technique,
		Tags:          f.Tags,
		CreatedAt:     f.CreatedAt.UnixMilli(),
		UpdatedAt:     f.UpdatedAt.UnixMilli(),
	}

	// Convert CVSS score
	if f.CVSSScore != nil {
		protoFinding.CvssScore = *f.CVSSScore
	}

	// Risk score
	protoFinding.RiskScore = f.RiskScore

	// Convert MITRE mappings
	if f.MitreAttack != nil {
		protoFinding.MitreAttack = mitreToProto(f.MitreAttack)
	}
	if f.MitreAtlas != nil {
		protoFinding.MitreAtlas = mitreToProto(f.MitreAtlas)
	}

	// Convert evidence
	protoFinding.Evidence = make([]*typespb.Evidence, len(f.Evidence))
	for i, e := range f.Evidence {
		// Convert metadata map[string]any to map[string]string
		metadata := make(map[string]string)
		for k, v := range e.Metadata {
			metadata[k] = fmt.Sprintf("%v", v)
		}
		protoFinding.Evidence[i] = &typespb.Evidence{
			Title:    e.Title,
			Type:     evidenceTypeToProto(e.Type),
			Content:  e.Content,
			Metadata: metadata,
		}
	}

	// Convert reproduction steps
	protoFinding.Reproduction = make([]*typespb.ReproStep, len(f.Reproduction))
	for i, r := range f.Reproduction {
		protoFinding.Reproduction[i] = &typespb.ReproStep{
			Order:       int32(r.Order),
			Description: r.Description,
			Input:       r.Input,
			Output:      r.Output,
		}
	}

	return protoFinding
}

// FindingFromProto converts a proto Finding to an SDK finding.
func FindingFromProto(pf *typespb.Finding) *finding.Finding {
	if pf == nil {
		return nil
	}

	f := &finding.Finding{
		ID:            pf.Id,
		MissionID:     pf.MissionId,
		AgentName:     pf.AgentName,
		DelegatedFrom: pf.DelegatedFrom,
		Title:         pf.Title,
		Description:   pf.Description,
		Category:      pf.Category,
		Subcategory:   pf.Subcategory,
		Severity:      severityFromProto(pf.Severity),
		Confidence:    pf.Confidence,
		Status:        statusFromProto(pf.Status),
		RiskScore:     pf.RiskScore,
		Remediation:   pf.Remediation,
		References:    pf.References,
		TargetID:      pf.TargetId,
		Technique:     pf.Technique,
		Tags:          pf.Tags,
	}

	// Convert CVSS score
	if pf.CvssScore != 0 {
		score := pf.CvssScore
		f.CVSSScore = &score
	}

	// Convert timestamps
	if pf.CreatedAt != 0 {
		f.CreatedAt = time.UnixMilli(pf.CreatedAt)
	}
	if pf.UpdatedAt != 0 {
		f.UpdatedAt = time.UnixMilli(pf.UpdatedAt)
	}

	// Convert MITRE mappings
	if pf.MitreAttack != nil {
		f.MitreAttack = mitreFromProto(pf.MitreAttack)
	}
	if pf.MitreAtlas != nil {
		f.MitreAtlas = mitreFromProto(pf.MitreAtlas)
	}

	// Convert evidence
	f.Evidence = make([]finding.Evidence, len(pf.Evidence))
	for i, e := range pf.Evidence {
		// Convert map[string]string to map[string]any
		metadata := make(map[string]any)
		for k, v := range e.Metadata {
			metadata[k] = v
		}
		f.Evidence[i] = finding.Evidence{
			Title:    e.Title,
			Type:     evidenceTypeFromProto(e.Type),
			Content:  e.Content,
			Metadata: metadata,
		}
	}

	// Convert reproduction steps
	f.Reproduction = make([]finding.ReproStep, len(pf.Reproduction))
	for i, r := range pf.Reproduction {
		f.Reproduction[i] = finding.ReproStep{
			Order:       int(r.Order),
			Description: r.Description,
			Input:       r.Input,
			Output:      r.Output,
		}
	}

	return f
}

// Helper functions for enum conversions

func severityToProto(s finding.Severity) typespb.FindingSeverity {
	switch s {
	case finding.SeverityCritical:
		return typespb.FindingSeverity_FINDING_SEVERITY_CRITICAL
	case finding.SeverityHigh:
		return typespb.FindingSeverity_FINDING_SEVERITY_HIGH
	case finding.SeverityMedium:
		return typespb.FindingSeverity_FINDING_SEVERITY_MEDIUM
	case finding.SeverityLow:
		return typespb.FindingSeverity_FINDING_SEVERITY_LOW
	case finding.SeverityInfo:
		return typespb.FindingSeverity_FINDING_SEVERITY_INFO
	default:
		return typespb.FindingSeverity_FINDING_SEVERITY_UNSPECIFIED
	}
}

func severityFromProto(s typespb.FindingSeverity) finding.Severity {
	switch s {
	case typespb.FindingSeverity_FINDING_SEVERITY_CRITICAL:
		return finding.SeverityCritical
	case typespb.FindingSeverity_FINDING_SEVERITY_HIGH:
		return finding.SeverityHigh
	case typespb.FindingSeverity_FINDING_SEVERITY_MEDIUM:
		return finding.SeverityMedium
	case typespb.FindingSeverity_FINDING_SEVERITY_LOW:
		return finding.SeverityLow
	case typespb.FindingSeverity_FINDING_SEVERITY_INFO:
		return finding.SeverityInfo
	default:
		return finding.SeverityInfo
	}
}

func statusToProto(s finding.Status) typespb.FindingStatus {
	switch s {
	case finding.StatusOpen:
		return typespb.FindingStatus_FINDING_STATUS_OPEN
	case finding.StatusConfirmed:
		return typespb.FindingStatus_FINDING_STATUS_CONFIRMED
	case finding.StatusResolved:
		return typespb.FindingStatus_FINDING_STATUS_CLOSED
	case finding.StatusFalsePositive:
		return typespb.FindingStatus_FINDING_STATUS_FALSE_POSITIVE
	default:
		return typespb.FindingStatus_FINDING_STATUS_UNSPECIFIED
	}
}

func statusFromProto(s typespb.FindingStatus) finding.Status {
	switch s {
	case typespb.FindingStatus_FINDING_STATUS_OPEN:
		return finding.StatusOpen
	case typespb.FindingStatus_FINDING_STATUS_CONFIRMED:
		return finding.StatusConfirmed
	case typespb.FindingStatus_FINDING_STATUS_CLOSED:
		return finding.StatusResolved
	case typespb.FindingStatus_FINDING_STATUS_FALSE_POSITIVE:
		return finding.StatusFalsePositive
	default:
		return finding.StatusOpen
	}
}

func evidenceTypeToProto(t finding.EvidenceType) typespb.EvidenceType {
	switch t {
	case finding.EvidenceHTTPRequest:
		return typespb.EvidenceType_EVIDENCE_TYPE_REQUEST
	case finding.EvidenceHTTPResponse:
		return typespb.EvidenceType_EVIDENCE_TYPE_RESPONSE
	case finding.EvidenceScreenshot:
		return typespb.EvidenceType_EVIDENCE_TYPE_SCREENSHOT
	case finding.EvidenceLog:
		return typespb.EvidenceType_EVIDENCE_TYPE_LOG
	case finding.EvidencePayload:
		return typespb.EvidenceType_EVIDENCE_TYPE_OTHER
	case finding.EvidenceConversation:
		return typespb.EvidenceType_EVIDENCE_TYPE_OTHER
	default:
		return typespb.EvidenceType_EVIDENCE_TYPE_UNSPECIFIED
	}
}

func evidenceTypeFromProto(t typespb.EvidenceType) finding.EvidenceType {
	switch t {
	case typespb.EvidenceType_EVIDENCE_TYPE_REQUEST:
		return finding.EvidenceHTTPRequest
	case typespb.EvidenceType_EVIDENCE_TYPE_RESPONSE:
		return finding.EvidenceHTTPResponse
	case typespb.EvidenceType_EVIDENCE_TYPE_SCREENSHOT:
		return finding.EvidenceScreenshot
	case typespb.EvidenceType_EVIDENCE_TYPE_LOG:
		return finding.EvidenceLog
	case typespb.EvidenceType_EVIDENCE_TYPE_OTHER:
		return finding.EvidencePayload
	default:
		return finding.EvidencePayload
	}
}

func mitreToProto(m *finding.MitreMapping) *typespb.MitreMapping {
	if m == nil {
		return nil
	}

	return &typespb.MitreMapping{
		Matrix:        m.Matrix,
		TacticId:      m.TacticID,
		TacticName:    m.TacticName,
		TechniqueId:   m.TechniqueID,
		TechniqueName: m.TechniqueName,
		SubTechniques: m.SubTechniques,
	}
}

func mitreFromProto(m *typespb.MitreMapping) *finding.MitreMapping {
	if m == nil {
		return nil
	}

	return &finding.MitreMapping{
		Matrix:        m.Matrix,
		TacticID:      m.TacticId,
		TacticName:    m.TacticName,
		TechniqueID:   m.TechniqueId,
		TechniqueName: m.TechniqueName,
		SubTechniques: m.SubTechniques,
	}
}

// ProtoToTask converts a proto Task to SDK agent.Task.
func ProtoToTask(pt *typespb.Task) agent.Task {
	if pt == nil {
		return agent.Task{}
	}

	return agent.Task{
		ID:       pt.GetId(),
		Goal:     pt.GetGoal(),
		Context:  FromTypedMap(pt.GetContext()),
		Metadata: FromTypedMap(pt.GetMetadata()),
		Constraints: agent.TaskConstraints{
			MaxTurns:     int(pt.GetConstraints().GetMaxTurns()),
			MaxTokens:    int(pt.GetConstraints().GetMaxTokens()),
			AllowedTools: pt.GetConstraints().GetAllowedTools(),
			BlockedTools: pt.GetConstraints().GetBlockedTools(),
		},
		Job: cloneJobSpec(pt.GetJob()),
	}
}

// cloneJobSpec copies a JobSpec so the agent-side Task and the wire message
// never share mutable state. A nil spec stays nil.
func cloneJobSpec(spec *jobpb.JobSpec) *jobpb.JobSpec {
	if spec == nil {
		return nil
	}
	return protolib.Clone(spec).(*jobpb.JobSpec)
}

// TaskToProto converts SDK agent.Task to proto Task.
func TaskToProto(t agent.Task) *typespb.Task {
	return &typespb.Task{
		Id:       t.ID,
		Goal:     t.Goal,
		Context:  ToTypedMap(t.Context),
		Metadata: ToTypedMap(t.Metadata),
		Constraints: &typespb.TaskConstraints{
			MaxTurns:     int32(t.Constraints.MaxTurns),
			MaxTokens:    int32(t.Constraints.MaxTokens),
			AllowedTools: t.Constraints.AllowedTools,
			BlockedTools: t.Constraints.BlockedTools,
		},
		Job: cloneJobSpec(t.Job),
	}
}

// ProtoToResult converts a proto Result to SDK agent.Result.
func ProtoToResult(pr *typespb.Result) agent.Result {
	if pr == nil {
		return agent.Result{}
	}

	result := agent.Result{
		Status:   ProtoToResultStatus(pr.GetStatus()),
		Output:   FromTypedValue(pr.GetOutput()),
		Findings: pr.GetFindingIds(),
		Metadata: FromTypedMap(pr.GetMetadata()),
	}

	// Convert error if present
	if pr.GetError() != nil {
		// Convert map[string]string to map[string]any
		details := make(map[string]any)
		for k, v := range pr.GetError().GetDetails() {
			details[k] = v
		}
		result.ErrorInfo = &agent.ResultError{
			Code:      ProtoErrorCodeToString(pr.GetError().GetCode()),
			Message:   pr.GetError().GetMessage(),
			Details:   details,
			Retryable: pr.GetError().GetRetryable(),
			Component: "", // Not in proto
		}
	}

	return result
}

// ResultToProto converts SDK agent.Result to proto Result.
func ResultToProto(r agent.Result) *typespb.Result {
	result := &typespb.Result{
		Status:     ResultStatusToProto(r.Status),
		Output:     ToTypedValue(r.Output),
		FindingIds: r.Findings,
		Metadata:   ToTypedMap(r.Metadata),
	}

	// Convert ErrorInfo if present
	if r.ErrorInfo != nil {
		// Convert map[string]any to map[string]string
		details := make(map[string]string)
		for k, v := range r.ErrorInfo.Details {
			details[k] = fmt.Sprintf("%v", v)
		}
		result.Error = &typespb.ResultError{
			Code:      StringToProtoErrorCode(r.ErrorInfo.Code),
			Message:   r.ErrorInfo.Message,
			Details:   details,
			Retryable: r.ErrorInfo.Retryable,
		}
	}

	return result
}

// ProtoToResultStatus converts proto ResultStatus to SDK ResultStatus.
func ProtoToResultStatus(ps typespb.ResultStatus) agent.ResultStatus {
	switch ps {
	case typespb.ResultStatus_RESULT_STATUS_SUCCESS:
		return agent.StatusSuccess
	case typespb.ResultStatus_RESULT_STATUS_FAILED:
		return agent.StatusFailed
	case typespb.ResultStatus_RESULT_STATUS_PARTIAL:
		return agent.StatusPartial
	case typespb.ResultStatus_RESULT_STATUS_CANCELLED:
		return agent.StatusCancelled
	case typespb.ResultStatus_RESULT_STATUS_TIMEOUT:
		return agent.StatusTimeout
	default:
		return agent.StatusFailed
	}
}

// ResultStatusToProto converts SDK ResultStatus to proto ResultStatus.
func ResultStatusToProto(s agent.ResultStatus) typespb.ResultStatus {
	switch s {
	case agent.StatusSuccess:
		return typespb.ResultStatus_RESULT_STATUS_SUCCESS
	case agent.StatusFailed:
		return typespb.ResultStatus_RESULT_STATUS_FAILED
	case agent.StatusPartial:
		return typespb.ResultStatus_RESULT_STATUS_PARTIAL
	case agent.StatusCancelled:
		return typespb.ResultStatus_RESULT_STATUS_CANCELLED
	case agent.StatusTimeout:
		return typespb.ResultStatus_RESULT_STATUS_TIMEOUT
	default:
		return typespb.ResultStatus_RESULT_STATUS_FAILED
	}
}

// ProtoErrorCodeToString converts proto ErrorCode to string.
func ProtoErrorCodeToString(code commonpb.ErrorCode) string {
	switch code {
	case commonpb.ErrorCode_ERROR_CODE_INTERNAL:
		return agent.ErrCodeInternalError
	case commonpb.ErrorCode_ERROR_CODE_INVALID_ARGUMENT:
		return "INVALID_ARGUMENT"
	case commonpb.ErrorCode_ERROR_CODE_NOT_FOUND:
		return agent.ErrCodeToolNotFound
	case commonpb.ErrorCode_ERROR_CODE_TIMEOUT:
		return agent.ErrCodeAgentTimeout
	case commonpb.ErrorCode_ERROR_CODE_UNAVAILABLE:
		return "UNAVAILABLE"
	case commonpb.ErrorCode_ERROR_CODE_PERMISSION_DENIED:
		return "PERMISSION_DENIED"
	case commonpb.ErrorCode_ERROR_CODE_ALREADY_EXISTS:
		return "ALREADY_EXISTS"
	case commonpb.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED:
		return "RESOURCE_EXHAUSTED"
	case commonpb.ErrorCode_ERROR_CODE_CANCELLED:
		return "CANCELLED"
	case commonpb.ErrorCode_ERROR_CODE_AGENT_TIMEOUT:
		return agent.ErrCodeAgentTimeout
	case commonpb.ErrorCode_ERROR_CODE_AGENT_PANIC:
		return agent.ErrCodeAgentPanic
	case commonpb.ErrorCode_ERROR_CODE_AGENT_INIT_FAILED:
		return agent.ErrCodeAgentInitFailed
	case commonpb.ErrorCode_ERROR_CODE_LLM_RATE_LIMITED:
		return agent.ErrCodeLLMRateLimited
	case commonpb.ErrorCode_ERROR_CODE_LLM_CONTEXT_EXCEEDED:
		return agent.ErrCodeLLMContextExceeded
	case commonpb.ErrorCode_ERROR_CODE_LLM_API_ERROR:
		return agent.ErrCodeLLMAPIError
	case commonpb.ErrorCode_ERROR_CODE_LLM_PARSE_ERROR:
		return agent.ErrCodeLLMParseError
	case commonpb.ErrorCode_ERROR_CODE_TOOL_NOT_FOUND:
		return agent.ErrCodeToolNotFound
	case commonpb.ErrorCode_ERROR_CODE_TOOL_TIMEOUT:
		return agent.ErrCodeToolTimeout
	case commonpb.ErrorCode_ERROR_CODE_TOOL_EXEC_FAILED:
		return agent.ErrCodeToolExecFailed
	case commonpb.ErrorCode_ERROR_CODE_NETWORK_TIMEOUT:
		return agent.ErrCodeNetworkTimeout
	case commonpb.ErrorCode_ERROR_CODE_NETWORK_UNREACHABLE:
		return agent.ErrCodeNetworkUnreachable
	case commonpb.ErrorCode_ERROR_CODE_TLS_ERROR:
		return agent.ErrCodeTLSError
	case commonpb.ErrorCode_ERROR_CODE_DELEGATION_FAILED:
		return agent.ErrCodeDelegationFailed
	case commonpb.ErrorCode_ERROR_CODE_CHILD_AGENT_FAILED:
		return agent.ErrCodeChildAgentFailed
	case commonpb.ErrorCode_ERROR_CODE_CONFIG_ERROR:
		return agent.ErrCodeConfigError
	default:
		return "UNKNOWN"
	}
}

// StringToProtoErrorCode converts string error code to proto ErrorCode.
func StringToProtoErrorCode(code string) commonpb.ErrorCode {
	switch code {
	case agent.ErrCodeInternalError:
		return commonpb.ErrorCode_ERROR_CODE_INTERNAL
	case agent.ErrCodeAgentTimeout:
		return commonpb.ErrorCode_ERROR_CODE_AGENT_TIMEOUT
	case agent.ErrCodeAgentPanic:
		return commonpb.ErrorCode_ERROR_CODE_AGENT_PANIC
	case agent.ErrCodeAgentInitFailed:
		return commonpb.ErrorCode_ERROR_CODE_AGENT_INIT_FAILED
	case agent.ErrCodeLLMRateLimited:
		return commonpb.ErrorCode_ERROR_CODE_LLM_RATE_LIMITED
	case agent.ErrCodeLLMContextExceeded:
		return commonpb.ErrorCode_ERROR_CODE_LLM_CONTEXT_EXCEEDED
	case agent.ErrCodeLLMAPIError:
		return commonpb.ErrorCode_ERROR_CODE_LLM_API_ERROR
	case agent.ErrCodeLLMParseError:
		return commonpb.ErrorCode_ERROR_CODE_LLM_PARSE_ERROR
	case agent.ErrCodeToolNotFound:
		return commonpb.ErrorCode_ERROR_CODE_TOOL_NOT_FOUND
	case agent.ErrCodeToolTimeout:
		return commonpb.ErrorCode_ERROR_CODE_TOOL_TIMEOUT
	case agent.ErrCodeToolExecFailed:
		return commonpb.ErrorCode_ERROR_CODE_TOOL_EXEC_FAILED
	case agent.ErrCodeNetworkTimeout:
		return commonpb.ErrorCode_ERROR_CODE_NETWORK_TIMEOUT
	case agent.ErrCodeNetworkUnreachable:
		return commonpb.ErrorCode_ERROR_CODE_NETWORK_UNREACHABLE
	case agent.ErrCodeTLSError:
		return commonpb.ErrorCode_ERROR_CODE_TLS_ERROR
	case agent.ErrCodeDelegationFailed:
		return commonpb.ErrorCode_ERROR_CODE_DELEGATION_FAILED
	case agent.ErrCodeChildAgentFailed:
		return commonpb.ErrorCode_ERROR_CODE_CHILD_AGENT_FAILED
	case agent.ErrCodeConfigError:
		return commonpb.ErrorCode_ERROR_CODE_CONFIG_ERROR
	default:
		return commonpb.ErrorCode_ERROR_CODE_INTERNAL
	}
}

// ProtoToMissionContext converts proto TypedMap to types.MissionContext.
func ProtoToMissionContext(tm *commonpb.TypedMap) types.MissionContext {
	if tm == nil {
		return types.MissionContext{}
	}

	m := FromTypedMap(tm.Entries)
	if m == nil {
		return types.MissionContext{}
	}

	// Extract fields from the map
	ctx := types.MissionContext{}
	if id, ok := m["id"].(string); ok {
		ctx.ID = id
	}
	if name, ok := m["name"].(string); ok {
		ctx.Name = name
	}
	// Add other fields as needed
	return ctx
}

// MissionContextToProto converts types.MissionContext to proto TypedMap.
func MissionContextToProto(mc types.MissionContext) *commonpb.TypedMap {
	m := map[string]any{
		"id":   mc.ID,
		"name": mc.Name,
	}
	return &commonpb.TypedMap{
		Entries: ToTypedMap(m),
	}
}

// ProtoToTargetInfo converts proto TypedMap to types.TargetInfo.
func ProtoToTargetInfo(tm *commonpb.TypedMap) types.TargetInfo {
	if tm == nil {
		return types.TargetInfo{}
	}

	m := FromTypedMap(tm.Entries)
	if m == nil {
		return types.TargetInfo{}
	}

	// Extract fields from the map
	info := types.TargetInfo{}
	if id, ok := m["id"].(string); ok {
		info.ID = id
	}
	if name, ok := m["name"].(string); ok {
		info.Name = name
	}
	if typ, ok := m["type"].(string); ok {
		info.Type = typ
	}
	if provider, ok := m["provider"].(string); ok {
		info.Provider = provider
	}
	if connection, ok := m["connection"].(map[string]any); ok {
		info.Connection = connection
	}
	if metadata, ok := m["metadata"].(map[string]any); ok {
		info.Metadata = metadata
	}
	return info
}

// TargetInfoToProto converts types.TargetInfo to proto TypedMap.
func TargetInfoToProto(ti types.TargetInfo) *commonpb.TypedMap {
	m := map[string]any{
		"id":         ti.ID,
		"name":       ti.Name,
		"type":       ti.Type,
		"provider":   ti.Provider,
		"connection": ti.Connection,
		"metadata":   ti.Metadata,
	}
	return &commonpb.TypedMap{
		Entries: ToTypedMap(m),
	}
}

// JSONSchemaToProtoNode converts a JSON schema map to proto JSONSchemaNode.
// This is a simplified conversion that handles basic JSON Schema structures.
func JSONSchemaToProtoNode(schema map[string]any) *harnesspb.JSONSchemaNode {
	if schema == nil {
		return nil
	}

	node := &harnesspb.JSONSchemaNode{}

	// Type
	if t, ok := schema["type"].(string); ok {
		node.Type = t
	}

	// Description
	if desc, ok := schema["description"].(string); ok {
		node.Description = desc
	}

	// Properties (object type)
	if props, ok := schema["properties"].(map[string]any); ok {
		node.Properties = make(map[string]*harnesspb.JSONSchemaNode)
		for k, v := range props {
			if propMap, ok := v.(map[string]any); ok {
				node.Properties[k] = JSONSchemaToProtoNode(propMap)
			}
		}
	}

	// Required fields
	if req, ok := schema["required"].([]any); ok {
		node.Required = make([]string, len(req))
		for i, r := range req {
			if s, ok := r.(string); ok {
				node.Required[i] = s
			}
		}
	} else if req, ok := schema["required"].([]string); ok {
		node.Required = req
	}

	// Items (array type)
	if items, ok := schema["items"].(map[string]any); ok {
		node.Items = JSONSchemaToProtoNode(items)
	}

	// Enum values
	if enumVals, ok := schema["enum"].([]any); ok {
		node.EnumValues = make([]string, len(enumVals))
		for i, e := range enumVals {
			node.EnumValues[i] = fmt.Sprintf("%v", e)
		}
	}

	// Format
	if format, ok := schema["format"].(string); ok {
		node.Format = format
	}

	// Pattern
	if pattern, ok := schema["pattern"].(string); ok {
		node.Pattern = &pattern
	}

	// Default value
	if defVal, ok := schema["default"]; ok {
		defaultStr := fmt.Sprintf("%v", defVal)
		node.DefaultValue = &defaultStr
	}

	// Nullable
	if nullable, ok := schema["nullable"].(bool); ok {
		node.Nullable = nullable
	}

	// Numeric constraints
	if min, ok := schema["minimum"].(float64); ok {
		node.Minimum = &min
	}
	if max, ok := schema["maximum"].(float64); ok {
		node.Maximum = &max
	}

	// String constraints
	if minLen, ok := schema["minLength"].(float64); ok {
		minLenInt := int32(minLen)
		node.MinLength = &minLenInt
	}
	if maxLen, ok := schema["maxLength"].(float64); ok {
		maxLenInt := int32(maxLen)
		node.MaxLength = &maxLenInt
	}

	// Array constraints
	if minItems, ok := schema["minItems"].(float64); ok {
		minItemsInt := int32(minItems)
		node.MinItems = &minItemsInt
	}
	if maxItems, ok := schema["maxItems"].(float64); ok {
		maxItemsInt := int32(maxItems)
		node.MaxItems = &maxItemsInt
	}

	return node
}
