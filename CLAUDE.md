- all ts/js files should be kebab-case

## UI / shadcn Rules

- NEVER use `mr-2` or margin classes on icons inside shadcn Button components - shadcn already handles icon spacing automatically

## Development Commands

- `make run` - Build and run the server (use this for dev)
- `killall vibeflow-server && make run` - Restart server after code changes
- `make build` - Build both CLI and server binaries
- Server runs on http://localhost:8080