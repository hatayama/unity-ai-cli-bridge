package catalog

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hatayama/unity-ai-cli-bridge/internal/unitybridge"
)

const uncategorizedCategory = "uncategorized"

var (
	ErrToolNotEnabled = errors.New("tool not currently enabled")
	ErrRecipeNotFound = errors.New("recipe not found")
)

//go:embed tools.json
var toolsJSON []byte

//go:embed recipes.json
var recipesJSON []byte

var (
	defaultCatalogOnce sync.Once
	defaultCatalog     Catalog
	defaultCatalogErr  error
)

type Catalog struct {
	toolsByName map[string]ToolMetadata
	recipesByID map[string]RecipeMetadata
	recipeOrder []string
}

type ToolMetadata struct {
	Tool             string             `json:"tool"`
	Category         string             `json:"category"`
	Summary          string             `json:"summary"`
	WhenToUse        []string           `json:"whenToUse"`
	WhenNotToUse     []string           `json:"whenNotToUse"`
	ArgumentGuidance []ArgumentGuidance `json:"argumentGuidance"`
	Examples         []Example          `json:"examples"`
	SideEffects      []string           `json:"sideEffects"`
	RelatedRecipes   []string           `json:"relatedRecipes"`
}

type ArgumentGuidance struct {
	Name     string `json:"name"`
	Guidance string `json:"guidance"`
}

type Example struct {
	Title   string `json:"title"`
	Command string `json:"command"`
}

type RecipeMetadata struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	Goal            string    `json:"goal"`
	WhenToUse       []string  `json:"whenToUse"`
	Prerequisites   []string  `json:"prerequisites"`
	RequiredTools   []string  `json:"requiredTools"`
	Steps           []string  `json:"steps"`
	ExampleCommands []Example `json:"exampleCommands"`
}

type ToolSummary struct {
	Name            string `json:"name"`
	Category        string `json:"category"`
	Summary         string `json:"summary"`
	MetadataPresent bool   `json:"metadataPresent"`
}

