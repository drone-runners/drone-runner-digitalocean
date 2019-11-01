# drone-runner-digitalocean

The digitalocean runner provisions and executes pipelines on a Digital Ocean droplet using the ssh protocol. A new droplet is provisioned for each pipeline execution, and then destroyed when the pipeline completes. This runner is intended for workloads that are not suitable for running inside containers. Drone server 1.4.0 or higher is required.

Documentation:<br/>
https://digitalocean-runner.docs.drone.io

Technical Support:<br/>
https://discourse.drone.io

Issue Tracker and Roadmap:<br/>
https://trello.com/b/ttae5E5o/drone
