# Issue: Set Up Automated Docker Image Publishing to ghcr.io

## Summary

Configure CI/CD to automatically publish Docker images to GitHub Container Registry (ghcr.io) when building on the main branch.

## Current State

- CI builds Docker images but does not push them anywhere
- Documentation references `ghcr.io/sipico/bunny-api-proxy:latest`
- Images are not actually available to pull

## Requirements

1. **Push to ghcr.io on main branch merges**
   - Image: `ghcr.io/sipico/bunny-api-proxy:latest`
   - Only on successful CI (tests pass, lint passes)

2. **Tag-based versioning** (future)
   - When a git tag is pushed (e.g., `v1.0.0`)
   - Push tagged image: `ghcr.io/sipico/bunny-api-proxy:v1.0.0`
   - Also update `:latest` tag

3. **Security**
   - Use GitHub's built-in `GITHUB_TOKEN` for authentication
   - No external secrets needed for ghcr.io

## Implementation

Update `.github/workflows/ci.yml` to add a publish job:

```yaml
publish:
  name: Publish Docker Image
  runs-on: ubuntu-latest
  needs: [test, lint, docker]  # Only after all checks pass
  if: github.ref == 'refs/heads/main' && github.event_name == 'push'
  permissions:
    contents: read
    packages: write
  steps:
    - uses: actions/checkout@v4

    - name: Log in to GitHub Container Registry
      uses: docker/login-action@v3
      with:
        registry: ghcr.io
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}

    - name: Build and push
      uses: docker/build-push-action@v5
      with:
        context: .
        push: true
        tags: ghcr.io/sipico/bunny-api-proxy:latest
```

## Acceptance Criteria

- [ ] Docker images are automatically pushed to ghcr.io on main branch merges
- [ ] `docker pull ghcr.io/sipico/bunny-api-proxy:latest` works
- [ ] Images are publicly accessible (no auth required to pull)
- [ ] CI workflow shows publish step

## Labels

- `ci-cd`
- `docker`
- `infrastructure`