type ToolHelp struct {
	Name            string          `json:"name"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	Category        string          `json:"category"`
	Summary         string          `json:"summary"`
	WhenToUse       []string        `json:"whenToUse,omitempty"`
	WhenNotToUse    []string        `json:"whenNotToUse,omitempty"`
	Arguments       []ArgumentHelp  `json:"arguments,omitempty"`
	Examples        []Example       `json:"examples,omitempty"`
	SideEffects     []string        `json:"sideEffects,omitempty"`
	RelatedRecipes  []string        `json:"relatedRecipes,omitempty"`
	InputSchema     json.RawMessage `json:"inputSchema"`
	OutputSchema    json.RawMessage `json:"outputSchema"`
	MetadataPresent bool            `json:"metadataPresent"`
}

type ArgumentHelp struct {
	Name        string   `json:"name"`
	Required    bool     `json:"required"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Guidance    string   `json:"guidance,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type RecipeSummary struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	RequiredTools []string `json:"requiredTools"`
}

type RecipeHelp struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	Goal            string    `json:"goal"`
	WhenToUse       []string  `json:"whenToUse,omitempty"`
	Prerequisites   []string  `json:"prerequisites,omitempty"`
	RequiredTools   []string  `json:"requiredTools"`
	Steps           []string  `json:"steps"`
	ExampleCommands []Example `json:"exampleCommands,omitempty"`
}

type RecipeAvailabilityError struct {
	RecipeID     string
	MissingTools []string
}

func (err RecipeAvailabilityError) Error() string {
	if len(err.MissingTools) == 0 {
		return fmt.Sprintf("recipe %s is not currently available", err.RecipeID)
	}

	return fmt.Sprintf("recipe %s is not currently available; enable: %s", err.RecipeID, strings.Join(err.MissingTools, ", "))
}

func Default() (Catalog, error) {
	defaultCatalogOnce.Do(func() {
		defaultCatalog, defaultCatalogErr = loadCatalog(toolsJSON, recipesJSON)
	})

	return defaultCatalog, defaultCatalogErr
}

func loadCatalog(rawTools []byte, rawRecipes []byte) (Catalog, error) {
	var toolMetadata []ToolMetadata
	if err := json.Unmarshal(rawTools, &toolMetadata); err != nil {
		return Catalog{}, fmt.Errorf("failed to decode tool catalog: %w", err)
	}

	var recipeMetadata []RecipeMetadata
	if err := json.Unmarshal(rawRecipes, &recipeMetadata); err != nil {
		return Catalog{}, fmt.Errorf("failed to decode recipe catalog: %w", err)
	}

	toolsByName := make(map[string]ToolMetadata, len(toolMetadata))
	for _, metadata := range toolMetadata {
		toolsByName[metadata.Tool] = normalizeToolMetadata(metadata)
	}

	recipesByID := make(map[string]RecipeMetadata, len(recipeMetadata))
	recipeOrder := make([]string, 0, len(recipeMetadata))
	for _, metadata := range recipeMetadata {
		normalizedMetadata := normalizeRecipeMetadata(metadata)
		recipesByID[normalizedMetadata.ID] = normalizedMetadata
		recipeOrder = append(recipeOrder, normalizedMetadata.ID)
	}

	return Catalog{
		toolsByName: toolsByName,
		recipesByID: recipesByID,
		recipeOrder: recipeOrder,
	}, nil
}

func normalizeToolMetadata(metadata ToolMetadata) ToolMetadata {
	normalized := metadata
	normalized.Category = normalizeCategory(metadata.Category)
	return normalized
}

func normalizeRecipeMetadata(metadata RecipeMetadata) RecipeMetadata {
	normalized := metadata
	normalized.RequiredTools = append([]string(nil), metadata.RequiredTools...)
	return normalized
}

func (catalog Catalog) BuildToolSummaries(enabledTools []unitybridge.ToolInfo, category string) []ToolSummary {
	toolMap := indexEnabledTools(enabledTools)
	toolNames := make([]string, 0, len(toolMap))
	for toolName := range toolMap {
		toolNames = append(toolNames, toolName)
	}
	sort.Strings(toolNames)

	normalizedCategory := ""
	if strings.TrimSpace(category) != "" {
		normalizedCategory = normalizeCategory(category)
	}
	summaries := make([]ToolSummary, 0, len(toolNames))
	for _, toolName := range toolNames {
		toolInfo := toolMap[toolName]
		metadata, ok := catalog.toolsByName[toolName]
		toolCategory := uncategorizedCategory
		summary := fallbackSummary(toolInfo.Description)
		if ok {
			toolCategory = metadata.Category
			summary = metadata.Summary
		}

		if normalizedCategory != "" && toolCategory != normalizedCategory {
			continue
		}

		summaries = append(summaries, ToolSummary{
			Name:            toolInfo.Name,
			Category:        toolCategory,
			Summary:         summary,
			MetadataPresent: ok,
		})
	}

	return summaries
}

func (catalog Catalog) BuildToolHelp(enabledTools []unitybridge.ToolInfo, toolName string) (ToolHelp, error) {
	toolMap := indexEnabledTools(enabledTools)
	toolInfo, ok := toolMap[toolName]
	if !ok {
		return ToolHelp{}, fmt.Errorf("%w: %s", ErrToolNotEnabled, toolName)
	}

	metadata, metadataPresent := catalog.toolsByName[toolName]
	help := ToolHelp{
		Name:            toolInfo.Name,
		Title:           toolInfo.Title,
		Description:     toolInfo.Description,
		Category:        uncategorizedCategory,
		Summary:         fallbackSummary(toolInfo.Description),
		InputSchema:     toolInfo.InputSchema,
		OutputSchema:    toolInfo.OutputSchema,
		MetadataPresent: metadataPresent,
	}

	if metadataPresent {
		help.Category = metadata.Category
		help.Summary = metadata.Summary
		help.WhenToUse = append([]string(nil), metadata.WhenToUse...)
		help.WhenNotToUse = append([]string(nil), metadata.WhenNotToUse...)
		help.Examples = append([]Example(nil), metadata.Examples...)
		help.SideEffects = append([]string(nil), metadata.SideEffects...)
		help.RelatedRecipes = catalog.filterAvailableRelatedRecipes(enabledTools, metadata.RelatedRecipes)
	}

	help.Arguments = buildArgumentHelp(toolInfo.InputSchema, metadata.ArgumentGuidance)
	return help, nil
}

func (catalog Catalog) BuildRecipeSummaries(enabledTools []unitybridge.ToolInfo) []RecipeSummary {
	enabledToolSet := buildEnabledToolSet(enabledTools)
	summaries := make([]RecipeSummary, 0, len(catalog.recipeOrder))
	for _, recipeID := range catalog.recipeOrder {
		metadata := catalog.recipesByID[recipeID]
		if !recipeIsAvailable(metadata, enabledToolSet) {
			continue
		}

		summaries = append(summaries, RecipeSummary{
			ID:            metadata.ID,
			Title:         metadata.Title,
			Summary:       metadata.Summary,
			RequiredTools: append([]string(nil), metadata.RequiredTools...),
		})
	}

	return summaries
}

func (catalog Catalog) BuildRecipeHelp(enabledTools []unitybridge.ToolInfo, recipeID string) (RecipeHelp, error) {
	metadata, ok := catalog.recipesByID[recipeID]
	if !ok {
		return RecipeHelp{}, fmt.Errorf("%w: %s", ErrRecipeNotFound, recipeID)
	}

	enabledToolSet := buildEnabledToolSet(enabledTools)
	missingTools := findMissingTools(metadata, enabledToolSet)
	if len(missingTools) > 0 {
		return RecipeHelp{}, RecipeAvailabilityError{
			RecipeID:     recipeID,
			MissingTools: missingTools,
		}
	}

	return RecipeHelp{
		ID:              metadata.ID,
		Title:           metadata.Title,
		Summary:         metadata.Summary,
		Goal:            metadata.Goal,
		WhenToUse:       append([]string(nil), metadata.WhenToUse...),
		Prerequisites:   append([]string(nil), metadata.Prerequisites...),
		RequiredTools:   append([]string(nil), metadata.RequiredTools...),
		Steps:           append([]string(nil), metadata.Steps...),
		ExampleCommands: append([]Example(nil), metadata.ExampleCommands...),
	}, nil
}

func (catalog Catalog) filterAvailableRelatedRecipes(enabledTools []unitybridge.ToolInfo, recipeIDs []string) []string {
	if len(recipeIDs) == 0 {
		return nil
	}

	enabledToolSet := buildEnabledToolSet(enabledTools)
	availableRecipes := make([]string, 0, len(recipeIDs))
	for _, recipeID := range recipeIDs {
		metadata, ok := catalog.recipesByID[recipeID]
		if !ok {
			continue
		}

		if !recipeIsAvailable(metadata, enabledToolSet) {
			continue
		}

		availableRecipes = append(availableRecipes, recipeID)
	}

	return availableRecipes
}

func buildArgumentHelp(rawSchema json.RawMessage, metadata []ArgumentGuidance) []ArgumentHelp {
	if len(rawSchema) == 0 {
		return nil
	}

	var schema map[string]any
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		return nil
	}

	propertiesValue, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}

	requiredProperties := make(map[string]struct{})
	requiredValue, ok := schema["required"].([]any)
	if ok {
		for _, rawPropertyName := range requiredValue {
			propertyName, ok := rawPropertyName.(string)
			if !ok {
				continue
			}

			requiredProperties[propertyName] = struct{}{}
		}
	}

	guidanceByName := make(map[string]string, len(metadata))
	for _, guidance := range metadata {
		guidanceByName[guidance.Name] = guidance.Guidance
	}

	propertyNames := make([]string, 0, len(propertiesValue))
	for propertyName := range propertiesValue {
		propertyNames = append(propertyNames, propertyName)
	}
	sort.Strings(propertyNames)

	arguments := make([]ArgumentHelp, 0, len(propertyNames))
	for _, propertyName := range propertyNames {
		propertyPayload, ok := propertiesValue[propertyName].(map[string]any)
		if !ok {
			continue
		}

		_, required := requiredProperties[propertyName]
		arguments = append(arguments, ArgumentHelp{
			Name:        propertyName,
			Required:    required,
			Type:        schemaType(propertyPayload["type"]),
			Description: stringValue(propertyPayload["description"]),
			Guidance:    guidanceByName[propertyName],
			Enum:        schemaEnum(propertyPayload["enum"]),
		})
	}

	return arguments
}

func schemaType(rawType any) string {
	switch typeValue := rawType.(type) {
	case string:
		return typeValue
	case []any:
		parts := make([]string, 0, len(typeValue))
		for _, rawPart := range typeValue {
			stringPart, ok := rawPart.(string)
			if !ok {
				continue
			}

			parts = append(parts, stringPart)
		}

		return strings.Join(parts, ", ")
	default:
		return "unknown"
	}
}

func schemaEnum(rawEnum any) []string {
	enumValues, ok := rawEnum.([]any)
	if !ok {
		return nil
	}

	values := make([]string, 0, len(enumValues))
	for _, rawValue := range enumValues {
		stringValue, ok := rawValue.(string)
		if !ok {
			continue
		}

		values = append(values, stringValue)
	}

	return values
}

func stringValue(rawValue any) string {
	value, ok := rawValue.(string)
	if !ok {
		return ""
	}

	return value
}

func indexEnabledTools(enabledTools []unitybridge.ToolInfo) map[string]unitybridge.ToolInfo {
	toolMap := make(map[string]unitybridge.ToolInfo, len(enabledTools))
	for _, toolInfo := range enabledTools {
		toolMap[toolInfo.Name] = toolInfo
	}

	return toolMap
}

func buildEnabledToolSet(enabledTools []unitybridge.ToolInfo) map[string]struct{} {
	enabledToolSet := make(map[string]struct{}, len(enabledTools))
	for _, toolInfo := range enabledTools {
		enabledToolSet[toolInfo.Name] = struct{}{}
	}

	return enabledToolSet
}

func recipeIsAvailable(metadata RecipeMetadata, enabledToolSet map[string]struct{}) bool {
	return len(findMissingTools(metadata, enabledToolSet)) == 0
}

func findMissingTools(metadata RecipeMetadata, enabledToolSet map[string]struct{}) []string {
	missingTools := make([]string, 0)
	for _, toolName := range metadata.RequiredTools {
		if _, ok := enabledToolSet[toolName]; ok {
			continue
		}

		missingTools = append(missingTools, toolName)
	}

	return missingTools
}

func fallbackSummary(description string) string {
	trimmedDescription := strings.TrimSpace(description)
	if trimmedDescription == "" {
		return "No summary available."
	}

	firstLine := strings.Split(trimmedDescription, "\n")[0]
	firstLine = strings.TrimSpace(firstLine)
	if firstLine == "" {
		return "No summary available."
	}

	return firstLine
}

func normalizeCategory(category string) string {
	trimmedCategory := strings.TrimSpace(category)
	if trimmedCategory == "" {
		return uncategorizedCategory
	}

	return strings.ToLower(trimmedCategory)
}
