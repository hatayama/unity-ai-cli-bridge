package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/hatayama/unity-ai-cli-bridge/internal/catalog"
	"github.com/hatayama/unity-ai-cli-bridge/internal/unitybridge"
)

func runHelp(args []string, stdout io.Writer, stderr io.Writer) int {
	toolName, parseArgs, err := peelLeadingToolArg(args)
	if err != nil {
		writeln(stderr, err.Error())
		return 1
	}

	flags := flag.NewFlagSet("help", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := bindConnectionFlags(flags)
	jsonOutput := flags.Bool("json", false, "Print JSON output")

	if err := flags.Parse(parseArgs); err != nil {
		return 1
	}

	if toolName == "" && len(flags.Args()) > 1 {
		writeln(stderr, "help accepts at most one tool name")
		return 1
	}

	if toolName == "" && len(flags.Args()) == 1 {
		toolName = flags.Args()[0]
	}

	toolList, cleanup, err := loadToolListWithSpinner(stderr, *options, 20*time.Second)
	if err != nil {
		writef(stderr, "failed to connect to Unity bridge: %v\n", err)
		return 1
	}
	defer cleanup()

	toolCatalog, err := catalog.Default()
	if err != nil {
		writef(stderr, "failed to load tool catalog: %v\n", err)
		return 1
	}

	if toolName == "" {
		summaries := toolCatalog.BuildToolSummaries(toolList.Tools, "")
		if *jsonOutput {
			payload := map[string]any{
				"tools": summaries,
			}
			return printJSON(stdout, payload, true)
		}

		printToolSummaries(stdout, summaries)
		return 0
	}

	help, err := toolCatalog.BuildToolHelp(toolList.Tools, toolName)
	if err != nil {
		if errors.Is(err, catalog.ErrToolNotEnabled) {
			writef(stderr, "tool not currently enabled: %s\n", toolName)
			return 1
		}

		writef(stderr, "failed to build help for tool %s: %v\n", toolName, err)
		return 1
	}

	if *jsonOutput {
		return printJSON(stdout, help, true)
	}

	printToolHelp(stdout, help)
	return 0
}

func runRecipes(args []string, stdout io.Writer, stderr io.Writer) int {
	recipeID, parseArgs, err := peelLeadingToolArg(args)
	if err != nil {
		writeln(stderr, err.Error())
		return 1
	}

	flags := flag.NewFlagSet("recipes", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := bindConnectionFlags(flags)
	jsonOutput := flags.Bool("json", false, "Print JSON output")

	if err := flags.Parse(parseArgs); err != nil {
		return 1
	}

	if recipeID == "" && len(flags.Args()) > 1 {
		writeln(stderr, "recipes accepts at most one recipe id")
		return 1
	}

	if recipeID == "" && len(flags.Args()) == 1 {
		recipeID = flags.Args()[0]
	}

	toolList, cleanup, err := loadToolListWithSpinner(stderr, *options, 20*time.Second)
	if err != nil {
		writef(stderr, "failed to connect to Unity bridge: %v\n", err)
		return 1
	}
	defer cleanup()

	toolCatalog, err := catalog.Default()
	if err != nil {
		writef(stderr, "failed to load recipe catalog: %v\n", err)
		return 1
	}

	if recipeID == "" {
		summaries := toolCatalog.BuildRecipeSummaries(toolList.Tools)
		if *jsonOutput {
			payload := map[string]any{
				"recipes": summaries,
			}
			return printJSON(stdout, payload, true)
		}

		printRecipeSummaries(stdout, summaries)
		return 0
	}

	recipeHelp, err := toolCatalog.BuildRecipeHelp(toolList.Tools, recipeID)
	if err != nil {
		if errors.Is(err, catalog.ErrRecipeNotFound) {
			writef(stderr, "recipe not found: %s\n", recipeID)
			return 1
		}

		recipeAvailabilityError := catalog.RecipeAvailabilityError{}
		if errors.As(err, &recipeAvailabilityError) {
			writef(stderr, "%s\n", recipeAvailabilityError.Error())
			return 1
		}

		writef(stderr, "failed to build recipe %s: %v\n", recipeID, err)
		return 1
	}

	if *jsonOutput {
		return printJSON(stdout, recipeHelp, true)
	}

	printRecipeHelp(stdout, recipeHelp)
	return 0
}

func printToolSummaries(writer io.Writer, summaries []catalog.ToolSummary) {
	if len(summaries) == 0 {
		_, _ = fmt.Fprintln(writer, "No enabled Unity tools are currently exposed.")
		return
	}

	_, _ = fmt.Fprintln(writer, "Enabled Unity tools:")
	for _, summary := range summaries {
		_, _ = fmt.Fprintf(writer, "- %s [%s]: %s\n", summary.Name, summary.Category, summary.Summary)
	}
}

func printToolHelp(writer io.Writer, help catalog.ToolHelp) {
	_, _ = fmt.Fprintf(writer, "Name: %s\n", help.Name)
	_, _ = fmt.Fprintf(writer, "Category: %s\n", help.Category)
	if strings.TrimSpace(help.Title) != "" {
		_, _ = fmt.Fprintf(writer, "Title: %s\n", help.Title)
	}
	_, _ = fmt.Fprintf(writer, "Summary: %s\n", help.Summary)

	if len(help.WhenToUse) > 0 {
		_, _ = fmt.Fprintln(writer, "When To Use:")
		printStringList(writer, help.WhenToUse)
	}

	if len(help.WhenNotToUse) > 0 {
		_, _ = fmt.Fprintln(writer, "When Not To Use:")
		printStringList(writer, help.WhenNotToUse)
	}

	_, _ = fmt.Fprintln(writer, "Arguments:")
	if len(help.Arguments) == 0 {
		_, _ = fmt.Fprintln(writer, "- none")
	} else {
		for _, argument := range help.Arguments {
			requirement := "optional"
			if argument.Required {
				requirement = "required"
			}

			_, _ = fmt.Fprintf(writer, "- %s (%s, %s)", argument.Name, requirement, argument.Type)
			if argument.Description != "" {
				_, _ = fmt.Fprintf(writer, ": %s", argument.Description)
			}
			_, _ = fmt.Fprintln(writer)

			if argument.Guidance != "" {
				_, _ = fmt.Fprintf(writer, "  Guidance: %s\n", argument.Guidance)
			}

			if len(argument.Enum) > 0 {
				_, _ = fmt.Fprintf(writer, "  Allowed Values: %s\n", strings.Join(argument.Enum, ", "))
			}
		}
	}

	if len(help.Examples) > 0 {
		_, _ = fmt.Fprintln(writer, "Examples:")
		printExamples(writer, help.Examples)
	}

	if len(help.SideEffects) > 0 {
		_, _ = fmt.Fprintln(writer, "Side Effects / Cautions:")
		printStringList(writer, help.SideEffects)
	}

	if len(help.RelatedRecipes) > 0 {
		_, _ = fmt.Fprintln(writer, "Related Recipes:")
		sortedRecipes := append([]string(nil), help.RelatedRecipes...)
		sort.Strings(sortedRecipes)
		printStringList(writer, sortedRecipes)
	}
}

func printRecipeSummaries(writer io.Writer, summaries []catalog.RecipeSummary) {
	if len(summaries) == 0 {
		_, _ = fmt.Fprintln(writer, "No recipes are currently available for the enabled Unity tools.")
		return
	}

	_, _ = fmt.Fprintln(writer, "Available recipes:")
	for _, summary := range summaries {
		_, _ = fmt.Fprintf(writer, "- %s: %s\n", summary.ID, summary.Title)
		_, _ = fmt.Fprintf(writer, "  Summary: %s\n", summary.Summary)
		_, _ = fmt.Fprintf(writer, "  Tools: %s\n", strings.Join(summary.RequiredTools, ", "))
	}
}

func printRecipeHelp(writer io.Writer, recipe catalog.RecipeHelp) {
	_, _ = fmt.Fprintf(writer, "ID: %s\n", recipe.ID)
	_, _ = fmt.Fprintf(writer, "Title: %s\n", recipe.Title)
	_, _ = fmt.Fprintf(writer, "Summary: %s\n", recipe.Summary)
	_, _ = fmt.Fprintf(writer, "Goal: %s\n", recipe.Goal)

	if len(recipe.WhenToUse) > 0 {
		_, _ = fmt.Fprintln(writer, "When To Use:")
		printStringList(writer, recipe.WhenToUse)
	}

	if len(recipe.Prerequisites) > 0 {
		_, _ = fmt.Fprintln(writer, "Prerequisites:")
		printStringList(writer, recipe.Prerequisites)
	}

	_, _ = fmt.Fprintln(writer, "Required Tools:")
	printStringList(writer, recipe.RequiredTools)

	_, _ = fmt.Fprintln(writer, "Steps:")
	for index, step := range recipe.Steps {
		_, _ = fmt.Fprintf(writer, "%d. %s\n", index+1, step)
	}

	if len(recipe.ExampleCommands) > 0 {
		_, _ = fmt.Fprintln(writer, "Example Commands:")
		printExamples(writer, recipe.ExampleCommands)
	}
}

func printStringList(writer io.Writer, values []string) {
	for _, value := range values {
		_, _ = fmt.Fprintf(writer, "- %s\n", value)
	}
}

func printExamples(writer io.Writer, examples []catalog.Example) {
	for _, example := range examples {
		_, _ = fmt.Fprintf(writer, "- %s\n", example.Title)
		_, _ = fmt.Fprintf(writer, "  %s\n", example.Command)
	}
}

func buildToolSummaries(toolList unitybridge.ToolListResult, category string) ([]catalog.ToolSummary, error) {
	toolCatalog, err := catalog.Default()
	if err != nil {
		return nil, err
	}

	return toolCatalog.BuildToolSummaries(toolList.Tools, category), nil
}
