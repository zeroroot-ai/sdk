package typespb

import (
	"github.com/zeroroot-ai/sdk/api/proto/gibson/common/v1:commonpb"
	"github.com/zeroroot-ai/sdk/api/proto/gibson/job/v1:jobpb"
)

// ResultStatus represents the execution status of a task or operation.
#ResultStatus:
	#RESULT_STATUS_UNSPECIFIED |
	#RESULT_STATUS_SUCCESS |
	#RESULT_STATUS_FAILED |
	#RESULT_STATUS_PARTIAL |
	#RESULT_STATUS_CANCELLED |
	#RESULT_STATUS_TIMEOUT

#RESULT_STATUS_UNSPECIFIED: 0
#RESULT_STATUS_SUCCESS:     1
#RESULT_STATUS_FAILED:      2
#RESULT_STATUS_PARTIAL:     3
#RESULT_STATUS_CANCELLED:   4
#RESULT_STATUS_TIMEOUT:     5

#ResultStatus_value: {
	RESULT_STATUS_UNSPECIFIED: 0
	RESULT_STATUS_SUCCESS:     1
	RESULT_STATUS_FAILED:      2
	RESULT_STATUS_PARTIAL:     3
	RESULT_STATUS_CANCELLED:   4
	RESULT_STATUS_TIMEOUT:     5
}

// FindingSeverity represents the severity level of a security finding.
#FindingSeverity:
	#FINDING_SEVERITY_UNSPECIFIED |
	#FINDING_SEVERITY_CRITICAL |
	#FINDING_SEVERITY_HIGH |
	#FINDING_SEVERITY_MEDIUM |
	#FINDING_SEVERITY_LOW |
	#FINDING_SEVERITY_INFO

#FINDING_SEVERITY_UNSPECIFIED: 0
#FINDING_SEVERITY_CRITICAL:    1
#FINDING_SEVERITY_HIGH:        2
#FINDING_SEVERITY_MEDIUM:      3
#FINDING_SEVERITY_LOW:         4
#FINDING_SEVERITY_INFO:        5

#FindingSeverity_value: {
	FINDING_SEVERITY_UNSPECIFIED: 0
	FINDING_SEVERITY_CRITICAL:    1
	FINDING_SEVERITY_HIGH:        2
	FINDING_SEVERITY_MEDIUM:      3
	FINDING_SEVERITY_LOW:         4
	FINDING_SEVERITY_INFO:        5
}

// FindingStatus represents the current status of a security finding.
#FindingStatus:
	#FINDING_STATUS_UNSPECIFIED |
	#FINDING_STATUS_OPEN |
	#FINDING_STATUS_CONFIRMED |
	#FINDING_STATUS_CLOSED |
	#FINDING_STATUS_FALSE_POSITIVE

#FINDING_STATUS_UNSPECIFIED:    0
#FINDING_STATUS_OPEN:           1
#FINDING_STATUS_CONFIRMED:      2
#FINDING_STATUS_CLOSED:         3
#FINDING_STATUS_FALSE_POSITIVE: 4

#FindingStatus_value: {
	FINDING_STATUS_UNSPECIFIED:    0
	FINDING_STATUS_OPEN:           1
	FINDING_STATUS_CONFIRMED:      2
	FINDING_STATUS_CLOSED:         3
	FINDING_STATUS_FALSE_POSITIVE: 4
}

// EvidenceType represents the type of evidence collected.
#EvidenceType:
	#EVIDENCE_TYPE_UNSPECIFIED |
	#EVIDENCE_TYPE_REQUEST |
	#EVIDENCE_TYPE_RESPONSE |
	#EVIDENCE_TYPE_SCREENSHOT |
	#EVIDENCE_TYPE_CODE |
	#EVIDENCE_TYPE_LOG |
	#EVIDENCE_TYPE_OTHER

#EVIDENCE_TYPE_UNSPECIFIED: 0
#EVIDENCE_TYPE_REQUEST:     1
#EVIDENCE_TYPE_RESPONSE:    2
#EVIDENCE_TYPE_SCREENSHOT:  3
#EVIDENCE_TYPE_CODE:        4
#EVIDENCE_TYPE_LOG:         5
#EVIDENCE_TYPE_OTHER:       6

#EvidenceType_value: {
	EVIDENCE_TYPE_UNSPECIFIED: 0
	EVIDENCE_TYPE_REQUEST:     1
	EVIDENCE_TYPE_RESPONSE:    2
	EVIDENCE_TYPE_SCREENSHOT:  3
	EVIDENCE_TYPE_CODE:        4
	EVIDENCE_TYPE_LOG:         5
	EVIDENCE_TYPE_OTHER:       6
}

