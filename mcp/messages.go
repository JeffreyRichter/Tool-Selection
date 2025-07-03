package mcp

// https://github.com/modelcontextprotocol/modelcontextprotocol/tree/main/schema

import (
	"encoding/json"
)

// Constants
const (
	LatestProtocolVersion = "2025-06-18"
	JSONRPCVersion       = "2.0"
)

// Standard JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Basic types
type ProgressToken interface{} // string | number
type Cursor string
type RequestID interface{} // string | number
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type LoggingLevel string

const (
	LoggingLevelDebug     LoggingLevel = "debug"
	LoggingLevelInfo      LoggingLevel = "info"
	LoggingLevelNotice    LoggingLevel = "notice"
	LoggingLevelWarning   LoggingLevel = "warning"
	LoggingLevelError     LoggingLevel = "error"
	LoggingLevelCritical  LoggingLevel = "critical"
	LoggingLevelAlert     LoggingLevel = "alert"
	LoggingLevelEmergency LoggingLevel = "emergency"
)

// Base interfaces and types
type Meta map[string]interface{}

type Request struct {
	Method string      `json:"method"`
	Params *ReqParams  `json:"params,omitempty"`
}

type ReqParams struct {
	Meta *Meta                  `json:"_meta,omitempty"`
	Data map[string]interface{} `json:",inline"`
}

type Notification struct {
	Method string          `json:"method"`
	Params *NotifyParams   `json:"params,omitempty"`
}

type NotifyParams struct {
	Meta *Meta                  `json:"_meta,omitempty"`
	Data map[string]interface{} `json:",inline"`
}

type Result struct {
	Meta *Meta                  `json:"_meta,omitempty"`
	Data map[string]interface{} `json:",inline"`
}

// JSON-RPC message types
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      RequestID   `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      RequestID   `json:"id"`
	Result  interface{} `json:"result"`
}

type JSONRPCError struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      RequestID `json:"id"`
	Error   struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data,omitempty"`
	} `json:"error"`
}

type JSONRPCMessage interface {
	isJSONRPCMessage()
}

func (r JSONRPCRequest) isJSONRPCMessage()      {}
func (n JSONRPCNotification) isJSONRPCMessage() {}
func (r JSONRPCResponse) isJSONRPCMessage()     {}
func (e JSONRPCError) isJSONRPCMessage()        {}

type EmptyResult struct {
	Meta *Meta `json:"_meta,omitempty"`
}

// Base metadata interface
type BaseMetadata struct {
	Name  string  `json:"name"`
	Title *string `json:"title,omitempty"`
}

// Implementation info
type Implementation struct {
	BaseMetadata
	Version string `json:"version"`
}

// Capabilities
type ClientCapabilities struct {
	Experimental *map[string]interface{} `json:"experimental,omitempty"`
	Roots        *struct {
		ListChanged *bool `json:"listChanged,omitempty"`
	} `json:"roots,omitempty"`
	Sampling    *interface{} `json:"sampling,omitempty"`
	Elicitation *interface{} `json:"elicitation,omitempty"`
}

type ServerCapabilities struct {
	Experimental *map[string]interface{} `json:"experimental,omitempty"`
	Logging      *interface{}            `json:"logging,omitempty"`
	Completions  *interface{}            `json:"completions,omitempty"`
	Prompts      *struct {
		ListChanged *bool `json:"listChanged,omitempty"`
	} `json:"prompts,omitempty"`
	Resources *struct {
		Subscribe   *bool `json:"subscribe,omitempty"`
		ListChanged *bool `json:"listChanged,omitempty"`
	} `json:"resources,omitempty"`
	Tools *struct {
		ListChanged *bool `json:"listChanged,omitempty"`
	} `json:"tools,omitempty"`
}

// Cancellation
type CancelledNotificationParams struct {
	RequestID RequestID `json:"requestId"`
	Reason    *string   `json:"reason,omitempty"`
}

// Initialization
type InitializeRequestParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    *string            `json:"instructions,omitempty"`
	Meta            *Meta              `json:"_meta,omitempty"`
}

// Progress notifications
type ProgressNotificationParams struct {
	ProgressToken ProgressToken `json:"progressToken"`
	Progress      float64       `json:"progress"`
	Total         *float64      `json:"total,omitempty"`
	Message       *string       `json:"message,omitempty"`
}

// Pagination
type PaginatedRequest struct {
	Request
	Cursor *Cursor `json:"cursor,omitempty"`
}

type PaginatedResult struct {
	Result
	NextCursor *Cursor `json:"nextCursor,omitempty"`
}

// Annotations
type Annotations struct {
	Audience     []Role   `json:"audience,omitempty"`
	Priority     *float64 `json:"priority,omitempty"`
	LastModified *string  `json:"lastModified,omitempty"`
}

// Content blocks
type ContentBlock interface {
	isContentBlock()
}

