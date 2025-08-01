// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated for LSP. DO NOT EDIT.

package protocol

// Code generated from protocol/metaModel.json at ref release/protocol/3.17.6-next.9 (hash c94395b5da53729e6dff931293b051009ccaaaa4).
// https://github.com/microsoft/vscode-languageserver-node/blob/release/protocol/3.17.6-next.9/protocol/metaModel.json
// LSP metaData.version = 3.17.0.

import (
	"context"

	"golang.org/x/tools/internal/jsonrpc2"
)

type Client interface {
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#logTrace
	LogTrace(context.Context, *LogTraceParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#progress
	Progress(context.Context, *ProgressParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#client_registerCapability
	RegisterCapability(context.Context, *RegistrationParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#client_unregisterCapability
	UnregisterCapability(context.Context, *UnregistrationParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#telemetry_event
	Event(context.Context, *any) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#textDocument_publishDiagnostics
	PublishDiagnostics(context.Context, *PublishDiagnosticsParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#window_logMessage
	LogMessage(context.Context, *LogMessageParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#window_showDocument
	ShowDocument(context.Context, *ShowDocumentParams) (*ShowDocumentResult, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#window_showMessage
	ShowMessage(context.Context, *ShowMessageParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#window_showMessageRequest
	ShowMessageRequest(context.Context, *ShowMessageRequestParams) (*MessageActionItem, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#window_workDoneProgress_create
	WorkDoneProgressCreate(context.Context, *WorkDoneProgressCreateParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_applyEdit
	ApplyEdit(context.Context, *ApplyWorkspaceEditParams) (*ApplyWorkspaceEditResult, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_codeLens_refresh
	CodeLensRefresh(context.Context) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_configuration
	Configuration(context.Context, *ParamConfiguration) ([]LSPAny, error)
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_diagnostic_refresh
	DiagnosticRefresh(context.Context) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_foldingRange_refresh
	FoldingRangeRefresh(context.Context) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_inlayHint_refresh
	InlayHintRefresh(context.Context) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_inlineValue_refresh
	InlineValueRefresh(context.Context) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_semanticTokens_refresh
	SemanticTokensRefresh(context.Context) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_textDocumentContent_refresh
	TextDocumentContentRefresh(context.Context, *TextDocumentContentRefreshParams) error
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#workspace_workspaceFolders
	WorkspaceFolders(context.Context) ([]WorkspaceFolder, error)
}

func clientDispatch(ctx context.Context, client Client, reply jsonrpc2.Replier, r jsonrpc2.Request) (bool, error) {
	switch r.Method() {
	case "$/logTrace":
		var params LogTraceParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.LogTrace(ctx, &params)
		return true, reply(ctx, nil, err)

	case "$/progress":
		var params ProgressParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.Progress(ctx, &params)
		return true, reply(ctx, nil, err)

	case "client/registerCapability":
		var params RegistrationParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.RegisterCapability(ctx, &params)
		return true, reply(ctx, nil, err)

	case "client/unregisterCapability":
		var params UnregistrationParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.UnregisterCapability(ctx, &params)
		return true, reply(ctx, nil, err)

	case "telemetry/event":
		var params any
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.Event(ctx, &params)
		return true, reply(ctx, nil, err)

	case "textDocument/publishDiagnostics":
		var params PublishDiagnosticsParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.PublishDiagnostics(ctx, &params)
		return true, reply(ctx, nil, err)

	case "window/logMessage":
		var params LogMessageParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.LogMessage(ctx, &params)
		return true, reply(ctx, nil, err)

	case "window/showDocument":
		var params ShowDocumentParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		resp, err := client.ShowDocument(ctx, &params)
		if err != nil {
			return true, reply(ctx, nil, err)
		}
		return true, reply(ctx, resp, nil)

	case "window/showMessage":
		var params ShowMessageParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.ShowMessage(ctx, &params)
		return true, reply(ctx, nil, err)

	case "window/showMessageRequest":
		var params ShowMessageRequestParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		resp, err := client.ShowMessageRequest(ctx, &params)
		if err != nil {
			return true, reply(ctx, nil, err)
		}
		return true, reply(ctx, resp, nil)

	case "window/workDoneProgress/create":
		var params WorkDoneProgressCreateParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.WorkDoneProgressCreate(ctx, &params)
		return true, reply(ctx, nil, err)

	case "workspace/applyEdit":
		var params ApplyWorkspaceEditParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		resp, err := client.ApplyEdit(ctx, &params)
		if err != nil {
			return true, reply(ctx, nil, err)
		}
		return true, reply(ctx, resp, nil)

	case "workspace/codeLens/refresh":
		err := client.CodeLensRefresh(ctx)
		return true, reply(ctx, nil, err)

	case "workspace/configuration":
		var params ParamConfiguration
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		resp, err := client.Configuration(ctx, &params)
		if err != nil {
			return true, reply(ctx, nil, err)
		}
		return true, reply(ctx, resp, nil)

	case "workspace/diagnostic/refresh":
		err := client.DiagnosticRefresh(ctx)
		return true, reply(ctx, nil, err)

	case "workspace/foldingRange/refresh":
		err := client.FoldingRangeRefresh(ctx)
		return true, reply(ctx, nil, err)

	case "workspace/inlayHint/refresh":
		err := client.InlayHintRefresh(ctx)
		return true, reply(ctx, nil, err)

	case "workspace/inlineValue/refresh":
		err := client.InlineValueRefresh(ctx)
		return true, reply(ctx, nil, err)

	case "workspace/semanticTokens/refresh":
		err := client.SemanticTokensRefresh(ctx)
		return true, reply(ctx, nil, err)

	case "workspace/textDocumentContent/refresh":
		var params TextDocumentContentRefreshParams
		if err := UnmarshalJSON(r.Params(), &params); err != nil {
			return true, sendParseError(ctx, reply, err)
		}
		err := client.TextDocumentContentRefresh(ctx, &params)
		return true, reply(ctx, nil, err)

	case "workspace/workspaceFolders":
		resp, err := client.WorkspaceFolders(ctx)
		if err != nil {
			return true, reply(ctx, nil, err)
		}
		return true, reply(ctx, resp, nil)

	default:
		return false, nil
	}
}

func (s *clientDispatcher) LogTrace(ctx context.Context, params *LogTraceParams) error {
	return s.sender.Notify(ctx, "$/logTrace", params)
}
func (s *clientDispatcher) Progress(ctx context.Context, params *ProgressParams) error {
	return s.sender.Notify(ctx, "$/progress", params)
}
func (s *clientDispatcher) RegisterCapability(ctx context.Context, params *RegistrationParams) error {
	return s.sender.Call(ctx, "client/registerCapability", params, nil)
}
func (s *clientDispatcher) UnregisterCapability(ctx context.Context, params *UnregistrationParams) error {
	return s.sender.Call(ctx, "client/unregisterCapability", params, nil)
}
func (s *clientDispatcher) Event(ctx context.Context, params *any) error {
	return s.sender.Notify(ctx, "telemetry/event", params)
}
func (s *clientDispatcher) PublishDiagnostics(ctx context.Context, params *PublishDiagnosticsParams) error {
	return s.sender.Notify(ctx, "textDocument/publishDiagnostics", params)
}
func (s *clientDispatcher) LogMessage(ctx context.Context, params *LogMessageParams) error {
	return s.sender.Notify(ctx, "window/logMessage", params)
}
func (s *clientDispatcher) ShowDocument(ctx context.Context, params *ShowDocumentParams) (*ShowDocumentResult, error) {
	var result *ShowDocumentResult
	if err := s.sender.Call(ctx, "window/showDocument", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *clientDispatcher) ShowMessage(ctx context.Context, params *ShowMessageParams) error {
	return s.sender.Notify(ctx, "window/showMessage", params)
}
func (s *clientDispatcher) ShowMessageRequest(ctx context.Context, params *ShowMessageRequestParams) (*MessageActionItem, error) {
	var result *MessageActionItem
	if err := s.sender.Call(ctx, "window/showMessageRequest", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *clientDispatcher) WorkDoneProgressCreate(ctx context.Context, params *WorkDoneProgressCreateParams) error {
	return s.sender.Call(ctx, "window/workDoneProgress/create", params, nil)
}
func (s *clientDispatcher) ApplyEdit(ctx context.Context, params *ApplyWorkspaceEditParams) (*ApplyWorkspaceEditResult, error) {
	var result *ApplyWorkspaceEditResult
	if err := s.sender.Call(ctx, "workspace/applyEdit", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *clientDispatcher) CodeLensRefresh(ctx context.Context) error {
	return s.sender.Call(ctx, "workspace/codeLens/refresh", nil, nil)
}
func (s *clientDispatcher) Configuration(ctx context.Context, params *ParamConfiguration) ([]LSPAny, error) {
	var result []LSPAny
	if err := s.sender.Call(ctx, "workspace/configuration", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *clientDispatcher) DiagnosticRefresh(ctx context.Context) error {
	return s.sender.Call(ctx, "workspace/diagnostic/refresh", nil, nil)
}
func (s *clientDispatcher) FoldingRangeRefresh(ctx context.Context) error {
	return s.sender.Call(ctx, "workspace/foldingRange/refresh", nil, nil)
}
func (s *clientDispatcher) InlayHintRefresh(ctx context.Context) error {
	return s.sender.Call(ctx, "workspace/inlayHint/refresh", nil, nil)
}
func (s *clientDispatcher) InlineValueRefresh(ctx context.Context) error {
	return s.sender.Call(ctx, "workspace/inlineValue/refresh", nil, nil)
}
func (s *clientDispatcher) SemanticTokensRefresh(ctx context.Context) error {
	return s.sender.Call(ctx, "workspace/semanticTokens/refresh", nil, nil)
}
func (s *clientDispatcher) TextDocumentContentRefresh(ctx context.Context, params *TextDocumentContentRefreshParams) error {
	return s.sender.Call(ctx, "workspace/textDocumentContent/refresh", params, nil)
}
func (s *clientDispatcher) WorkspaceFolders(ctx context.Context) ([]WorkspaceFolder, error) {
	var result []WorkspaceFolder
	if err := s.sender.Call(ctx, "workspace/workspaceFolders", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