// QueryScope represents the scope of a GraphRAG query.
#QueryScope:
	#QUERY_SCOPE_UNSPECIFIED |
	#QUERY_SCOPE_MISSION_RUN |
	#QUERY_SCOPE_MISSION |
	#QUERY_SCOPE_GLOBAL

#QUERY_SCOPE_UNSPECIFIED: 0
#QUERY_SCOPE_MISSION_RUN: 1
#QUERY_SCOPE_MISSION:     2
#QUERY_SCOPE_GLOBAL:      3

#QueryScope_value: {
	QUERY_SCOPE_UNSPECIFIED: 0
	QUERY_SCOPE_MISSION_RUN: 1
	QUERY_SCOPE_MISSION:     2
	QUERY_SCOPE_GLOBAL:      3
}

// Task represents a goal-oriented task with context and constraints.
//
// context is free-form: the daemon does not read it. Anything the daemon
// must enforce lives in a declared field. Resources a dispatched task needs
// (repositories, credential names, World inputs, acceptance) live in job,
// because the daemon reads them before it mints the task grant.
#Task: {
	id?:   string @protobuf(1,string)
	goal?: string @protobuf(2,string)
	context?: {
		[string]: commonpb.#TypedValue
	} @protobuf(3,map[string]gibson.common.v1.TypedValue)
	constraints?: #TaskConstraints @protobuf(4,TaskConstraints)
	metadata?: {
		[string]: commonpb.#TypedValue
	} @protobuf(5,map[string]gibson.common.v1.TypedValue)

	// job names the typed resources this task needs and how it is judged
	// (gibson#1706). It is set when the task opens or continues a job on a
	// bank member. goal stays the goal; the spec's own goal is ignored on a
	// dispatched task. Absent on a task that needs no repository, no
	// credential and no acceptance step.
	job?: jobpb.#JobSpec @protobuf(6,gibson.job.v1.JobSpec)
}

// TaskConstraints represents execution constraints for a task.
#TaskConstraints: {
	maxTurns?:  int32 @protobuf(1,int32,name=max_turns)
	maxTokens?: int32 @protobuf(2,int32,name=max_tokens)
	allowedTools?: [...string] @protobuf(3,string,name=allowed_tools)
	blockedTools?: [...string] @protobuf(4,string,name=blocked_tools)
}

// Result represents the outcome of a task execution.
#Result: {
	status?: #ResultStatus        @protobuf(1,ResultStatus)
	output?: commonpb.#TypedValue @protobuf(2,gibson.common.v1.TypedValue)
	findingIds?: [...string] @protobuf(3,string,name=finding_ids)
	metadata?: {
		[string]: commonpb.#TypedValue
	} @protobuf(4,map[string]gibson.common.v1.TypedValue)
	error?: #ResultError @protobuf(5,ResultError)
}

// ResultError represents a structured error with retry information.
#ResultError: {
	code?:    commonpb.#ErrorCode @protobuf(1,gibson.common.v1.ErrorCode)
	message?: string              @protobuf(2,string)
	details?: {
		[string]: string
	} @protobuf(3,map[string]string)
	retryable?: bool @protobuf(4,bool)
}

