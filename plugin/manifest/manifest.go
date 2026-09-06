// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package manifest defines the Gibson plugin manifest schema, YAML loader, and
// validator for the plugin-runtime spec (plugin.gibson.zeroroot.ai/v1).
//
// A plugin manifest is a YAML file (conventionally named plugin.yaml) that
// serves as the single declarative source of truth for a plugin's identity,
// declared secret needs, RPC method contract, lifecycle configuration, runtime
// requirements, and declared network egress.
//
// The same [Validate] function backs the CLI validator (gibson plugin
// validate), the SDK at plugin startup, and the daemon at registration time.
// All three call-sites surface identical structured errors so misconfigurations
// are caught as early as possible.
//
// Example usage:
//
//	m, err := manifest.Load("plugin.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
package manifest

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// APIVersionV1 is the only supported manifest API version.
const APIVersionV1 = "plugin.gibson.zeroroot.ai/v1"

// KindPlugin is the only valid manifest Kind value.
const KindPlugin = "Plugin"

// WorkloadClassPlugin is the only valid spec.workload_class value for this
// schema. Tool and agent manifests are out of scope for this validator.
const WorkloadClassPlugin = "plugin"

// DefaultStartupTimeout is applied when spec.health.startup_timeout is
// omitted from the manifest.
const DefaultStartupTimeout = 30 * time.Second

// DefaultLivenessInterval is applied when spec.health.liveness_interval is
// omitted from the manifest.
const DefaultLivenessInterval = 10 * time.Second

// DefaultRuntime is the runtime mode assumed when spec.runtime is omitted.
const DefaultRuntime = "process"

// Content-trust classifications for spec.policy.content_trust (ADR-0010).
const (
	// ContentTrustTrusted is the default: the plugin's input is trusted and it
	// may take the in-process dispatch path.
	ContentTrustTrusted = "trusted"
	// ContentTrustUntrusted marks a plugin that processes untrusted input; the
	// daemon's dispatch policy gates it (setec-or-deny under setec-only).
	ContentTrustUntrusted = "untrusted"
	// DefaultContentTrust is assumed when spec.policy.content_trust is omitted.
	DefaultContentTrust = ContentTrustTrusted
)

// Manifest is the top-level structure parsed from a plugin's plugin.yaml file.
// All fields correspond directly to the YAML structure defined by
// plugin.gibson.zeroroot.ai/v1.
type Manifest struct {
	// APIVersion must equal plugin.gibson.zeroroot.ai/v1.
	APIVersion string `yaml:"apiVersion"`

	// Kind must equal "Plugin".
	Kind string `yaml:"kind"`

	// Metadata contains the plugin's identity fields.
	Metadata ManifestMetadata `yaml:"metadata"`

	// Spec contains the plugin's operational declaration.
	Spec ManifestSpec `yaml:"spec"`
}

// ManifestMetadata carries the plugin's identity fields.
type ManifestMetadata struct {
	// Name is a required, DNS-label-style identifier for the plugin.
	// Must match ^[a-z][a-z0-9-]{0,61}[a-z0-9]$.
	Name string `yaml:"name"`

	// Version is the plugin's semantic version (e.g. "1.2.3").
	Version string `yaml:"version"`

	// Description is an optional human-readable explanation of the plugin.
	Description string `yaml:"description,omitempty"`

	// Author is an optional identifier for the plugin's author.
	Author string `yaml:"author,omitempty"`
}

// ManifestSpec is the operational declaration embedded under spec in the YAML.
type ManifestSpec struct {
	// WorkloadClass must equal "plugin". This validator rejects any other value;
	// tool and agent manifests are out of scope for this schema.
	WorkloadClass string `yaml:"workload_class"`

	// Secrets is the list of credential declarations the plugin requires.
	// May be empty when the plugin needs no credentials.
	Secrets []SecretDecl `yaml:"secrets,omitempty"`

	// Methods is the list of RPC method declarations this plugin serves.
	// Must be non-empty (at least one method is required) unless
	// DynamicMethods is true.
	Methods []MethodDecl `yaml:"methods,omitempty"`

	// DynamicMethods, when true, declares that this plugin discovers some or
	// all of its methods at startup via a method source
	// (plugin.WithMethodSource) rather than declaring them statically in
	// spec.methods. When true, spec.methods may be empty.
	DynamicMethods bool `yaml:"dynamic_methods,omitempty"`

	// Runtime selects the execution environment. Must be one of "process",
	// "pod", or "setec". Defaults to "process" when omitted.
	Runtime string `yaml:"runtime,omitempty"`

	// Policy contains security policy overrides for this plugin.
	Policy ManifestPolicy `yaml:"policy,omitempty"`

	// Health carries lifecycle timeout configuration.
	Health ManifestHealth `yaml:"health,omitempty"`

	// Egress declares the external network targets this plugin contacts.
	// Informational in process and pod modes; enforced in setec mode.
	Egress []EgressDecl `yaml:"egress,omitempty"`
}

