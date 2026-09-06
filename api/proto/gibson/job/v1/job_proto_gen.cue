// Package gibson.job.v1 declares jobs, the unit of work a bank member holds
// (gibson#1706, ADR-0017).
//
// Every input to a member is a structured job. A chat turn is a job with
// only a goal. A job is one persistent Claude Code session with its own
// worktrees. It stays open across inputs until a scorer closes it with a
// verdict and a score. The worker never closes its own job.
package jobpb

import (
	"github.com/zeroroot-ai/sdk/api/proto/gibson/common/v1:commonpb"
	"time"
)

// DeliverableKind is what the member does with the work on a repository at
// wrap-up. The driver performs it under the job's connector token. Claude
// commits on the job branch and never holds the token.
#DeliverableKind:
	#DELIVERABLE_KIND_UNSPECIFIED |
	#DELIVERABLE_KIND_PUSH_BRANCH |
	#DELIVERABLE_KIND_MERGE_REQUEST |
	#DELIVERABLE_KIND_NONE

#DELIVERABLE_KIND_UNSPECIFIED: 0

// DELIVERABLE_KIND_PUSH_BRANCH: push the job branch to the remote.
#DELIVERABLE_KIND_PUSH_BRANCH: 1

// DELIVERABLE_KIND_MERGE_REQUEST: push the job branch and open a merge
// request against base_branch.
#DELIVERABLE_KIND_MERGE_REQUEST: 2

// DELIVERABLE_KIND_NONE: leave the work in the sandbox. Nothing leaves.
#DELIVERABLE_KIND_NONE: 3

#DeliverableKind_value: {
	DELIVERABLE_KIND_UNSPECIFIED:   0
	DELIVERABLE_KIND_PUSH_BRANCH:   1
	DELIVERABLE_KIND_MERGE_REQUEST: 2
	DELIVERABLE_KIND_NONE:          3
}

// RepositorySpec names one repository a job works in and what leaves the
// sandbox at wrap-up.
#RepositorySpec: {
	// name is the directory name of the worktree the member sees. Unique
	// inside the job. Required.
	name?: string @protobuf(1,string,"(buf.validate.field).string=")

	// connector_ref names the connector that holds the forge credential, in
	// the slash form "connector/<name>". Required.
	connectorRef?: string @protobuf(2,string,name=connector_ref,"(buf.validate.field).string=")

	// project is the project path on the forge, for example "group/repo".
	// Required.
	project?: string @protobuf(3,string,"(buf.validate.field).string=")

	// base_branch is the branch the job branch starts from and the merge
	// request targets. Empty means the project default branch.
	baseBranch?: string @protobuf(4,string,name=base_branch)

	// deliverable is what the member does with the work at wrap-up.
	// Required.
	deliverable?: #DeliverableKind @protobuf(5,DeliverableKind,"(buf.validate.field).enum=")
}

// Acceptance is how a job is judged. The job node executor runs the verify
// loop against it. A person or an agent that closes a job by hand reads it
// as guidance.
#Acceptance: {
	// verifier_component names the component that judges the work, in the
	// slash form "agent/<name>". Empty means no verify loop: a person or an
	// agent closes the job.
	verifierComponent?: string @protobuf(1,string,name=verifier_component)

	// passing_score is the score, from 0.0 to 1.0, at or above which the
	// verifier accepts the work.
	passingScore?: float64 @protobuf(2,double,name=passing_score,"(buf.validate.field).double=")

	// max_passes is how many verify passes the job node executor runs before
	// it closes the job as FAILED. Zero means one pass. This bounds the
	// verify loop INSIDE one job. A mission node's RetryPolicy is a
	// different thing: it retries the whole node.
	maxPasses?: int32 @protobuf(3,int32,name=max_passes,"(buf.validate.field).int32=")
}

// JobSpec is the structured input that opens a job: the goal, the
// repositories and credentials the work needs, the World nodes it starts
// from, and how it is judged. The daemon reads repositories and
// credential_names before it mints the per-turn grant, so they are declared
// fields. context is free-form. Anything the daemon must enforce lives in a
// declared field.
//
// A JobSpec says WHAT to do. It carries no execution bounds. Bounds are
// gibson.types.v1.TaskConstraints and they ride on the dispatch envelope
// (gibson.types.v1.Task.constraints), which the daemon fills from the bank
// policy and from the mission node. This package imports nothing above
// gibson.common.v1 on purpose, so gibson.types.v1.Task can carry a JobSpec
// without a cyclic import.
#JobSpec: {
	// goal is what the job must achieve, in plain words.
	goal?: string @protobuf(1,string)

	// repositories lists the repositories the job works in.
	repositories?: [...#RepositorySpec] @protobuf(2,RepositorySpec)

	// credential_names lists the credentials the member may read during the
	// job. The per-turn grant covers these and no others.
	credentialNames?: [...string] @protobuf(3,string,name=credential_names)

	// inputs lists World node ids the job starts from: findings, plans,
	// earlier deliverables.
	inputs?: [...string] @protobuf(4,string)

	// acceptance is how the job is judged.
	acceptance?: #Acceptance @protobuf(5,Acceptance)

	// context is free-form context for the member. The daemon does not read
	// it.
	context?: {
		[string]: commonpb.#TypedValue
	} @protobuf(6,map[string]gibson.common.v1.TypedValue)
}

