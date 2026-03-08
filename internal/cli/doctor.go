package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hatayama/unity-ai-cli-bridge/internal/diagnostics"
	"github.com/hatayama/unity-ai-cli-bridge/internal/protocol"
)

const (
	doctorStatusOK    = "ok"
	doctorStatusWarn  = "warn"
	doctorStatusError = "error"
)

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type doctorReport struct {
	ProjectRoot     string        `json:"projectRoot"`
	ConnectionFile  string        `json:"connectionFile"`
	StatusFile      string        `json:"statusFile"`
	DiagnosticsFile string        `json:"diagnosticsFile"`
	ToolsFile       string        `json:"toolsFile"`
	Checks          []doctorCheck `json:"checks"`
	Advice          []string      `json:"advice"`
}

func (report doctorReport) HasFailure() bool {
	for _, check := range report.Checks {
		if check.Status == doctorStatusError {
			return true
		}
	}

	return false
}

func buildDoctorReport(options *connectionOptions, liveTimeout time.Duration) doctorReport {
	report := doctorReport{
		Checks: make([]doctorCheck, 0, 8),
		Advice: make([]string, 0, 4),
	}

	projectRoot := options.resolveProjectRoot()
	report.ProjectRoot = projectRoot
	if strings.TrimSpace(projectRoot) == "" {
		report.Checks = append(report.Checks, doctorCheck{
			Name:    "project",
			Status:  doctorStatusWarn,
			Message: "could not infer the Unity project root from the current directory",
		})
	} else {
		report.Checks = append(report.Checks, doctorCheck{
			Name:    "project",
			Status:  doctorStatusOK,
			Message: fmt.Sprintf("resolved project root %s", projectRoot),
		})
	}

	snapshotPaths := diagnostics.PathsForProject(projectRoot)
	report.DiagnosticsFile = snapshotPaths.DiagnosticsFile
	report.ToolsFile = snapshotPaths.ToolsFile

	diagnosticsSnapshot, _, diagnosticsErr := diagnostics.LoadDiagnostics(projectRoot)
	if diagnosticsErr == nil {
		report.Checks = append(report.Checks, doctorCheck{
			Name:    "companion-diagnostics",
			Status:  doctorStatusOK,
			Message: fmt.Sprintf("loaded diagnostics snapshot from %s", snapshotPaths.DiagnosticsFile),
		})
	} else {
		report.Checks = append(report.Checks, doctorCheck{
			Name:    "companion-diagnostics",
			Status:  doctorStatusWarn,
			Message: fmt.Sprintf("diagnostics snapshot is not available: %v", diagnosticsErr),
		})
	}

	toolSnapshot, _, toolSnapshotErr := diagnostics.LoadToolSnapshot(projectRoot)
	if toolSnapshotErr == nil {
		report.Checks = append(report.Checks, doctorCheck{
			Name:    "tool-snapshot",
			Status:  doctorStatusOK,
			Message: fmt.Sprintf("loaded %d tools from %s", len(toolSnapshot.Tools), snapshotPaths.ToolsFile),
		})
	} else {
		report.Checks = append(report.Checks, doctorCheck{
			Name:    "tool-snapshot",
			Status:  doctorStatusWarn,
			Message: fmt.Sprintf("tool snapshot is not available: %v", toolSnapshotErr),
		})
	}

	resolved, resolveErr := options.resolve()
	if resolveErr != nil {
		report.Checks = append(report.Checks, doctorCheck{
			Name:    "discovery",
			Status:  doctorStatusError,
			Message: resolveErr.Error(),
		})
		report.Advice = append(report.Advice, "Open the Unity project and make sure Project Settings > AI > Unity MCP is enabled.")
		return report
	}

	report.ConnectionFile = resolved.ConnectionFile
	report.StatusFile = resolved.StatusFile
	report.Checks = append(report.Checks, doctorCheck{
		Name:    "discovery",
		Status:  doctorStatusOK,
		Message: fmt.Sprintf("resolved bridge discovery file %s", resolved.ConnectionFile),
	})

	heartbeatStatus := "missing"
	heartbeatLevel := doctorStatusWarn
	if resolved.StatusInfo != nil {
		heartbeatStatus = resolved.StatusInfo.Status
		if strings.EqualFold(resolved.StatusInfo.Status, "ready") {
			heartbeatLevel = doctorStatusOK
		}
	}

	report.Checks = append(report.Checks, doctorCheck{
		Name:    "heartbeat",
		Status:  heartbeatLevel,
		Message: fmt.Sprintf("bridge heartbeat status is %s", heartbeatStatus),
	})

	shouldProbeLive := heartbeatLevel != doctorStatusOK
	if diagnosticsSnapshot != nil && diagnosticsSnapshot.BridgeRunning {
		shouldProbeLive = false
	}

	if !shouldProbeLive {
		report.Checks = append(report.Checks, doctorCheck{
			Name:    "live-probe",
			Status:  doctorStatusOK,
			Message: "skipped because heartbeat or diagnostics already report a healthy bridge",
		})
		return report
	}

	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()

	bridge, cleanup, err := openBridgeClient(*options, liveTimeout)
	if err != nil {
		report.Checks = append(report.Checks, liveProbeFailureCheck(err))
		addDoctorAdvice(&report, err, diagnosticsSnapshot)
		return report
	}
	defer cleanup()

	_, err = bridge.ListTools(ctx, "")
	if err != nil {
		report.Checks = append(report.Checks, liveProbeFailureCheck(err))
		addDoctorAdvice(&report, err, diagnosticsSnapshot)
		return report
	}

	report.Checks = append(report.Checks, doctorCheck{
		Name:    "live-probe",
		Status:  doctorStatusOK,
		Message: "live bridge probe succeeded",
	})
	return report
}