// SecretDecl declares a single secret that the plugin consumes.
type SecretDecl struct {
	// Name is the broker-qualified name for the secret, e.g.
	// "cred:db_password" or "provider_config:anthropic:default".
	// Must match ^(cred|provider_config):[a-z0-9_/:.-]+$.
	Name string `yaml:"name"`

	// Scope controls when the secret is resolved.
	// Must be one of "startup" or "per_call".
	Scope string `yaml:"scope"`

	// Rotation controls how the plugin reacts to a credential rotation event.
	// Must be one of "live" or "restart".
	Rotation string `yaml:"rotation"`

	// Required, when true, causes the plugin to refuse startup if the secret
	// is unavailable.
	Required bool `yaml:"required"`
}

// MethodDecl declares a single method this plugin implements.
//
// Under the Go-first authoring model (ADR-0065 R4) a method's request/response
// contract is NOT declared here: it is derived by the SDK from the typed Go
// structs the author registers with plugin.WithHandler, and travels to the
// daemon as a JSON-Schema descriptor. The manifest carries only the method's
// identity (name) and its human-readable description for the catalog.
type MethodDecl struct {
	// Name is a required, non-empty method identifier. It must match a handler
	// registered via plugin.WithHandler.
	Name string `yaml:"name"`

	// Description is an optional human-readable explanation of the method,
	// surfaced in the component catalog.
	Description string `yaml:"description,omitempty"`
}

// ManifestPolicy contains security policy overrides for the plugin.
type ManifestPolicy struct {
	// SetecRequired, when true, causes the SDK to refuse startup unless the
	// runtime mode is "setec". Default false.
	SetecRequired bool `yaml:"setec_required,omitempty"`

	// ContentTrust classifies the trust level of the data this plugin processes
	// at call time: "trusted" (default) or "untrusted". An untrusted plugin is
	// gated by the daemon's dispatch policy (ADR-0010 / gibson#997) — under the
	// hosted setec-only shape it must execute in a setec sandbox or be denied,
	// never in-process. Declared here so the trust classification travels with
	// the plugin at registration. Default "trusted".
	ContentTrust string `yaml:"content_trust,omitempty"`
}

// ManifestHealth carries lifecycle probe timeout configuration.
// Duration fields are expressed as Go duration strings in YAML (e.g. "30s").
type ManifestHealth struct {
	// StartupTimeoutRaw is the YAML-parsed startup timeout value.
	// Use [ManifestHealth.EffectiveStartupTimeout] for the resolved value.
	StartupTimeoutRaw parsedDuration `yaml:"startup_timeout,omitempty"`

	// LivenessIntervalRaw is the YAML-parsed liveness interval value.
	// Use [ManifestHealth.EffectiveLivenessInterval] for the resolved value.
	LivenessIntervalRaw parsedDuration `yaml:"liveness_interval,omitempty"`
}

// EffectiveStartupTimeout returns the configured startup timeout, falling back
// to [DefaultStartupTimeout] (30s) when the manifest omits the field.
func (h ManifestHealth) EffectiveStartupTimeout() time.Duration {
	if h.StartupTimeoutRaw.d == 0 {
		return DefaultStartupTimeout
	}
	return h.StartupTimeoutRaw.d
}

// EffectiveLivenessInterval returns the configured liveness probe interval,
// falling back to [DefaultLivenessInterval] (10s) when the manifest omits
// the field.
func (h ManifestHealth) EffectiveLivenessInterval() time.Duration {
	if h.LivenessIntervalRaw.d == 0 {
		return DefaultLivenessInterval
	}
	return h.LivenessIntervalRaw.d
}

// EgressDecl declares a single external network target the plugin contacts.
type EgressDecl struct {
	// Host is a DNS name or wildcard pattern (e.g. "*.example.com").
	// IP addresses are not permitted.
	Host string `yaml:"host"`

	// Protocol must be one of "https", "http", "grpc", "tcp", or "udp".
	Protocol string `yaml:"protocol"`

	// Port is the single destination port. Must be in [1, 65535].
	Port int `yaml:"port"`

	// Purpose is an optional human-readable description of why egress to this
	// target is needed.
	Purpose string `yaml:"purpose,omitempty"`
}

// parsedDuration is a time.Duration that unmarshals from a Go duration string
// in YAML (e.g. "30s", "1m30s"). A zero value represents "not set".
type parsedDuration struct {
	d time.Duration
}

