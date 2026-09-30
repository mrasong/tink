> I've  a github repo  mrasong/tink, it's a golang server service, now , I want to use the github package (aka ghcr.io), how can I do? is this free for me ?

---

Yes. For your Go server `mrasong/tink`, **GitHub Container Registry (GHCR)** is a very good fit.

Your image can be:

```text
ghcr.io/mrasong/tink:latest
```

or preferably versioned:

```text
ghcr.io/mrasong/tink:v1.0.0
```

### Is it free?

**If the container package is public: yes, effectively free.** GitHub explicitly says public GitHub Packages are free, and Container Registry storage/bandwidth is currently free. Public GHCR images can also be pulled anonymously. :chatgpt-content-reference{index="0"}

So for an open-source `mrasong/tink`, I would absolutely use GHCR.

---

## Recommended setup for `mrasong/tink`

I'd recommend letting **GitHub Actions automatically build and publish the image** whenever you create a release/tag.

Your repository:

```text
mrasong/tink
├── cmd/
├── internal/
├── go.mod
├── go.sum
├── Dockerfile
└── .github/
    └── workflows/
        └── docker.yml
```

### 1. Create a Dockerfile

For a Go server, something like:

```dockerfile
# Build
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /tink \
    ./cmd/tink

# Runtime
FROM alpine:3.22

RUN apk add --no-cache ca-certificates

COPY --from=builder /tink /usr/local/bin/tink

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/tink"]
```

Adjust `./cmd/tink` and port `8080` to match your actual project.

---

## 2. Add GitHub Actions

Create:

```text
.github/workflows/docker.yml
```

with:

```yaml
name: Docker

on:
  push:
    branches:
      - main
    tags:
      - "v*"

permissions:
  contents: read
  packages: write

jobs:
  docker:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ghcr.io/mrasong/tink
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=ref,event=tag
            type=sha

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
```

GitHub specifically supports using `GITHUB_TOKEN` from Actions to publish a package associated with the workflow repository. :chatgpt-content-reference{index="1"}

---

## 3. Push your first version

For example:

```bash
git add .
git commit -m "Add Docker image"
git push origin main
```

GitHub Actions will build:

```text
ghcr.io/mrasong/tink:latest
```

Then:

```bash
docker pull ghcr.io/mrasong/tink:latest
```

If you create:

```bash
git tag v1.0.0
git push origin v1.0.0
```

you'll also get:

```text
ghcr.io/mrasong/tink:v1.0.0
```

And the SHA tag, e.g.:

```text
ghcr.io/mrasong/tink:sha-abc1234
```

---

# Even better: multi-architecture

Since you use Mac/Apple Silicon, I'd strongly recommend building both:

```text
linux/amd64
linux/arm64
```

Then the same command works on both Intel servers and ARM Macs:

```bash
docker pull ghcr.io/mrasong/tink:latest
```

Docker automatically selects the correct architecture.

Change the workflow's build step to:

```yaml
- name: Set up QEMU
  uses: docker/setup-qemu-action@v3

- name: Set up Buildx
  uses: docker/setup-buildx-action@v3

- name: Build and push
  uses: docker/build-push-action@v6
  with:
    context: .
    push: true
    platforms: linux/amd64,linux/arm64
    tags: ${{ steps.meta.outputs.tags }}
    labels: ${{ steps.meta.outputs.labels }}
```

I'd choose this approach for `tink`.

---

## Then deployment becomes very simple

For example your server can have:

```yaml
services:
  tink:
    image: ghcr.io/mrasong/tink:latest
    container_name: tink
    restart: unless-stopped
    ports:
      - "8080:8080"
```

Then upgrading is simply:

```bash
docker compose pull tink
docker compose up -d tink
```

Or, if you want reproducible production deployments:

```yaml
image: ghcr.io/mrasong/tink:v1.2.0
```

and upgrade explicitly:

```bash
docker compose pull
docker compose up -d
```

---

### One important thing

When you first publish the package, GitHub's default visibility is **private**. You'll want to go to the package settings and make the `tink` container **Public** if you want anyone to be able to:

```bash
docker pull ghcr.io/mrasong/tink
```

without logging in. Public Container Registry images support anonymous pulls. :chatgpt-content-reference{index="2"}

**For your project, I'd use:**

```text
GitHub repo
    ↓
GitHub Actions
    ↓
Docker Buildx
    ↓
GHCR
    ↓
ghcr.io/mrasong/tink
    ├── latest
    ├── v1.0.0
    ├── v1.1.0
    └── ...
```

This gives you a clean **GitHub → CI → GHCR → server** pipeline with essentially no registry cost for a public image. :chatgpt-content-reference{index="3"}

If you want, I can also give you a **:chatgpt-content-reference{index="5"}** with `Dockerfile + docker-compose.yml + GitHub Actions + semantic version tags + amd64/arm64 + automatic GitHub Release`, tailored to the actual structure of `mrasong/tink`.