type TextContent struct {
	Type        string       `json:"type"` // "text"
	Text        string       `json:"text"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        *Meta        `json:"_meta,omitempty"`
}

func (t TextContent) isContentBlock() {}

type ImageContent struct {
	Type        string       `json:"type"` // "image"
	Data        string       `json:"data"`
	MimeType    string       `json:"mimeType"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        *Meta        `json:"_meta,omitempty"`
}

func (i ImageContent) isContentBlock() {}

type AudioContent struct {
	Type        string       `json:"type"` // "audio"
	Data        string       `json:"data"`
	MimeType    string       `json:"mimeType"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        *Meta        `json:"_meta,omitempty"`
}

func (a AudioContent) isContentBlock() {}

// Resources
type Resource struct {
	BaseMetadata
	URI         string       `json:"uri"`
	Description *string      `json:"description,omitempty"`
	MimeType    *string      `json:"mimeType,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Size        *int64       `json:"size,omitempty"`
	Meta        *Meta        `json:"_meta,omitempty"`
}

type ResourceTemplate struct {
	BaseMetadata
	URITemplate string       `json:"uriTemplate"`
	Description *string      `json:"description,omitempty"`
	MimeType    *string      `json:"mimeType,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        *Meta        `json:"_meta,omitempty"`
}

type ResourceContents struct {
	URI      string  `json:"uri"`
	MimeType *string `json:"mimeType,omitempty"`
	Meta     *Meta   `json:"_meta,omitempty"`
}

type TextResourceContents struct {
	ResourceContents
	Text string `json:"text"`
}

type BlobResourceContents struct {
	ResourceContents
	Blob string `json:"blob"`
}

type ResourceLink struct {
	Resource
	Type string `json:"type"` // "resource_link"
}

func (r ResourceLink) isContentBlock() {}

type EmbeddedResource struct {
	Type        string       `json:"type"` // "resource"
	Resource    interface{}  `json:"resource"` // TextResourceContents | BlobResourceContents
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        *Meta        `json:"_meta,omitempty"`
}

func (e EmbeddedResource) isContentBlock() {}

// Resource requests/responses
type ListResourcesResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor *Cursor    `json:"nextCursor,omitempty"`
	Meta       *Meta      `json:"_meta,omitempty"`
}

type ListResourceTemplatesResult struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	NextCursor        *Cursor            `json:"nextCursor,omitempty"`
	Meta              *Meta              `json:"_meta,omitempty"`
}

type ReadResourceRequestParams struct {
	URI string `json:"uri"`
}

type ReadResourceResult struct {
	Contents []interface{} `json:"contents"` // (TextResourceContents | BlobResourceContents)[]
	Meta     *Meta         `json:"_meta,omitempty"`
}

type SubscribeRequestParams struct {
	URI string `json:"uri"`
}

type UnsubscribeRequestParams struct {
	URI string `json:"uri"`
}

type ResourceUpdatedNotificationParams struct {
	URI string `json:"uri"`
}

// Prompts
type Prompt struct {
	BaseMetadata
	Description *string          `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Meta        *Meta            `json:"_meta,omitempty"`
}

type PromptArgument struct {
	BaseMetadata
	Description *string `json:"description,omitempty"`
	Required    *bool   `json:"required,omitempty"`
}

type PromptMessage struct {
	Role    Role         `json:"role"`
	Content ContentBlock `json:"content"`
}

type ListPromptsResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor *Cursor  `json:"nextCursor,omitempty"`
	Meta       *Meta    `json:"_meta,omitempty"`
}

type GetPromptRequestParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

type GetPromptResult struct {
	Description *string         `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
	Meta        *Meta           `json:"_meta,omitempty"`
}

// Tools
type ToolAnnotations struct {
	Title           *string `json:"title,omitempty"`
	ReadOnlyHint    *bool   `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool   `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool   `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool   `json:"openWorldHint,omitempty"`
}

