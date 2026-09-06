package commonpb

#HealthStatus: {
	status?:    string @protobuf(1,string) // healthy, degraded, unhealthy
	message?:   string @protobuf(2,string)
	checkedAt?: int64  @protobuf(3,int64,name=checked_at) // Unix timestamp in milliseconds
}

#JSONSchema: {
	json?: string @protobuf(1,string) // Serialized JSON Schema as string
}

#Error: {
	code?:      string @protobuf(1,string)
	message?:   string @protobuf(2,string)
	details?:   string @protobuf(3,string)
	retryable?: bool   @protobuf(4,bool)
}

// NullValue represents a null value in TypedValue
#NullValue:
	#NULL_VALUE_UNSPECIFIED

#NULL_VALUE_UNSPECIFIED: 0

#NullValue_value: NULL_VALUE_UNSPECIFIED: 0

// TypedValue represents a dynamically typed value
#TypedValue: {
	{} | {
		nullValue: #NullValue @protobuf(1,NullValue,name=null_value)
	} | {
		stringValue: string @protobuf(2,string,name=string_value)
	} | {
		intValue: int64 @protobuf(3,int64,name=int_value)
	} | {
		doubleValue: float64 @protobuf(4,double,name=double_value)
	} | {
		boolValue: bool @protobuf(5,bool,name=bool_value)
	} | {
		bytesValue: bytes @protobuf(6,bytes,name=bytes_value)
	} | {
		arrayValue: #TypedArray @protobuf(7,TypedArray,name=array_value)
	} | {
		mapValue: #TypedMap @protobuf(8,TypedMap,name=map_value)
	}
}

// TypedArray represents an array of TypedValues
#TypedArray: {
	items?: [...#TypedValue] @protobuf(1,TypedValue)
}

// TypedMap represents a map of string keys to TypedValues
#TypedMap: {
	entries?: {
		[string]: #TypedValue
	} @protobuf(1,map[string]TypedValue)
}

// ErrorCode defines standard error codes across the system
#ErrorCode:
	#ERROR_CODE_UNSPECIFIED |
	#ERROR_CODE_INTERNAL |
	#ERROR_CODE_INVALID_ARGUMENT |
	#ERROR_CODE_NOT_FOUND |
	#ERROR_CODE_TIMEOUT |
	#ERROR_CODE_UNAVAILABLE |
	#ERROR_CODE_PERMISSION_DENIED |
	#ERROR_CODE_ALREADY_EXISTS |
	#ERROR_CODE_RESOURCE_EXHAUSTED |
	#ERROR_CODE_CANCELLED |
	#ERROR_CODE_AGENT_TIMEOUT |
	#ERROR_CODE_AGENT_PANIC |
	#ERROR_CODE_AGENT_INIT_FAILED |
	#ERROR_CODE_LLM_RATE_LIMITED |
	#ERROR_CODE_LLM_CONTEXT_EXCEEDED |
	#ERROR_CODE_LLM_API_ERROR |
	#ERROR_CODE_LLM_PARSE_ERROR |
	#ERROR_CODE_TOOL_NOT_FOUND |
	#ERROR_CODE_TOOL_TIMEOUT |
	#ERROR_CODE_TOOL_EXEC_FAILED |
	#ERROR_CODE_NETWORK_TIMEOUT |
	#ERROR_CODE_NETWORK_UNREACHABLE |
	#ERROR_CODE_TLS_ERROR |
	#ERROR_CODE_DELEGATION_FAILED |
	#ERROR_CODE_CHILD_AGENT_FAILED |
	#ERROR_CODE_CONFIG_ERROR

#ERROR_CODE_UNSPECIFIED:          0
#ERROR_CODE_INTERNAL:             1
#ERROR_CODE_INVALID_ARGUMENT:     2
#ERROR_CODE_NOT_FOUND:            3
#ERROR_CODE_TIMEOUT:              4
#ERROR_CODE_UNAVAILABLE:          5
#ERROR_CODE_PERMISSION_DENIED:    6
#ERROR_CODE_ALREADY_EXISTS:       7
#ERROR_CODE_RESOURCE_EXHAUSTED:   8
#ERROR_CODE_CANCELLED:            9
#ERROR_CODE_AGENT_TIMEOUT:        10
#ERROR_CODE_AGENT_PANIC:          11
#ERROR_CODE_AGENT_INIT_FAILED:    12
#ERROR_CODE_LLM_RATE_LIMITED:     13
#ERROR_CODE_LLM_CONTEXT_EXCEEDED: 14
#ERROR_CODE_LLM_API_ERROR:        15
#ERROR_CODE_LLM_PARSE_ERROR:      16
#ERROR_CODE_TOOL_NOT_FOUND:       17
#ERROR_CODE_TOOL_TIMEOUT:         18
#ERROR_CODE_TOOL_EXEC_FAILED:     19
#ERROR_CODE_NETWORK_TIMEOUT:      20
#ERROR_CODE_NETWORK_UNREACHABLE:  21
#ERROR_CODE_TLS_ERROR:            22
#ERROR_CODE_DELEGATION_FAILED:    23
#ERROR_CODE_CHILD_AGENT_FAILED:   24
#ERROR_CODE_CONFIG_ERROR:         25

