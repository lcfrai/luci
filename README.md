# Panacea App

A desktop cockpit that streams real AI coding-agent CLI sessions from cloud microVMs.

## Structure

- `sandboxd/` - Sandbox daemon service
- `control-plane/` - Control plane API service

## Building

```bash
go build ./sandboxd/...
go build ./control-plane/...
```

## Testing

```bash
go test ./sandboxd/...
go test ./control-plane/...
```

## CI/CD

See `azure-pipelines.yml` for the continuous integration pipeline.
