// Package plugin provides the public interface for Relicta plugins.
package plugin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/relicta-tech/relicta/internal/plugin/proto"
)

// getInfoTimeout is the maximum duration for GetInfo RPC calls.
// This prevents indefinite hangs when communicating with plugins.
const getInfoTimeout = 5 * time.Second

// GRPCPlugin is the plugin implementation for gRPC.
type GRPCPlugin struct {
	plugin.Plugin
	Impl Plugin
}

// GRPCServer returns the gRPC server for this plugin.
func (p *GRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterPluginServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

// GRPCClient returns the gRPC client for this plugin.
func (p *GRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: proto.NewPluginClient(c)}, nil
}

// GRPCServer is the server-side implementation of the plugin gRPC interface.
type GRPCServer struct {
	proto.UnimplementedPluginServer
	Impl Plugin
}

// GetInfo returns plugin metadata.
func (s *GRPCServer) GetInfo(ctx context.Context, req *proto.Empty) (*proto.PluginInfo, error) {
	info := s.Impl.GetInfo()

	hooks := make([]string, len(info.Hooks))
	for i, h := range info.Hooks {
		hooks[i] = string(h)
	}

	return &proto.PluginInfo{
		Name:         info.Name,
		Version:      info.Version,
		Description:  info.Description,
		Author:       info.Author,
		Hooks:        hooks,
		ConfigSchema: info.ConfigSchema,
	}, nil
}

