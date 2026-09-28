package memory

import (
	"context"
	"testing"

	"gz-ia/internal/features/memory"
	"gz-ia/packages/orchy"
)

func TestMemoryBattery_Tools(t *testing.T) {
	tmpDir := t.TempDir()

	svc := memory.NewService(tmpDir)
	plugin := NewMemoryPlugin(tmpDir, WithMemoryService(svc))

	kernel := orchy.NewKernel()
	if err := kernel.Use(plugin); err != nil {
		t.Fatalf("Use falló: %v", err)
	}

	if err := kernel.Boot(context.Background()); err != nil {
		t.Fatalf("Boot falló: %v", err)
	}
	defer func() { _ = kernel.Shutdown(context.Background()) }()

	ctx := context.Background()

	// 1. memory_save
	saveTool := plugin.saveTool
	saveRes, err := saveTool.Execute(ctx, SaveInput{
		SessionID: "s1",
		Title:     "Decisión de arquitectura",
		Content:   "Usar Microkernel Orchy para plugins MCP",
		Category:  "architecture",
		Tags:      []string{"mcp", "orchy"},
	})
	if err != nil {
		t.Fatalf("memory_save falló: %v", err)
	}
	rec, ok := saveRes.(*memory.Record)
	if !ok || rec.ID == "" {
		t.Fatalf("memory_save resultado inesperado: %v", saveRes)
	}

	// 2. memory_search
	searchTool := plugin.searchTool
	searchRes, err := searchTool.Execute(ctx, SearchInput{
		Query:     "Microkernel Orchy",
		SessionID: "s1",
	})
	if err != nil {
		t.Fatalf("memory_search falló: %v", err)
	}
	results, ok := searchRes.([]memory.SearchResult)
	if !ok || len(results) == 0 {
		t.Fatalf("memory_search resultado inesperado: %v", searchRes)
	}

	// 3. memory_list
	listTool := plugin.listTool
	listRes, err := listTool.Execute(ctx, ListInput{SessionID: "s1"})
	if err != nil {
		t.Fatalf("memory_list falló: %v", err)
	}
	listRecs, ok := listRes.([]memory.Record)
	if !ok || len(listRecs) == 0 {
		t.Fatalf("memory_list resultado inesperado: %v", listRes)
	}

	// 4. memory_consolidate
	consolidateTool := plugin.consolidateTool
	consRes, err := consolidateTool.Execute(ctx, ConsolidateInput{SessionID: "s1"})
	if err != nil {
		t.Fatalf("memory_consolidate falló: %v", err)
	}
	consRecs, ok := consRes.([]memory.Record)
	if !ok || len(consRecs) == 0 {
		t.Fatalf("memory_consolidate resultado inesperado: %v", consRes)
	}
}