#ErrorCode_value: {
	ERROR_CODE_UNSPECIFIED:          0
	ERROR_CODE_INTERNAL:             1
	ERROR_CODE_INVALID_ARGUMENT:     2
	ERROR_CODE_NOT_FOUND:            3
	ERROR_CODE_TIMEOUT:              4
	ERROR_CODE_UNAVAILABLE:          5
	ERROR_CODE_PERMISSION_DENIED:    6
	ERROR_CODE_ALREADY_EXISTS:       7
	ERROR_CODE_RESOURCE_EXHAUSTED:   8
	ERROR_CODE_CANCELLED:            9
	ERROR_CODE_AGENT_TIMEOUT:        10
	ERROR_CODE_AGENT_PANIC:          11
	ERROR_CODE_AGENT_INIT_FAILED:    12
	ERROR_CODE_LLM_RATE_LIMITED:     13
	ERROR_CODE_LLM_CONTEXT_EXCEEDED: 14
	ERROR_CODE_LLM_API_ERROR:        15
	ERROR_CODE_LLM_PARSE_ERROR:      16
	ERROR_CODE_TOOL_NOT_FOUND:       17
	ERROR_CODE_TOOL_TIMEOUT:         18
	ERROR_CODE_TOOL_EXEC_FAILED:     19
	ERROR_CODE_NETWORK_TIMEOUT:      20
	ERROR_CODE_NETWORK_UNREACHABLE:  21
	ERROR_CODE_TLS_ERROR:            22
	ERROR_CODE_DELEGATION_FAILED:    23
	ERROR_CODE_CHILD_AGENT_FAILED:   24
	ERROR_CODE_CONFIG_ERROR:         25
}

// HealthState defines standard health states
#HealthState:
	#HEALTH_STATE_UNSPECIFIED |
	#HEALTH_STATE_HEALTHY |
	#HEALTH_STATE_DEGRADED |
	#HEALTH_STATE_UNHEALTHY

#HEALTH_STATE_UNSPECIFIED: 0
#HEALTH_STATE_HEALTHY:     1
#HEALTH_STATE_DEGRADED:    2
#HEALTH_STATE_UNHEALTHY:   3

#HealthState_value: {
	HEALTH_STATE_UNSPECIFIED: 0
	HEALTH_STATE_HEALTHY:     1
	HEALTH_STATE_DEGRADED:    2
	HEALTH_STATE_UNHEALTHY:   3
}

// Metadata contains labels and annotations for resources
#Metadata: {
	labels?: {
		[string]: string
	} @protobuf(1,map[string]string)
	annotations?: {
		[string]: string
	} @protobuf(2,map[string]string)
}

// Principal names who acts: a person, the tenant, a component run, or a
// platform service. Banks record their owner as a Principal. Jobs record who
// opened them and who sent each input as a Principal (gibson#1706).
#Principal: {
	// Kind is the class of principal. It decides how to read id.
	#Kind:
		#KIND_UNSPECIFIED |
		#KIND_USER |
		#KIND_TENANT |
		#KIND_COMPONENT |
		#KIND_SERVICE

	#KIND_UNSPECIFIED: 0

	// KIND_USER is a person. id is the Zitadel user id.
	#KIND_USER: 1

	// KIND_TENANT is the tenant itself. id is the tenant id.
	#KIND_TENANT: 2

	// KIND_COMPONENT is an agent, tool or plugin run. id is the component
	// principal id.
	#KIND_COMPONENT: 3

	// KIND_SERVICE is a platform service. id is the service account id.
	#KIND_SERVICE: 4

	#Kind_value: {
		KIND_UNSPECIFIED: 0
		KIND_USER:        1
		KIND_TENANT:      2
		KIND_COMPONENT:   3
		KIND_SERVICE:     4
	}

	// kind is the class of principal.
	kind?: #Kind @protobuf(1,Kind)

	// id identifies the principal inside its kind.
	id?: string @protobuf(2,string)
}