func liveProbeFailureCheck(err error) doctorCheck {
	if errors.Is(err, context.DeadlineExceeded) {
		return doctorCheck{
			Name:    "live-probe",
			Status:  doctorStatusWarn,
			Message: "live bridge probe timed out",
		}
	}

	var approvalDenied protocol.ApprovalDeniedError
	if errors.As(err, &approvalDenied) {
		return doctorCheck{
			Name:    "live-probe",
			Status:  doctorStatusError,
			Message: approvalDenied.Error(),
		}
	}

	return doctorCheck{
		Name:    "live-probe",
		Status:  doctorStatusWarn,
		Message: err.Error(),
	}
}

func addDoctorAdvice(report *doctorReport, err error, diagnosticsSnapshot *diagnostics.DiagnosticsSnapshot) {
	var approvalDenied protocol.ApprovalDeniedError
	if errors.As(err, &approvalDenied) {
		report.Advice = append(report.Advice, "Approve the pending unity-ai-cli client in Project Settings > AI > Unity MCP.")
		return
	}

	if errors.Is(err, context.DeadlineExceeded) {
		report.Advice = append(report.Advice, "If Unity shows a pending connection, approve it in Project Settings > AI > Unity MCP.")
		if diagnosticsSnapshot != nil && diagnosticsSnapshot.ActiveClientCount > 0 {
			report.Advice = append(report.Advice, "Another direct client may already be connected. Close it before retrying this CLI.")
		}
		return
	}

	if strings.Contains(strings.ToLower(err.Error()), "no unity mcp bridge found") {
		report.Advice = append(report.Advice, "Open the project in Unity and enable Unity MCP before retrying.")
		return
	}

	report.Advice = append(report.Advice, "Open Project Settings > AI > Unity MCP and verify that the bridge is enabled and ready.")
}

func evaluateWaitTarget(options connectionOptions, target string, requireLive bool) (bool, string) {
	switch target {
	case "status":
		resolved, err := options.resolve()
		if err != nil {
			return false, err.Error()
		}
		if resolved.StatusInfo != nil && strings.EqualFold(resolved.StatusInfo.Status, "ready") {
			return true, "bridge heartbeat is ready"
		}
		return false, "bridge heartbeat is not ready yet"
	case "bridge":
		resolved, err := options.resolve()
		if err == nil && resolved.StatusInfo != nil && strings.EqualFold(resolved.StatusInfo.Status, "ready") {
			return true, "bridge heartbeat is ready"
		}

		projectRoot := options.resolveProjectRoot()
		diagnosticsSnapshot, _, diagnosticsErr := diagnostics.LoadDiagnostics(projectRoot)
		if diagnosticsErr == nil && diagnosticsSnapshot.BridgeRunning {
			return true, "companion diagnostics report a running bridge"
		}

		if err != nil {
			return false, err.Error()
		}

		return false, "bridge is not ready yet"
	case "tools":
		if requireLive {
			bridge, cleanup, err := openBridgeClient(options, 5*time.Second)
			if err != nil {
				return false, err.Error()
			}
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			toolList, err := bridge.ListTools(ctx, "")
			if err != nil {
				return false, err.Error()
			}
			if len(toolList.Tools) == 0 {
				return false, "live probe returned zero tools"
			}
			return true, fmt.Sprintf("live bridge returned %d tools", len(toolList.Tools))
		}

		projectRoot := options.resolveProjectRoot()
		toolSnapshot, _, err := diagnostics.LoadToolSnapshot(projectRoot)
		if err != nil {
			return false, err.Error()
		}
		if len(toolSnapshot.Tools) == 0 {
			return false, "tool snapshot is empty"
		}
		return true, fmt.Sprintf("tool snapshot contains %d tools", len(toolSnapshot.Tools))
	default:
		return false, fmt.Sprintf("unsupported wait target %s", target)
	}
}
