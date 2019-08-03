# drone-runner-digitalocean

The digitalocean runner provisions and executes pipelines on a Digital Ocean droplet using the ssh protocol. A new droplet is provisioned for each pipeline execution, and then destroyed when the pipeline completes. This runner is intended for workloads that are not suitable for running inside containers. Drone server 1.2.1 or higher is required.
