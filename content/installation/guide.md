---
date: 2000-01-01T00:00:00+00:00
title: Installation Guide
title_in_header: Linux
author: bradrydzewski
weight: 1
toc: true
description: |
  Install the runner using Docker.
---

This article explains how to install the digitalocean runner on Linux using Docker. The digitalocean runner is packaged as a minimal Docker image distributed on [DockerHub](https://hub.docker.com/r/drone/drone-runner-digitalocean).

# Step 1 - Download

Install Docker and pull the public image:

```
$ docker pull drone/drone-runner-digitalocean
```

# Step 2 - Configure

The runner is configured using environment variables. This article references the below configuration options. See [Configuration]({{< relref "reference" >}}) for a complete list of configuration options.

`DRONE_RPC_HOST`
: provides the hostname (and optional port) of your Drone server. The runner connects to the server at the host address to receive pipelines for execution.

`DRONE_RPC_PROTO`
: provides the protocol used to connect to your Drone server. The value must be either http or https.

`DRONE_RPC_SECRET`
: provides the shared secret used to authenticate with your Drone server. This must match the secret defined in your Drone server configuration.

`DRONE_PUBLIC_KEY_FILE`
: provides the public key used for remote ssh access to the machine. This public key must also be added to your digital ocean account.

`DRONE_PRIVATE_KEY_FILE`
: provides the private key used for remote ssh access to the machine.

# Step 3 - Install

The below command creates the a container and start the runner. _Remember to replace the environment variables below with your Drone server details._

```
$ docker run -d \
  -v /path/on/host/id_rsa:/path/in/container/id_rsa \
  -v /path/on/host/id_rsa.pub:/path/in/container/id_rsa.pub \
  -e DRONE_RPC_PROTO=https \
  -e DRONE_RPC_HOST=drone.company.com \
  -e DRONE_RPC_SECRET=super-duper-secret \
  -e DRONE_PUBLIC_KEY_FILE=/path/in/container/id_rsa.pub \
  -e DRONE_PRIVATE_KEY_FILE=/path/in/container/id_rsa \
  -p 3000:3000 \
  --restart always \
  --name runner \
  drone/drone-runner-digitalocean
```

# Step 4 - Verify

Use the `docker logs` command to view the logs and verify the runner successfully established a connection with the Drone server.

```
$ docker logs runner

INFO[0000] starting the server
INFO[0000] successfully pinged the remote server 
```