// JobState is where a job is in its life.
#JobState:
	#JOB_STATE_UNSPECIFIED |
	#JOB_STATE_OPEN |
	#JOB_STATE_WORKING |
	#JOB_STATE_WAITING |
	#JOB_STATE_CLOSED

#JOB_STATE_UNSPECIFIED: 0

// JOB_STATE_OPEN: the job is opened and waits for a member to take it.
#JOB_STATE_OPEN: 1

// JOB_STATE_WORKING: the member runs a turn on the job.
#JOB_STATE_WORKING: 2

// JOB_STATE_WAITING: the member asked a question or finished a turn and
// waits for the next input.
#JOB_STATE_WAITING: 3

// JOB_STATE_CLOSED: a scorer closed the job. verdict and score are set.
#JOB_STATE_CLOSED: 4

#JobState_value: {
	JOB_STATE_UNSPECIFIED: 0
	JOB_STATE_OPEN:        1
	JOB_STATE_WORKING:     2
	JOB_STATE_WAITING:     3
	JOB_STATE_CLOSED:      4
}

// JobVerdict is the outcome a scorer gives when it closes a job.
#JobVerdict:
	#JOB_VERDICT_UNSPECIFIED |
	#JOB_VERDICT_ACCOMPLISHED |
	#JOB_VERDICT_FAILED |
	#JOB_VERDICT_ABANDONED

#JOB_VERDICT_UNSPECIFIED: 0

// JOB_VERDICT_ACCOMPLISHED: the job met its acceptance.
#JOB_VERDICT_ACCOMPLISHED: 1

// JOB_VERDICT_FAILED: the job did not meet its acceptance.
#JOB_VERDICT_FAILED: 2

// JOB_VERDICT_ABANDONED: the job went idle past the bank stale limit, or
// the bank was deleted.
#JOB_VERDICT_ABANDONED: 3

#JobVerdict_value: {
	JOB_VERDICT_UNSPECIFIED:  0
	JOB_VERDICT_ACCOMPLISHED: 1
	JOB_VERDICT_FAILED:       2
	JOB_VERDICT_ABANDONED:    3
}

// InputKind is what an input asks the member to do.
#InputKind:
	#INPUT_KIND_UNSPECIFIED |
	#INPUT_KIND_TURN |
	#INPUT_KIND_ANSWER |
	#INPUT_KIND_WRAP_UP

#INPUT_KIND_UNSPECIFIED: 0

// INPUT_KIND_TURN: run one more turn with this message.
#INPUT_KIND_TURN: 1

// INPUT_KIND_ANSWER: this message answers the question the job asked
// when it entered WAITING.
#INPUT_KIND_ANSWER: 2

// INPUT_KIND_WRAP_UP: the final turn after CloseJob. Commit, push, open
// the merge request, and summarize. The daemon sends it, never a client.
#INPUT_KIND_WRAP_UP: 3

#InputKind_value: {
	INPUT_KIND_UNSPECIFIED: 0
	INPUT_KIND_TURN:        1
	INPUT_KIND_ANSWER:      2
	INPUT_KIND_WRAP_UP:     3
}

// JobEventKind is what a JobEvent reports.
#JobEventKind:
	#JOB_EVENT_KIND_UNSPECIFIED |
	#JOB_EVENT_KIND_OPENED |
	#JOB_EVENT_KIND_INPUT |
	#JOB_EVENT_KIND_STATE |
	#JOB_EVENT_KIND_DELIVERABLE |
	#JOB_EVENT_KIND_CLOSED

#JOB_EVENT_KIND_UNSPECIFIED: 0

// JOB_EVENT_KIND_OPENED: the job was opened. state is set.
#JOB_EVENT_KIND_OPENED: 1

// JOB_EVENT_KIND_INPUT: an input was sent. input is set without its
// grant.
#JOB_EVENT_KIND_INPUT: 2

// JOB_EVENT_KIND_STATE: the state changed. state is set.
#JOB_EVENT_KIND_STATE: 3

// JOB_EVENT_KIND_DELIVERABLE: the member reported a deliverable.
// deliverable is set.
#JOB_EVENT_KIND_DELIVERABLE: 4

// JOB_EVENT_KIND_CLOSED: the job closed. verdict and score are set. The
// stream ends after this event.
#JOB_EVENT_KIND_CLOSED: 5