// Duration returns the underlying time.Duration.
func (p parsedDuration) Duration() time.Duration {
	return p.d
}

// UnmarshalYAML implements yaml.Unmarshaler so duration fields are parsed
// from Go duration strings in the manifest.
func (p *parsedDuration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return fmt.Errorf("line %d: health duration must be a string, got %s", value.Line, value.Tag)
	}
	if s == "" {
		p.d = 0
		return nil
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("line %d: invalid duration %q: %w", value.Line, s, err)
	}
	p.d = parsed
	return nil
}

// MarshalYAML implements yaml.Marshaler for round-trip fidelity.
func (p parsedDuration) MarshalYAML() (interface{}, error) {
	if p.d == 0 {
		return "", nil
	}
	return p.d.String(), nil
}

// ValidationError is returned by [Validate] when the manifest violates one or
// more schema rules. The Violations slice describes every problem found in a
// single pass so plugin authors see all issues at once.
type ValidationError struct {
	// Violations is the ordered list of rule violation messages. Never empty.
	Violations []string
}

// Error implements the error interface. It joins all violation messages with
// newlines so the output is readable as a single error string.
func (e *ValidationError) Error() string {
	return "manifest validation failed:\n  " + strings.Join(e.Violations, "\n  ")
}

// --- compiled validation regexps ---

// reName validates spec.metadata.name: DNS-label-ish, 2–63 characters.
var reName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,61}[a-z0-9]$`)

// reSemver validates a v-prefixed or bare semver string.
// Accepts "1.2.3", "1.2.3-alpha.1", "1.2.3+build.1", "v1.2.3" etc.
var reSemver = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-zA-Z0-9.+\-]+)?(\+[a-zA-Z0-9.+]+)?$`)

// reSecretName validates the broker-qualified secret name format from
// Spec 1 Requirement 12.
var reSecretName = regexp.MustCompile(`^(cred|provider_config):[a-z0-9_/:.\-]+$`)

