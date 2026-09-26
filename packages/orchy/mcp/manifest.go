package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"gz-ia/packages/orchy/tools"
)

// McpToolDefinition defines a tool exposed via the Model Context Protocol.
type McpToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolCatalogMetrics represents execution metrics in a catalog item.
type ToolCatalogMetrics struct {
	TotalCalls        int   `json:"totalCalls"`
	SuccessfulCalls   int   `json:"successfulCalls"`
	FailedCalls       int   `json:"failedCalls"`
	AverageDurationMs int64 `json:"averageDurationMs"`
}

// ToolCatalogItem represents a registered tool in the introspection catalog.
type ToolCatalogItem struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Status      string             `json:"status"`
	Schema      tools.ToolSchema   `json:"schema"`
	Metrics     ToolCatalogMetrics `json:"metrics"`
}

// PluginDescriptor provides basic metadata about a loaded plugin.
type PluginDescriptor interface {
	Name() string
	Version() string
}

// ManifestGenerator produces MCP tool definitions and AI system prompts from registered tools and plugins.
type ManifestGenerator struct {
	toolRegistry   *tools.ToolRegistry
	pluginProvider func() []PluginDescriptor
}

// NewManifestGenerator constructs a ManifestGenerator.
func NewManifestGenerator(toolRegistry *tools.ToolRegistry, pluginProviders ...func() []PluginDescriptor) *ManifestGenerator {
	var provider func() []PluginDescriptor
	if len(pluginProviders) > 0 {
		provider = pluginProviders[0]
	}

	return &ManifestGenerator{
		toolRegistry:   toolRegistry,
		pluginProvider: provider,
	}
}

// ToStandardJsonSchema converts a ToolSchema into a standard JSON Schema object.
func (m *ManifestGenerator) ToStandardJsonSchema(schema tools.ToolSchema) map[string]any {
	result := make(map[string]any)

	// Determine type
	schemaType := "object"
	if schema.Kind != "" {
		schemaType = schema.Kind
	} else if schema.Type != "" {
		schemaType = schema.Type
	}
	result["type"] = schemaType

	if schema.Description != "" {
		result["description"] = schema.Description
	}

	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	if schema.AdditionalProperties != nil {
		result["additionalProperties"] = *schema.AdditionalProperties
	}

	if schema.Properties != nil {
		props := make(map[string]any)
		for k, v := range schema.Properties {
			props[k] = m.transformSchemaNode(v, false)
		}
		result["properties"] = props
	}

	if schema.Items != nil {
		result["items"] = m.transformSchemaNode(schema.Items, false)
	}

	for k, v := range schema.Extra {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}

	return result
}

func (m *ManifestGenerator) transformSchemaNode(node any, isRoot bool) any {
	if node == nil {
		return nil
	}

	switch val := node.(type) {
	case tools.ToolSchema:
		return m.ToStandardJsonSchema(val)
	case *tools.ToolSchema:
		if val == nil {
			return nil
		}
		return m.ToStandardJsonSchema(*val)
	case map[string]any:
		result := make(map[string]any)
		if kind, ok := val["kind"].(string); ok && kind != "" {
			result["type"] = kind
		} else if typ, ok := val["type"].(string); ok && typ != "" {
			result["type"] = typ
		} else if isRoot {
			result["type"] = "object"
		}

		for k, v := range val {
			if k == "kind" {
				continue
			}
			if k == "properties" {
				if propMap, ok := v.(map[string]any); ok {
					props := make(map[string]any)
					for pk, pv := range propMap {
						props[pk] = m.transformSchemaNode(pv, false)
					}
					result["properties"] = props
					continue
				}
			}
			result[k] = m.transformSchemaNode(v, false)
		}
		return result
	case []any:
		resList := make([]any, len(val))
		for i, item := range val {
			resList[i] = m.transformSchemaNode(item, false)
		}
		return resList
	default:
		return val
	}
}

// GenerateMcpTools returns MCP tool definitions for all healthy (non-DEAD) registered tools.
func (m *ManifestGenerator) GenerateMcpTools() []McpToolDefinition {
	if m.toolRegistry == nil {
		return nil
	}

	healthyTools := m.toolRegistry.GetHealthyTools()
	definitions := make([]McpToolDefinition, 0, len(healthyTools))

	for _, tool := range healthyTools {
		definitions = append(definitions, McpToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: m.ToStandardJsonSchema(tool.Schema()),
		})
	}

	return definitions
}

// GenerateCatalog returns catalog items for all registered tools, including status and metrics.
func (m *ManifestGenerator) GenerateCatalog() []ToolCatalogItem {
	if m.toolRegistry == nil {
		return nil
	}

	all := m.toolRegistry.List()
	catalog := make([]ToolCatalogItem, 0, len(all))

	for _, tool := range all {
		metrics := tool.Metrics()
		catalog = append(catalog, ToolCatalogItem{
			Name:        tool.Name(),
			Description: tool.Description(),
			Status:      string(tool.Status()),
			Schema:      tool.Schema(),
			Metrics: ToolCatalogMetrics{
				TotalCalls:        metrics.TotalCalls,
				SuccessfulCalls:   metrics.SuccessfulCalls,
				FailedCalls:       metrics.FailedCalls,
				AverageDurationMs: int64(metrics.AverageDurationMs),
			},
		})
	}

	return catalog
}

// GenerateAiContextMarkdown generates Markdown documentation suitable for LLM system prompts.
func (m *ManifestGenerator) GenerateAiContextMarkdown(systemName string) string {
	if systemName == "" {
		systemName = "Orchy Microkernel System"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s - AI Context\n\n", systemName))
	sb.WriteString("Este documento describe las capacidades y herramientas activas en el entorno.\n\n")

	if m.pluginProvider != nil {
		plugins := m.pluginProvider()
		if len(plugins) > 0 {
			sb.WriteString("## Plugins Cargados\n")
			for _, p := range plugins {
				sb.WriteString(fmt.Sprintf("- **%s** (v%s)\n", p.Name(), p.Version()))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("## Herramientas Disponibles (Tools)\n")
	if m.toolRegistry == nil || m.toolRegistry.Count() == 0 {
		sb.WriteString("_No hay herramientas registradas actualmente._\n")
		return sb.String()
	}

	toolsList := m.toolRegistry.List()
	for _, tool := range toolsList {
		schemaJSON, _ := json.MarshalIndent(m.ToStandardJsonSchema(tool.Schema()), "", "  ")

		sb.WriteString(fmt.Sprintf("### `%s`\n", tool.Name()))
		sb.WriteString(fmt.Sprintf("- **Estado**: `%s`\n", tool.Status()))
		sb.WriteString(fmt.Sprintf("- **Descripción**: %s\n", tool.Description()))
		sb.WriteString("- **Parámetros (Input Schema)**:\n```json\n")
		sb.WriteString(string(schemaJSON))
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}
