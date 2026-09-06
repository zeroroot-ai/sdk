// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// observationToProto converts a typed agent.Observation into an ObserveRequest.
// Scope is intentionally not carried — the daemon derives it from mission context
// (ADR-0007). Returns an error for an unknown observation type.
func observationToProto(obs agent.Observation) (*harnesspb.ObserveRequest, error) {
	switch o := obs.(type) {
	case agent.HostObservation:
		ports := make([]*harnesspb.PortObservation, len(o.Ports))
		for i, p := range o.Ports {
			eps := make([]*harnesspb.EndpointObservation, len(p.Endpoints))
			for j, e := range p.Endpoints {
				eps[j] = &harnesspb.EndpointObservation{Path: e.Path, Status: int32(e.Status)}
			}
			techs := make([]*harnesspb.TechnologyObservation, len(p.Technologies))
			for j, tch := range p.Technologies {
				techs[j] = &harnesspb.TechnologyObservation{Name: tch.Name, Version: tch.Version}
			}
			var cert *harnesspb.CertificateObservation
			if p.Certificate != nil {
				cert = &harnesspb.CertificateObservation{
					Fingerprint: p.Certificate.Fingerprint,
					Subject:     p.Certificate.Subject,
					Issuer:      p.Certificate.Issuer,
					NotAfter:    p.Certificate.NotAfter,
				}
			}
			ports[i] = &harnesspb.PortObservation{
				Number:       int32(p.Number),
				Protocol:     p.Protocol,
				Service:      p.Service,
				Product:      p.Product,
				Version:      p.Version,
				Endpoints:    eps,
				Technologies: techs,
				Certificate:  cert,
			}
		}
		return &harnesspb.ObserveRequest{
			Observation: &harnesspb.ObserveRequest_Host{
				Host: &harnesspb.HostObservation{
					Address:    o.Address,
					SshHostKey: o.SSHHostKey,
					CloudId:    o.CloudID,
					Ports:      ports,
				},
			},
		}, nil
	case agent.DomainObservation:
		return &harnesspb.ObserveRequest{
			Observation: &harnesspb.ObserveRequest_Domain{
				Domain: &harnesspb.DomainObservation{Name: o.Name},
			},
		}, nil
	case agent.SubdomainObservation:
		return &harnesspb.ObserveRequest{
			Observation: &harnesspb.ObserveRequest_Subdomain{
				Subdomain: &harnesspb.SubdomainObservation{
					Fqdn:      o.FQDN,
					Domain:    o.Domain,
					Addresses: o.Addresses,
				},
			},
		}, nil
	case agent.CredentialObservation:
		return &harnesspb.ObserveRequest{
			Observation: &harnesspb.ObserveRequest_Credential{
				Credential: &harnesspb.CredentialObservation{
					SecretHash: o.SecretHash, Username: o.Username, Kind: o.Kind,
				},
			},
		}, nil
	case agent.AccountObservation:
		return &harnesspb.ObserveRequest{
			Observation: &harnesspb.ObserveRequest_Account{
				Account: &harnesspb.AccountObservation{Identifier: o.Identifier, Kind: o.Kind},
			},
		}, nil
	case agent.MemoryObservation:
		return &harnesspb.ObserveRequest{
			Observation: &harnesspb.ObserveRequest_Memory{
				Memory: &harnesspb.MemoryObservation{
					Text: o.Text, Kind: o.Kind, Tags: o.Tags, SourceRef: o.SourceRef,
				},
			},
		}, nil
	case agent.LifecycleEntityObservation:
		edges := make([]*harnesspb.LifecycleEntityEdge, len(o.Edges))
		for i, e := range o.Edges {
			edges[i] = &harnesspb.LifecycleEntityEdge{
				Type:               e.Type,
				TargetLabel:        e.TargetLabel,
				TargetIdProperties: e.TargetIDProperties,
			}
		}
		return &harnesspb.ObserveRequest{
			Observation: &harnesspb.ObserveRequest_LifecycleEntity{
				LifecycleEntity: &harnesspb.LifecycleEntityObservation{
					Label:        o.Label,
					IdProperties: o.IDProperties,
					Properties:   o.Properties,
					Edges:        edges,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported observation type %T", obs)
	}
}

// Observe emits a typed observation into the World via the callback channel
// (ADR-0007). The daemon resolves identity and topology and derives scope from
// mission context.
func (h *CallbackHarness) Observe(ctx context.Context, obs agent.Observation) error {
	ctx, span := h.tracer.Start(ctx, "gibson.brain.observe", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	req, err := observationToProto(obs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	req.Context = h.client.contextInfo()

	resp, err := h.client.Observe(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("Observe callback failed: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("Observe rejected: %s", resp.Error.Message)
	}
	return nil
}

// Observe is not yet wired in platform pull-mode: that transport emits over the
// component service, which has no typed observation endpoint yet. Platform-hosted
// agents therefore have no graph-write path at all — the generic StoreNode RPC that
// once filled the gap was retired with ADR-0012 (gibson#1265), and it was never
// reachable from this package anyway. No existing agent calls Observe (it is new in
// ADR-0007), so this is safe to leave unsupported here.
func (h *PlatformHarness) Observe(_ context.Context, _ agent.Observation) error {
	return errors.New("Observe is not supported in platform pull-mode yet (use the callback harness); tracked for the platform transport")
}
