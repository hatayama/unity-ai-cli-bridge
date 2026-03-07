# Repository Ground Rules

## Core Premise

- This repository is a development and verification project for the Unity-side environment.
- The CLI is distributed separately and must not be treated as being packaged from this Unity project.

## Unity AI Assistant Package

- `com.unity.ai.assistant` is managed by Unity.
- Files under `Library/PackageCache/com.unity.ai.assistant@...` are for inspection only.
- Do not treat `PackageCache` edits as implementation work.
- Do not rely on `PackageCache` edits as durable changes.

## Allowed Implementation Scope

- Add or modify project-local code under this repository, such as `Assets/`, `Packages/` owned by this project, tests, docs, and helper scripts.
- Use `com.unity.ai.assistant` public behavior and public APIs as integration points.
- If a problem appears to require changing Unity-managed package internals, treat it as a package limitation and design a workaround in project-local code or in the separately distributed CLI.

## Design Consequence

- Any plan that assumes direct modification of Unity-managed `com.unity.ai.assistant` internals is invalid for this repository.
- Changes in this repository must remain reproducible from tracked files only.

## Writing Language

- Code comments must be written in English.
- Commit messages must be written in English.