type Tool struct {
	BaseMetadata
	Description  *string          `json:"description,omitempty"`
	InputSchema  json.RawMessage  `json:"inputSchema"`
	OutputSchema *json.RawMessage `json:"outputSchema,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
	Meta         *Meta            `json:"_meta,omitempty"`
}

type ListToolsResult struct {
	Tools      []Tool  `json:"tools"`
	NextCursor *Cursor `json:"nextCursor,omitempty"`
	Meta       *Meta   `json:"_meta,omitempty"`
}

type CallToolRequestParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content           []ContentBlock         `json:"content"`
	StructuredContent map[string]interface{} `json:"structuredContent,omitempty"`
	IsError           *bool                  `json:"isError,omitempty"`
	Meta              *Meta                  `json:"_meta,omitempty"`
}

// Logging
type SetLevelRequestParams struct {
	Level LoggingLevel `json:"level"`
}

type LoggingMessageNotificationParams struct {
	Level  LoggingLevel `json:"level"`
	Logger *string      `json:"logger,omitempty"`
	Data   interface{}  `json:"data"`
}

// Sampling
type SamplingMessage struct {
	Role    Role        `json:"role"`
	Content interface{} `json:"content"` // TextContent | ImageContent | AudioContent
}

type ModelHint struct {
	Name *string `json:"name,omitempty"`
}

type ModelPreferences struct {
	Hints                []ModelHint `json:"hints,omitempty"`
	CostPriority         *float64    `json:"costPriority,omitempty"`
	SpeedPriority        *float64    `json:"speedPriority,omitempty"`
	IntelligencePriority *float64    `json:"intelligencePriority,omitempty"`
}

type CreateMessageRequestParams struct {
	Messages         []SamplingMessage `json:"messages"`
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	SystemPrompt     *string           `json:"systemPrompt,omitempty"`
	IncludeContext   *string           `json:"includeContext,omitempty"` // "none" | "thisServer" | "allServers"
	Temperature      *float64          `json:"temperature,omitempty"`
	MaxTokens        int               `json:"maxTokens"`
	StopSequences    []string          `json:"stopSequences,omitempty"`
	Metadata         interface{}       `json:"metadata,omitempty"`
}

type CreateMessageResult struct {
	SamplingMessage
	Model      string  `json:"model"`
	StopReason *string `json:"stopReason,omitempty"` // "endTurn" | "stopSequence" | "maxTokens" | string
	Meta       *Meta   `json:"_meta,omitempty"`
}

// Autocomplete
type PromptReference struct {
	BaseMetadata
	Type string `json:"type"` // "ref/prompt"
}

type ResourceTemplateReference struct {
	Type string `json:"type"` // "ref/resource"
	URI  string `json:"uri"`
}

type CompleteRequestParams struct {
	Ref      interface{} `json:"ref"` // PromptReference | ResourceTemplateReference
	Argument struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"argument"`
	Context *struct {
		Arguments map[string]string `json:"arguments,omitempty"`
	} `json:"context,omitempty"`
}

type CompleteResult struct {
	Completion struct {
		Values  []string `json:"values"`
		Total   *int     `json:"total,omitempty"`
		HasMore *bool    `json:"hasMore,omitempty"`
	} `json:"completion"`
	Meta *Meta `json:"_meta,omitempty"`
}

// Roots
type Root struct {
	URI  string  `json:"uri"`
	Name *string `json:"name,omitempty"`
	Meta *Meta   `json:"_meta,omitempty"`
}

type ListRootsResult struct {
	Roots []Root `json:"roots"`
	Meta  *Meta  `json:"_meta,omitempty"`
}

// Elicitation
type PrimitiveSchemaDefinition interface {
	isPrimitiveSchemaDefinition()
}

type StringSchema struct {
	Type        string  `json:"type"` // "string"
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	MinLength   *int    `json:"minLength,omitempty"`
	MaxLength   *int    `json:"maxLength,omitempty"`
	Format      *string `json:"format,omitempty"` // "email" | "uri" | "date" | "date-time"
}

func (s StringSchema) isPrimitiveSchemaDefinition() {}

type NumberSchema struct {
	Type        string   `json:"type"` // "number" | "integer"
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
}

func (n NumberSchema) isPrimitiveSchemaDefinition() {}

type BooleanSchema struct {
	Type        string  `json:"type"` // "boolean"
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Default     *bool   `json:"default,omitempty"`
}

func (b BooleanSchema) isPrimitiveSchemaDefinition() {}

type EnumSchema struct {
	Type        string   `json:"type"` // "string"
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Enum        []string `json:"enum"`
	EnumNames   []string `json:"enumNames,omitempty"`
}

func (e EnumSchema) isPrimitiveSchemaDefinition() {}

type ElicitRequestParams struct {
	Message         string `json:"message"`
	RequestedSchema struct {
		Type       string                               `json:"type"` // "object"
		Properties map[string]PrimitiveSchemaDefinition `json:"properties"`
		Required   []string                             `json:"required,omitempty"`
	} `json:"requestedSchema"`
}

type ElicitResult struct {
	Action  string                 `json:"action"` // "accept" | "decline" | "cancel"
	Content map[string]interface{} `json:"content,omitempty"`
	Meta    *Meta                  `json:"_meta,omitempty"`
}

// Message type unions for client and server
type ClientRequest interface {
	isClientRequest()
}

type ClientNotification interface {
	isClientNotification()
}

type ClientResult interface {
	isClientResult()
}

type ServerRequest interface {
	isServerRequest()
}

type ServerNotification interface {
	isServerNotification()
}

type ServerResult interface {
	isServerResult()
}

// Implement marker interfaces (examples for key types)
func (r JSONRPCRequest) isClientRequest()  {}
func (r JSONRPCRequest) isServerRequest()  {}
func (n JSONRPCNotification) isClientNotification() {}
func (n JSONRPCNotification) isServerNotification() {}
func (r InitializeResult) isServerResult() {}
func (r CreateMessageResult) isClientResult() {}
func (r ListRootsResult) isClientResult() {}
func (r ElicitResult) isClientResult() {}
func (r EmptyResult) isClientResult() {}
func (r EmptyResult) isServerResult() {}