// Finding represents a security vulnerability or issue discovered during testing.
#Finding: {
	id?:            string           @protobuf(1,string)
	missionId?:     string           @protobuf(2,string,name=mission_id)
	agentName?:     string           @protobuf(3,string,name=agent_name)
	delegatedFrom?: string           @protobuf(4,string,name=delegated_from)
	title?:         string           @protobuf(5,string)
	description?:   string           @protobuf(6,string)
	category?:      string           @protobuf(7,string)
	subcategory?:   string           @protobuf(8,string)
	severity?:      #FindingSeverity @protobuf(9,FindingSeverity)
	confidence?:    float64          @protobuf(10,double)
	status?:        #FindingStatus   @protobuf(11,FindingStatus)
	mitreAttack?:   #MitreMapping    @protobuf(12,MitreMapping,name=mitre_attack)
	mitreAtlas?:    #MitreMapping    @protobuf(13,MitreMapping,name=mitre_atlas)
	evidence?: [...#Evidence] @protobuf(14,Evidence)
	reproduction?: [...#ReproStep] @protobuf(15,ReproStep)
	cvssScore?:   float64 @protobuf(16,double,name=cvss_score)
	riskScore?:   float64 @protobuf(17,double,name=risk_score)
	remediation?: string  @protobuf(18,string)
	references?: [...string] @protobuf(19,string)
	targetId?:  string @protobuf(20,string,name=target_id)
	technique?: string @protobuf(21,string)
	tags?: [...string] @protobuf(22,string)
	createdAt?: int64 @protobuf(23,int64,name=created_at) // Unix timestamp in milliseconds
	updatedAt?: int64 @protobuf(24,int64,name=updated_at) // Unix timestamp in milliseconds

	// Compliance framework mappings — added by audit-finding-compliance-mappings.
	// Links the finding to specific control IDs in compliance frameworks
	// (SOC2, NIST AI RMF, MITRE ATLAS, MITRE ATT&CK). Multiple entries per
	// framework allowed; downstream exporters (SARIF) surface them.
	complianceMappings?: [...#ComplianceMapping] @protobuf(25,ComplianceMapping,name=compliance_mappings)
}

// ComplianceMapping links a Finding to a compliance framework control.
// See core/sdk/finding/compliance_mapping.go for the author-side Go type.
#ComplianceMapping: {
	// Compliance framework identifier (e.g. SOC2, NIST_AI_RMF, MITRE_ATLAS,
	// MITRE_ATTACK, PLATFORM).
	framework?: string @protobuf(1,string)

	// Control identifier within the framework.
	controlId?: string @protobuf(2,string,name=control_id)

	// Optional human-readable rationale.
	rationale?: string @protobuf(3,string)

	// Optional pointer to supporting evidence.
	evidenceRef?: string @protobuf(4,string,name=evidence_ref)
}

// MitreMapping represents a mapping to MITRE ATT&CK or ATLAS framework.
#MitreMapping: {
	matrix?:        string @protobuf(1,string)
	tacticId?:      string @protobuf(2,string,name=tactic_id)
	tacticName?:    string @protobuf(3,string,name=tactic_name)
	techniqueId?:   string @protobuf(4,string,name=technique_id)
	techniqueName?: string @protobuf(5,string,name=technique_name)
	subTechniques?: [...string] @protobuf(6,string,name=sub_techniques)
}

// Evidence represents supporting evidence for a finding.
#Evidence: {
	title?:   string        @protobuf(1,string)
	type?:    #EvidenceType @protobuf(2,EvidenceType)
	content?: string        @protobuf(3,string)
	metadata?: {
		[string]: string
	} @protobuf(4,map[string]string)
}

// ReproStep represents a step in reproducing a finding.
#ReproStep: {
	order?:       int32  @protobuf(1,int32)
	description?: string @protobuf(2,string)
	input?:       string @protobuf(3,string)
	output?:      string @protobuf(4,string)
}

// GraphQuery represents a query against the knowledge graph.
#GraphQuery: {
	text?: string @protobuf(1,string)
	embedding?: [...float32] @protobuf(2,float)
	topK?: int32 @protobuf(3,int32,name=top_k)
	nodeTypes?: [...string] @protobuf(4,string,name=node_types)
	minScore?:     float64     @protobuf(5,double,name=min_score)
	maxScore?:     float64     @protobuf(6,double,name=max_score)
	missionId?:    string      @protobuf(7,string,name=mission_id)
	missionRunId?: string      @protobuf(8,string,name=mission_run_id)
	scope?:        #QueryScope @protobuf(9,QueryScope)
	filters?: {
		[string]: string
	} @protobuf(10,map[string]string)

	// Weights for hybrid scoring (must sum to 1.0)
	vectorWeight?: float64 @protobuf(11,double,name=vector_weight)
	graphWeight?:  float64 @protobuf(12,double,name=graph_weight)
}

// MissionRunSummary describes one run of a mission.
//
// Returned by the GetMissionRunHistory knowledge read. An agent lists the runs,
// then pulls findings for a previous one with GetRunFindings — the two are
// consumed together, which is why run history sits with the knowledge reads and
// not with mission lifecycle control.
#MissionRunSummary: {
	missionId?: string @protobuf(1,string,name=mission_id)

	// run_number is sequential from 1.
	runNumber?: int32 @protobuf(2,int32,name=run_number)

	// status is the final status of this run.
	status?:        string @protobuf(3,string)
	findingsCount?: int32  @protobuf(4,int32,name=findings_count)
	createdAt?:     int64  @protobuf(5,int64,name=created_at)

	// completed_at is 0 while the run is still in flight.
	completedAt?: int64 @protobuf(6,int64,name=completed_at)
}