// Execute runs the plugin for a given hook.
func (s *GRPCServer) Execute(ctx context.Context, req *proto.ExecuteRequest) (*proto.ExecuteResponse, error) {
	// Convert config from JSON
	var config map[string]any
	if req.Config != "" {
		if err := json.Unmarshal([]byte(req.Config), &config); err != nil {
			return &proto.ExecuteResponse{
				Success: false,
				Error:   "invalid config JSON: " + err.Error(),
			}, nil
		}
	}

	// Convert context
	releaseCtx := ReleaseContext{
		Version:         req.Context.Version,
		PreviousVersion: req.Context.PreviousVersion,
		TagName:         req.Context.TagName,
		ReleaseType:     req.Context.ReleaseType,
		RepositoryURL:   req.Context.RepositoryUrl,
		RepositoryOwner: req.Context.RepositoryOwner,
		RepositoryName:  req.Context.RepositoryName,
		Branch:          req.Context.Branch,
		CommitSHA:       req.Context.CommitSha,
		Changelog:       req.Context.Changelog,
		ReleaseNotes:    req.Context.ReleaseNotes,
		Environment:     req.Context.Environment,
	}

	if req.Context.Changes != nil {
		releaseCtx.Changes = convertProtoChanges(req.Context.Changes)
	}

	// Execute
	resp, err := s.Impl.Execute(ctx, ExecuteRequest{
		Hook:    protoHookToHook(req.Hook),
		Config:  config,
		Context: releaseCtx,
		DryRun:  req.DryRun,
	})
	if err != nil {
		return &proto.ExecuteResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Convert outputs to JSON
	var outputsJSON string
	if resp.Outputs != nil {
		data, _ := json.Marshal(resp.Outputs)
		outputsJSON = string(data)
	}

	// Convert artifacts
	artifacts := make([]*proto.Artifact, len(resp.Artifacts))
	for i, a := range resp.Artifacts {
		artifacts[i] = &proto.Artifact{
			Name:     a.Name,
			Path:     a.Path,
			Type:     a.Type,
			Size:     a.Size,
			Checksum: a.Checksum,
		}
	}

	return &proto.ExecuteResponse{
		Success:   resp.Success,
		Message:   resp.Message,
		Error:     resp.Error,
		Outputs:   outputsJSON,
		Artifacts: artifacts,
	}, nil
}

// Validate validates the plugin configuration.
func (s *GRPCServer) Validate(ctx context.Context, req *proto.ValidateRequest) (*proto.ValidateResponse, error) {
	var config map[string]any
	if req.Config != "" {
		if err := json.Unmarshal([]byte(req.Config), &config); err != nil {
			return &proto.ValidateResponse{
				Valid: false,
				Errors: []*proto.ValidationError{{
					Field:   "config",
					Message: "invalid JSON: " + err.Error(),
				}},
			}, nil
		}
	}

	resp, err := s.Impl.Validate(ctx, config)
	if err != nil {
		return &proto.ValidateResponse{
			Valid: false,
			Errors: []*proto.ValidationError{{
				Field:   "",
				Message: err.Error(),
			}},
		}, nil
	}

	errors := make([]*proto.ValidationError, len(resp.Errors))
	for i, e := range resp.Errors {
		errors[i] = &proto.ValidationError{
			Field:   e.Field,
			Message: e.Message,
			Code:    e.Code,
		}
	}

	return &proto.ValidateResponse{
		Valid:  resp.Valid,
		Errors: errors,
	}, nil
}

// GRPCClient is the client-side implementation of the plugin gRPC interface.
type GRPCClient struct {
	client proto.PluginClient
}

// GetInfo returns plugin metadata.
// Uses a timeout context to prevent indefinite hangs when communicating with plugins.
func (c *GRPCClient) GetInfo() Info {
	ctx, cancel := context.WithTimeout(context.Background(), getInfoTimeout)
	defer cancel()

	resp, err := c.client.GetInfo(ctx, &proto.Empty{})
	if err != nil {
		return Info{}
	}

	hooks := make([]Hook, len(resp.Hooks))
	for i, h := range resp.Hooks {
		hooks[i] = Hook(h)
	}

	return Info{
		Name:         resp.Name,
		Version:      resp.Version,
		Description:  resp.Description,
		Author:       resp.Author,
		Hooks:        hooks,
		ConfigSchema: resp.ConfigSchema,
	}
}

// Execute runs the plugin for the given hook.
func (c *GRPCClient) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	configJSON, _ := json.Marshal(req.Config)

	protoReq := &proto.ExecuteRequest{
		Hook:   hookToProtoHook(req.Hook),
		Config: string(configJSON),
		DryRun: req.DryRun,
	}

	if req.Context.Version != "" {
		protoReq.Context = &proto.ReleaseContext{
			Version:         req.Context.Version,
			PreviousVersion: req.Context.PreviousVersion,
			TagName:         req.Context.TagName,
			ReleaseType:     req.Context.ReleaseType,
			RepositoryUrl:   req.Context.RepositoryURL,
			RepositoryOwner: req.Context.RepositoryOwner,
			RepositoryName:  req.Context.RepositoryName,
			Branch:          req.Context.Branch,
			CommitSha:       req.Context.CommitSHA,
			Changelog:       req.Context.Changelog,
			ReleaseNotes:    req.Context.ReleaseNotes,
			Environment:     req.Context.Environment,
		}

		if req.Context.Changes != nil {
			protoReq.Context.Changes = convertChangesToProto(req.Context.Changes)
		}
	}

	resp, err := c.client.Execute(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	var outputs map[string]any
	if resp.Outputs != "" {
		_ = json.Unmarshal([]byte(resp.Outputs), &outputs) // Ignore unmarshal error for optional field
	}

	artifacts := make([]Artifact, len(resp.Artifacts))
	for i, a := range resp.Artifacts {
		artifacts[i] = Artifact{
			Name:     a.Name,
			Path:     a.Path,
			Type:     a.Type,
			Size:     a.Size,
			Checksum: a.Checksum,
		}
	}

	return &ExecuteResponse{
		Success:   resp.Success,
		Message:   resp.Message,
		Error:     resp.Error,
		Outputs:   outputs,
		Artifacts: artifacts,
	}, nil
}

// Validate validates the plugin configuration.
func (c *GRPCClient) Validate(ctx context.Context, config map[string]any) (*ValidateResponse, error) {
	configJSON, _ := json.Marshal(config)

	resp, err := c.client.Validate(ctx, &proto.ValidateRequest{
		Config: string(configJSON),
	})
	if err != nil {
		return nil, err
	}

	errors := make([]ValidationError, len(resp.Errors))
	for i, e := range resp.Errors {
		errors[i] = ValidationError{
			Field:   e.Field,
			Message: e.Message,
			Code:    e.Code,
		}
	}

	return &ValidateResponse{
		Valid:  resp.Valid,
		Errors: errors,
	}, nil
}

// Helper functions for converting between types

func protoHookToHook(h proto.Hook) Hook {
	switch h {
	case proto.Hook_HOOK_PRE_INIT:
		return HookPreInit
	case proto.Hook_HOOK_POST_INIT:
		return HookPostInit
	case proto.Hook_HOOK_PRE_PLAN:
		return HookPrePlan
	case proto.Hook_HOOK_POST_PLAN:
		return HookPostPlan
	case proto.Hook_HOOK_PRE_VERSION:
		return HookPreVersion
	case proto.Hook_HOOK_POST_VERSION:
		return HookPostVersion
	case proto.Hook_HOOK_PRE_NOTES:
		return HookPreNotes
	case proto.Hook_HOOK_POST_NOTES:
		return HookPostNotes
	case proto.Hook_HOOK_PRE_APPROVE:
		return HookPreApprove
	case proto.Hook_HOOK_POST_APPROVE:
		return HookPostApprove
	case proto.Hook_HOOK_PRE_PUBLISH:
		return HookPrePublish
	case proto.Hook_HOOK_POST_PUBLISH:
		return HookPostPublish
	case proto.Hook_HOOK_ON_SUCCESS:
		return HookOnSuccess
	case proto.Hook_HOOK_ON_ERROR:
		return HookOnError
	default:
		return ""
	}
}

func hookToProtoHook(h Hook) proto.Hook {
	switch h {
	case HookPreInit:
		return proto.Hook_HOOK_PRE_INIT
	case HookPostInit:
		return proto.Hook_HOOK_POST_INIT
	case HookPrePlan:
		return proto.Hook_HOOK_PRE_PLAN
	case HookPostPlan:
		return proto.Hook_HOOK_POST_PLAN
	case HookPreVersion:
		return proto.Hook_HOOK_PRE_VERSION
	case HookPostVersion:
		return proto.Hook_HOOK_POST_VERSION
	case HookPreNotes:
		return proto.Hook_HOOK_PRE_NOTES
	case HookPostNotes:
		return proto.Hook_HOOK_POST_NOTES
	case HookPreApprove:
		return proto.Hook_HOOK_PRE_APPROVE
	case HookPostApprove:
		return proto.Hook_HOOK_POST_APPROVE
	case HookPrePublish:
		return proto.Hook_HOOK_PRE_PUBLISH
	case HookPostPublish:
		return proto.Hook_HOOK_POST_PUBLISH
	case HookOnSuccess:
		return proto.Hook_HOOK_ON_SUCCESS
	case HookOnError:
		return proto.Hook_HOOK_ON_ERROR
	default:
		return proto.Hook_HOOK_UNSPECIFIED
	}
}

func convertProtoChanges(c *proto.CategorizedChanges) *CategorizedChanges {
	return &CategorizedChanges{
		Features:    convertProtoCommits(c.Features),
		Fixes:       convertProtoCommits(c.Fixes),
		Breaking:    convertProtoCommits(c.Breaking),
		Performance: convertProtoCommits(c.Performance),
		Refactor:    convertProtoCommits(c.Refactor),
		Docs:        convertProtoCommits(c.Docs),
		Other:       convertProtoCommits(c.Other),
	}
}

func convertProtoCommits(commits []*proto.ConventionalCommit) []ConventionalCommit {
	result := make([]ConventionalCommit, len(commits))
	for i, c := range commits {
		result[i] = ConventionalCommit{
			Hash:                c.Hash,
			Type:                c.Type,
			Scope:               c.Scope,
			Description:         c.Description,
			Body:                c.Body,
			Breaking:            c.Breaking,
			BreakingDescription: c.BreakingDescription,
			Issues:              c.Issues,
			Author:              c.Author,
			Date:                c.Date,
		}
	}
	return result
}

func convertChangesToProto(c *CategorizedChanges) *proto.CategorizedChanges {
	return &proto.CategorizedChanges{
		Features:    convertCommitsToProto(c.Features),
		Fixes:       convertCommitsToProto(c.Fixes),
		Breaking:    convertCommitsToProto(c.Breaking),
		Performance: convertCommitsToProto(c.Performance),
		Refactor:    convertCommitsToProto(c.Refactor),
		Docs:        convertCommitsToProto(c.Docs),
		Other:       convertCommitsToProto(c.Other),
	}
}

func convertCommitsToProto(commits []ConventionalCommit) []*proto.ConventionalCommit {
	result := make([]*proto.ConventionalCommit, len(commits))
	for i, c := range commits {
		result[i] = &proto.ConventionalCommit{
			Hash:                c.Hash,
			Type:                c.Type,
			Scope:               c.Scope,
			Description:         c.Description,
			Body:                c.Body,
			Breaking:            c.Breaking,
			BreakingDescription: c.BreakingDescription,
			Issues:              c.Issues,
			Author:              c.Author,
			Date:                c.Date,
		}
	}
	return result
}