#JobEventKind_value: {
	JOB_EVENT_KIND_UNSPECIFIED: 0
	JOB_EVENT_KIND_OPENED:      1
	JOB_EVENT_KIND_INPUT:       2
	JOB_EVENT_KIND_STATE:       3
	JOB_EVENT_KIND_DELIVERABLE: 4
	JOB_EVENT_KIND_CLOSED:      5
}

// Deliverable is one outward result of a job the driver performed at
// wrap-up.
#Deliverable: {
	// kind is what was done.
	kind?: #DeliverableKind @protobuf(1,DeliverableKind,"(buf.validate.field).enum=")

	// ref is the branch name or the merge request id on the forge.
	ref?: string @protobuf(2,string)

	// url is where a person opens the deliverable.
	url?: string @protobuf(3,string)
}

// Job is one persistent Claude Code session on a bank member with its own
// worktrees.
#Job: {
	// id is the job id. The daemon assigns it.
	id?: string @protobuf(1,string)

	// bank_id is the bank the job was opened on.
	bankId?: string @protobuf(2,string,name=bank_id)

	// member_id is the member that holds the job. Empty while the job waits
	// in the bank queue.
	memberId?: string @protobuf(3,string,name=member_id)

	// state is where the job is in its life.
	state?: #JobState @protobuf(4,JobState,"(buf.validate.field).enum=")

	// spec is the structured input that opened the job.
	spec?: #JobSpec @protobuf(5,JobSpec)

	// claude_session_id is the Claude Code session the member reopens with
	// --resume. The member reports it through ReportJobState.
	claudeSessionId?: string @protobuf(6,string,name=claude_session_id)

	// opened_by is who opened the job.
	openedBy?: commonpb.#Principal @protobuf(7,gibson.common.v1.Principal,name=opened_by)

	// opened_at is when the job was opened.
	openedAt?: time.Time @protobuf(8,google.protobuf.Timestamp,name=opened_at)

	// last_input_at is when the job last received input. The stale limit
	// counts from here.
	lastInputAt?: time.Time @protobuf(9,google.protobuf.Timestamp,name=last_input_at)

	// closed_at is when the job closed. Unset while open.
	closedAt?: time.Time @protobuf(10,google.protobuf.Timestamp,name=closed_at)

	// verdict is the outcome. Unspecified while open.
	verdict?: #JobVerdict @protobuf(11,JobVerdict,"(buf.validate.field).enum=")

	// score is the scorer's score, from 0.0 to 1.0. Zero while open.
	score?: float64 @protobuf(12,double,"(buf.validate.field).double=")

	// deliverables lists what left the sandbox at wrap-up.
	deliverables?: [...#Deliverable] @protobuf(13,Deliverable)

	// attempts counts verify passes the job node executor ran on this job.
	attempts?: int32 @protobuf(14,int32,"(buf.validate.field).int32=")
}

// Input is one message to a job. The member pulls inputs outbound through
// SubscribeInput on the harness callback and runs one turn per input under
// the grant the input carries.
#Input: {
	// job_id is the job the input belongs to.
	jobId?: string @protobuf(1,string,name=job_id)

	// message is the text of the input.
	message?: string @protobuf(2,string)

	// sender is who sent the input.
	sender?: commonpb.#Principal @protobuf(3,gibson.common.v1.Principal)

	// grant is the per-turn capability grant JWT for this input. The daemon
	// sets it when it delivers the input to the member. A client never sets
	// it. The daemon rejects an input that arrives with a grant.
	grant?: string @protobuf(4,string)

	// sent_at is when the input was sent.
	sentAt?: time.Time @protobuf(5,google.protobuf.Timestamp,name=sent_at)

	// kind is what the input asks the member to do.
	kind?: #InputKind @protobuf(6,InputKind,"(buf.validate.field).enum=")

	// id is the input id. The daemon assigns it. A member that reconnects
	// uses it to skip an input it already ran.
	id?: string @protobuf(7,string)
}

// JobEvent is one change on a job, for StreamJobEvents.
#JobEvent: {
	// seq is the per-job sequence number, from 1. Pass it back as since_seq
	// to resume without a gap or a duplicate.
	seq?: uint64 @protobuf(1,uint64)

	// occurred_at is when the change happened.
	occurredAt?: time.Time @protobuf(2,google.protobuf.Timestamp,name=occurred_at)

	// kind is what changed.
	kind?: #JobEventKind @protobuf(3,JobEventKind,"(buf.validate.field).enum=")

	// job_id is the job.
	jobId?: string @protobuf(4,string,name=job_id)

	// state is the state after the change. Set for OPENED, STATE and CLOSED.
	state?: #JobState @protobuf(5,JobState,"(buf.validate.field).enum=")

	// input is the input that was sent, without its grant. Set for INPUT.
	input?: #Input @protobuf(6,Input)

	// deliverable is the deliverable the member reported. Set for
	// DELIVERABLE.
	deliverable?: #Deliverable @protobuf(7,Deliverable)

	// verdict is the outcome. Set for CLOSED.
	verdict?: #JobVerdict @protobuf(8,JobVerdict,"(buf.validate.field).enum=")

	// score is the scorer's score. Set for CLOSED.
	score?: float64 @protobuf(9,double)

	// message is a human-readable note on the change.
	message?: string @protobuf(10,string)
}

