1. Install go 1.12 or later
2. Test

    go test ./...

3. Build executables

    sh scripts/build.sh

4. Build images

    docker build -t drone/drone-runner-digitalocean:latest-linux-amd64 -f docker/Dockerfile.linux.amd64 .
    docker build -t drone/drone-runner-digitalocean:latest-linux-arm64 -f docker/Dockerfile.linux.arm64 .
    docker build -t drone/drone-runner-digitalocean:latest-linux-arm   -f docker/Dockerfile.linux.arm   .