// reDNSHost matches a valid hostname or wildcard like "*.example.com".
// IPs are rejected because the pattern requires at least one non-digit label.
var reDNSHost = regexp.MustCompile(`^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// Load parses the YAML manifest at path, applies defaults, and validates it.
//
// Unknown YAML fields cause an error whose message includes the line number of
// the first unknown key (via the strict decoder). Validation errors from
// cross-field rules are collected in a single [*ValidationError] so all
// violations are visible at once.
//
// Load is equivalent to:
//
//	b, err := os.ReadFile(path)
//	if err != nil { return nil, err }
//	return LoadBytes(b)
func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: open %s: %w", path, err)
	}
	m, err := LoadBytes(b)
	if err != nil {
		return nil, fmt.Errorf("manifest: parse %s: %w", path, err)
	}
	return m, nil
}

// LoadBytes parses the YAML manifest from b, applies defaults, and validates
// it.
//
// Unknown YAML fields cause an error whose message includes the YAML line
// number of the first unknown key.
func LoadBytes(b []byte) (*Manifest, error) {
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true) // reject unknown fields; errors include line numbers

	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest: yaml decode: %w", err)
	}

	ApplyDefaults(&m)

	if err := Validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ApplyDefaults fills in zero-value fields with their specified defaults.
// [Load] and [LoadBytes] call it automatically; callers constructing a
// Manifest in memory (e.g. plugin.WithParsedManifest) should call it before
// [Validate].
func ApplyDefaults(m *Manifest) {
	if m.Spec.Runtime == "" {
		m.Spec.Runtime = DefaultRuntime
	}
	if m.Spec.Policy.ContentTrust == "" {
		m.Spec.Policy.ContentTrust = DefaultContentTrust
	}
	// Duration defaults are surfaced via EffectiveStartupTimeout /
	// EffectiveLivenessInterval; no mutation needed here.
}

// Validate performs a full semantic check of m against the manifest schema.
// All violations are collected in a single pass and returned together as a
// [*ValidationError] so callers can surface every issue at once.
//
// Validate does NOT resolve proto message types against a live proto registry —
// that requires a protoregistry.Files available only at daemon registration
// time (Requirement 1.5). Proto name validation here is lexical only.
//
// Returns nil if the manifest is valid.
func Validate(m *Manifest) error {
	var violations []string
	add := func(format string, args ...any) {
		violations = append(violations, fmt.Sprintf(format, args...))
	}

	// --- top-level ---
	if m.APIVersion != APIVersionV1 {
		add("apiVersion must be %q, got %q", APIVersionV1, m.APIVersion)
	}
	if m.Kind != KindPlugin {
		add("kind must be %q, got %q", KindPlugin, m.Kind)
	}

	// --- metadata ---
	if m.Metadata.Name == "" {
		add("metadata.name is required")
	} else if !reName.MatchString(m.Metadata.Name) {
		add("metadata.name %q must match ^[a-z][a-z0-9-]{0,61}[a-z0-9]$", m.Metadata.Name)
	}
	if m.Metadata.Version == "" {
		add("metadata.version is required")
	} else if !reSemver.MatchString(m.Metadata.Version) {
		add("metadata.version %q is not valid semver (e.g. 1.2.3 or 0.1.0-alpha)", m.Metadata.Version)
	}

	// --- spec.workload_class ---
	if m.Spec.WorkloadClass != WorkloadClassPlugin {
		add("spec.workload_class must be %q (this validator governs plugins only; "+
			"tool and agent manifests are out of scope), got %q",
			WorkloadClassPlugin, m.Spec.WorkloadClass)
	}

	// --- spec.secrets ---
	for i, s := range m.Spec.Secrets {
		pfx := fmt.Sprintf("spec.secrets[%d]", i)
		if s.Name == "" {
			add("%s.name is required", pfx)
		} else if !reSecretName.MatchString(s.Name) {
			add("%s.name %q must match ^(cred|provider_config):[a-z0-9_/:.-]+$", pfx, s.Name)
		}
		switch s.Scope {
		case "startup", "per_call":
			// valid
		default:
			add("%s.scope must be \"startup\" or \"per_call\", got %q", pfx, s.Scope)
		}
		switch s.Rotation {
		case "live", "restart":
			// valid
		default:
			add("%s.rotation must be \"live\" or \"restart\", got %q", pfx, s.Rotation)
		}
	}

	// --- spec.methods ---
	if len(m.Spec.Methods) == 0 && !m.Spec.DynamicMethods {
		add("spec.methods is required and must contain at least one method " +
			"(or set spec.dynamic_methods: true for startup discovery)")
	}
	for i, meth := range m.Spec.Methods {
		pfx := fmt.Sprintf("spec.methods[%d]", i)
		if meth.Name == "" {
			add("%s.name is required", pfx)
		}
	}

	// --- spec.runtime ---
	switch m.Spec.Runtime {
	case "process", "pod", "setec", "":
		// valid ("" is accepted here because ApplyDefaults may not have run
		// when Validate is called directly on a partially-constructed Manifest)
	default:
		add("spec.runtime must be one of \"process\", \"pod\", or \"setec\", got %q", m.Spec.Runtime)
	}

	// --- spec.policy.content_trust ---
	switch m.Spec.Policy.ContentTrust {
	case ContentTrustTrusted, ContentTrustUntrusted, "":
		// valid ("" accepted pre-ApplyDefaults, as with spec.runtime above)
	default:
		add("spec.policy.content_trust must be %q or %q, got %q", ContentTrustTrusted, ContentTrustUntrusted, m.Spec.Policy.ContentTrust)
	}

	// --- spec.egress ---
	violations = append(violations, ValidateEgress("spec.egress", m.Spec.Egress)...)

	if len(violations) > 0 {
		return &ValidationError{Violations: violations}
	}
	return nil
}

// ValidateEgress checks a list of egress declarations and returns the rule
// violations found, each prefixed "<prefix>[i]". It backs the spec.egress
// section of [Validate] and is exported so sister manifest schemas that embed
// [EgressDecl] (e.g. the MCP-bridge connector manifest) enforce identical
// rules. Returns nil when every declaration is valid.
func ValidateEgress(prefix string, decls []EgressDecl) []string {
	var violations []string
	add := func(format string, args ...any) {
		violations = append(violations, fmt.Sprintf(format, args...))
	}
	for i, e := range decls {
		pfx := fmt.Sprintf("%s[%d]", prefix, i)
		if e.Host == "" {
			add("%s.host is required", pfx)
		} else if !reDNSHost.MatchString(e.Host) {
			add("%s.host %q must be a DNS name or wildcard (e.g. *.example.com); "+
				"IP addresses are not permitted", pfx, e.Host)
		}
		switch e.Protocol {
		case "https", "http", "grpc", "tcp", "udp":
			// valid
		default:
			add("%s.protocol must be one of \"https\", \"http\", \"grpc\", \"tcp\", \"udp\", got %q",
				pfx, e.Protocol)
		}
		if e.Port < 1 || e.Port > 65535 {
			add("%s.port must be in [1, 65535], got %d", pfx, e.Port)
		}
	}
	return violations
}

// IsValidationError reports whether err is a [*ValidationError] produced by
// [Validate]. This is useful for distinguishing schema violations from I/O or
// parse errors.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}
