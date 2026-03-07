package catalog

import (
	"errors"
	"testing"

	"github.com/hatayama/unity-ai-cli-bridge/internal/unitybridge"
)

func TestBuildToolSummaries_UsesEnabledToolsOnly(t *testing.T) {
	catalog, err := Default()
	if err != nil {
		t.Fatalf("expected default catalog to load, got %v", err)
	}

	summaries := catalog.BuildToolSummaries([]unitybridge.ToolInfo{
		{
			Name:        "Unity_GetConsoleLogs",
			Description: "Get Unity Console logs including messages, warnings, and errors.",
		},
		{
			Name:        "Unity_UnknownTool",
			Description: "Fallback summary source.",
		},
	}, "")

	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %#v", summaries)
	}

	if summaries[0].Name != "Unity_GetConsoleLogs" {
		t.Fatalf("expected Unity_GetConsoleLogs to be listed, got %#v", summaries[0])
	}

	if summaries[0].Category != "console" {
		t.Fatalf("expected console category, got %#v", summaries[0])
	}

	if summaries[1].Category != uncategorizedCategory {
		t.Fatalf("expected uncategorized fallback, got %#v", summaries[1])
	}
}

func TestBuildToolHelp_MergesMetadataAndSchema(t *testing.T) {
	catalog, err := Default()
	if err != nil {
		t.Fatalf("expected default catalog to load, got %v", err)
	}

	help, err := catalog.BuildToolHelp([]unitybridge.ToolInfo{
		{
			Name:        "Unity_GetConsoleLogs",
			Description: "Get Unity Console logs including messages, warnings, and errors with their stack traces.",
			InputSchema: []byte(`{
              "type": "object",
              "properties": {
                "maxEntries": {"type": "integer", "description": "Maximum number of log entries to return"},
                "includeStackTrace": {"type": "boolean", "description": "Whether to include stack traces"}
              },
              "required": ["maxEntries"]
            }`),
		},
	}, "Unity_GetConsoleLogs")
	if err != nil {
		t.Fatalf("expected help to succeed, got %v", err)
	}

	if help.Category != "console" {
		t.Fatalf("expected console category, got %#v", help)
	}

	if help.Summary == "" {
		t.Fatalf("expected summary, got %#v", help)
	}

	if len(help.Arguments) != 2 {
		t.Fatalf("expected 2 arguments, got %#v", help.Arguments)
	}

	if !help.Arguments[1].Required {
		t.Fatalf("expected maxEntries to be required after sorting, got %#v", help.Arguments)
	}

	if len(help.RelatedRecipes) != 1 || help.RelatedRecipes[0] != "inspect-console" {
		t.Fatalf("expected inspect-console relation, got %#v", help.RelatedRecipes)
	}
}

func TestBuildToolHelp_ReturnsErrorForDisabledTool(t *testing.T) {
	catalog, err := Default()
	if err != nil {
		t.Fatalf("expected default catalog to load, got %v", err)
	}

	_, err = catalog.BuildToolHelp([]unitybridge.ToolInfo{
		{
			Name: "Unity_RunCommand",
		},
	}, "Unity_GetConsoleLogs")
	if !errors.Is(err, ErrToolNotEnabled) {
		t.Fatalf("expected ErrToolNotEnabled, got %v", err)
	}
}

func TestBuildRecipeSummaries_FiltersUnavailableRecipes(t *testing.T) {
	catalog, err := Default()
	if err != nil {
		t.Fatalf("expected default catalog to load, got %v", err)
	}

	summaries := catalog.BuildRecipeSummaries([]unitybridge.ToolInfo{
		{Name: "Unity_GetConsoleLogs"},
	})

	if len(summaries) != 1 {
		t.Fatalf("expected only one available recipe, got %#v", summaries)
	}

	if summaries[0].ID != "inspect-console" {
		t.Fatalf("expected inspect-console, got %#v", summaries[0])
	}
}

func TestBuildRecipeHelp_ReturnsMissingTools(t *testing.T) {
	catalog, err := Default()
	if err != nil {
		t.Fatalf("expected default catalog to load, got %v", err)
	}

	_, err = catalog.BuildRecipeHelp([]unitybridge.ToolInfo{
		{Name: "Unity_AssetGeneration_GenerateAsset"},
	}, "generate-asset")
	recipeAvailabilityError := RecipeAvailabilityError{}
	if !errors.As(err, &recipeAvailabilityError) {
		t.Fatalf("expected RecipeAvailabilityError, got %v", err)
	}

	if len(recipeAvailabilityError.MissingTools) != 1 || recipeAvailabilityError.MissingTools[0] != "Unity_AssetGeneration_GetModels" {
		t.Fatalf("unexpected missing tools: %#v", recipeAvailabilityError.MissingTools)
	}
}
