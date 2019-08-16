---
date: 2000-01-01T00:00:00+00:00
title: Overview
author: bradrydzewski
weight: -2
toc: false
description: |
  Overview of Digital Ocean pipelines.
---

A `digitalocean` pipeline is a pipeline that executes shell commands on digitalocean droplets. The droplet is created when the pipeline starts and terminated upon completion. This is useful for workloads that need to run inside a dedicated virtual machine.

Example pipeline configuration:

{{< highlight text "linenos=table" >}}
---
kind: pipeline
type: digitalocean
name: default

token:
  from_secret: token

steps:
- name: greeting
  commands:
  - echo hello world

...
{{< / highlight >}}

The kind and type attributes define a digitalocean pipeline.

{{< highlight text "linenos=table" >}}
---
kind: pipline
type: digitalocean
{{< / highlight >}}

The steps section defines a series of shell commands. These commands are executed on the remote digital ocean droplet. If any command returns a non-zero exit code, the pipeline fails and exits.

{{< highlight text "linenos=table,linenostart=9" >}}
steps:
- name: greeting
  commands:
  - echo hello world
{{< / highlight >}}
