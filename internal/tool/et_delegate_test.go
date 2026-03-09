package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/meganerd/pi-go/internal/et"
)

// mockDelegator implements et.Delegator for testing.
type mockDelegator struct {
	available bool
	result    *et.Result
	err       error
}

func (m *mockDelegator) Available() bool { return m.available }
func (m *mockDelegator) Delegate(_ context.Context, _ string, _ *et.Options) (*et.Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestEtDelegateTool_Name(t *testing.T) {
	tool := &EtDelegateTool{}
	if tool.Name() != "et_delegate" {
		t.Errorf("name = %q", tool.Name())
	}
}

func TestEtDelegateTool_NotAvailable(t *testing.T) {
	tool := &EtDelegateTool{Delegator: &mockDelegator{available: false}}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"task":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Output, "not available") {
		t.Errorf("should indicate not available, got: %s", result.Output)
	}
}

func TestEtDelegateTool_NilDelegator(t *testing.T) {
	tool := &EtDelegateTool{}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"task":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Output, "not available") {
		t.Errorf("should indicate not available, got: %s", result.Output)
	}
}

func TestEtDelegateTool_EmptyTask(t *testing.T) {
	tool := &EtDelegateTool{Delegator: &mockDelegator{available: true}}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"task":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Output, "task is required") {
		t.Errorf("should reject empty task, got: %s", result.Output)
	}
}

func TestEtDelegateTool_Success(t *testing.T) {
	tool := &EtDelegateTool{
		Delegator: &mockDelegator{
			available: true,
			result: &et.Result{
				Output:   "Generated 3 files\n",
				Files:    []string{"/tmp/out/main.go", "/tmp/out/main_test.go"},
				ExitCode: 0,
			},
		},
	}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"task":"generate a hello world program"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Output, "Generated 3 files") {
		t.Errorf("should contain et output, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("should list output files, got: %s", result.Output)
	}
}

func TestEtDelegateTool_NonZeroExit(t *testing.T) {
	tool := &EtDelegateTool{
		Delegator: &mockDelegator{
			available: true,
			result: &et.Result{
				Output:   "Build failed\n",
				ExitCode: 1,
			},
		},
	}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"task":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Output, "exited with code 1") {
		t.Errorf("should show exit code, got: %s", result.Output)
	}
}

func TestEtDelegateTool_SchemaValid(t *testing.T) {
	tool := &EtDelegateTool{}
	schema := tool.Schema()
	var parsed map[string]interface{}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("schema type = %v", parsed["type"])
	}
}
