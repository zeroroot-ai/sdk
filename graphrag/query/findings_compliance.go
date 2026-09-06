// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package query

// FindingsByControlQuery returns findings whose compliance_mappings list
// contains a specific (framework, control_id) entry, scoped to a tenant.
//
// Parameters:
//
//	tenant     — actor_tenant_id to scope the query
//	framework  — e.g. "SOC2", "NIST_AI_RMF", "MITRE_ATLAS"
//	controlID  — e.g. "CC7.1", "MEASURE.2.7", "AML.T0051"
//	limit      — max results (0 = 1000)
//
// Returns a parameterized Cypher string + params map. Callers pass both
// to their Neo4j client unchanged — no string concatenation of user input.
func FindingsByControlQuery(tenant, framework, controlID string, limit int) (string, map[string]any) {
	if limit <= 0 {
		limit = 1000
	}
	cypher := `
MATCH (f:finding)
WHERE f.tenant_id = $tenant
  AND ANY(m IN f.compliance_mappings
          WHERE m.framework = $framework
            AND m.control_id = $control_id)
RETURN f
ORDER BY f.created_at DESC
LIMIT $limit
`
	return cypher, map[string]any{
		"tenant":     tenant,
		"framework":  framework,
		"control_id": controlID,
		"limit":      limit,
	}
}

// FindingsCoverageByFrameworkQuery returns a summary of how many findings
// map to each control within a single framework, tenant-scoped. Used by
// the compliance dashboard coverage tile.
//
// Returns rows of (control_id, finding_count) sorted by finding_count
// descending.
func FindingsCoverageByFrameworkQuery(tenant, framework string) (string, map[string]any) {
	cypher := `
MATCH (f:finding)
WHERE f.tenant_id = $tenant
UNWIND f.compliance_mappings AS m
WITH m
WHERE m.framework = $framework
RETURN m.control_id AS control_id, count(*) AS finding_count
ORDER BY finding_count DESC
`
	return cypher, map[string]any{
		"tenant":    tenant,
		"framework": framework,
	}
}

// FindingsMultiFrameworkQuery returns findings that map to controls in
// TWO OR MORE frameworks — useful for identifying cross-framework
// evidence that proves multiple compliance stories with one finding.
func FindingsMultiFrameworkQuery(tenant string, limit int) (string, map[string]any) {
	if limit <= 0 {
		limit = 500
	}
	cypher := `
MATCH (f:finding)
WHERE f.tenant_id = $tenant
WITH f,
     size(apoc.coll.toSet([m IN f.compliance_mappings | m.framework])) AS framework_count
WHERE framework_count > 1
RETURN f, framework_count
ORDER BY framework_count DESC, f.created_at DESC
LIMIT $limit
`
	return cypher, map[string]any{
		"tenant": tenant,
		"limit":  limit,
	}
}