// OpenJobRequest opens a job on a bank.
#OpenJobRequest: {
	// bank_id is the bank to open the job on. Required.
	bankId?: string @protobuf(1,string,name=bank_id,"(buf.validate.field).string=")

	// member_id pins the job to one member of that bank. Empty lets the
	// daemon pick a member with a free slot.
	memberId?: string @protobuf(2,string,name=member_id)

	// spec is the structured input. Required.
	spec?: #JobSpec @protobuf(3,JobSpec,"(buf.validate.field).required")
}

// OpenJobResponse carries the opened job.
#OpenJobResponse: {
	// job is the job as stored. member_id is empty while it waits in the
	// queue.
	job?: #Job @protobuf(1,Job)
}

// SendInputRequest sends the next message to an open job.
#SendInputRequest: {
	// job_id is the job. Required.
	jobId?: string @protobuf(1,string,name=job_id,"(buf.validate.field).string=")

	// message is the text of the input. Required.
	message?: string @protobuf(2,string,"(buf.validate.field).string=")

	// kind is what the input asks the member to do. Unspecified means
	// INPUT_KIND_TURN. A client never sends INPUT_KIND_WRAP_UP.
	kind?: #InputKind @protobuf(3,InputKind,"(buf.validate.field).enum=")
}

// SendInputResponse carries the input as the daemon recorded it.
#SendInputResponse: {
	// input is the input without its grant.
	input?: #Input @protobuf(1,Input)
}

// CloseJobRequest closes a job with a verdict and a score.
#CloseJobRequest: {
	// job_id is the job. Required.
	jobId?: string @protobuf(1,string,name=job_id,"(buf.validate.field).string=")

	// verdict is the outcome. Required.
	verdict?: #JobVerdict @protobuf(2,JobVerdict,"(buf.validate.field).enum=")

	// score is the scorer's score, from 0.0 to 1.0.
	score?: float64 @protobuf(3,double,"(buf.validate.field).double=")
}

// CloseJobResponse carries the job after the close was recorded. The
// wrap-up turn runs after this returns.
#CloseJobResponse: {
	// job is the job as stored.
	job?: #Job @protobuf(1,Job)
}

// GetJobRequest names one job.
#GetJobRequest: {
	// job_id is the job. Required.
	jobId?: string @protobuf(1,string,name=job_id,"(buf.validate.field).string=")
}

// GetJobResponse carries one job.
#GetJobResponse: {
	// job is the job.
	job?: #Job @protobuf(1,Job)
}

// ListJobsRequest pages through jobs of the caller's tenant. Every filter
// that is set narrows the list.
#ListJobsRequest: {
	// bank_id keeps only jobs of this bank.
	bankId?: string @protobuf(1,string,name=bank_id)

	// member_id keeps only jobs this member holds.
	memberId?: string @protobuf(2,string,name=member_id)

	// state keeps only jobs in this state. Unspecified means every state.
	state?: #JobState @protobuf(3,JobState,"(buf.validate.field).enum=")

	// page_size is the maximum number of jobs to return. Zero means the
	// server default.
	pageSize?: int32 @protobuf(4,int32,name=page_size,"(buf.validate.field).int32=")

	// page_token is the next_page_token of the previous page. Empty for the
	// first page.
	pageToken?: string @protobuf(5,string,name=page_token)
}

// ListJobsResponse carries one page of jobs.
#ListJobsResponse: {
	// jobs is the page.
	jobs?: [...#Job] @protobuf(1,Job)

	// next_page_token fetches the next page. Empty on the last page.
	nextPageToken?: string @protobuf(2,string,name=next_page_token)
}

// StreamJobEventsRequest names the job to follow.
#StreamJobEventsRequest: {
	// job_id is the job. Required.
	jobId?: string @protobuf(1,string,name=job_id,"(buf.validate.field).string=")

	// since_seq is the last event seq the client saw. The stream starts with
	// the backlog after it. Zero means the whole backlog.
	sinceSeq?: uint64 @protobuf(2,uint64,name=since_seq)
}

// StreamJobEventsResponse carries one job event.
#StreamJobEventsResponse: {
	// event is the change.
	event?: #JobEvent @protobuf(1,JobEvent)
}
