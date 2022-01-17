# 🤖  Cloud Robotics 🤖 
[![REUSE status](https://api.reuse.software/badge/github.com/SAP/cloud-robotics)](https://api.reuse.software/info/github.com/SAP/cloud-robotics)

This Cloud Robotics version is our adaption of the open source
[Google Cloud Robotics](https://github.com/googlecloudrobotics/core) platform that provides
infrastructure essential to building and running robotics solutions for business
automation. Cloud Robotics Core makes managing robot fleets easy for developers,
integrators, and operators. It enables:

* packaging and distribution of applications
* secure, bidirectional robot-cloud communication
* easy integration into SAP BTP Kyma infrastructure.

We consider it as a friendly fork of Google Cloud Robotics and try to keep it as close as possible to it. However there are some differences:
- This version does not Google Cloud Platform services
- We introduced multi-tenancy features
- Some components we did not use have been removed
- There are docker builds instead of bazel
- Cluster provisiong is separated from deployment of Cloud Robotics. In fact there is no automated cluster provisioning at the moment.

# Documentation

Documentation of the applications and their architecture can be found at:
https://sap.github.io/cloud-robotics/

# Source Code

Most interesting bits are under `src` and `charts`:
* `src/go`: the code that goes into images referenced from `charts`
* `charts` contains kubernetes resources for the core platform and apps

# Deployment

We built and tested Cloud Robotics using [Kyma](https://kyma-project.io/) k8s clusters where it naturally runs best.
However most of our functions are working using [Kubernetes](https://kubernetes.io/) plus [Istio](https://istio.io/) only.

We Cloud Robotics our deployment on these Kubernetes infrastructure so far:
- SAP BTP Kyma
- Kyma @ Gardener
- Istio @ Gardener
- Istio @ GCP

For getting started please see the [deploy-from-sources how-to](docs/how-to/deploy-from-sources.md).

# Setting up and connecting robot clusters

Please see the [connecting-robot how-to](docs/how-to/connecting-robot.md).

# Are you looking for a Cloud Robotics use case?
Try executing SAP EWM warehouse orders on autonmous robots 🤖 with the apps of our [EWM Cloud Robotics repository](https://github.com/SAP/ewm-cloud-robotics/).

# Get involved
You are welcome to join forces with us in the quest to ease deploying applications on 🤖! Simply check the [Contribution Guidelines](CONTRIBUTING.md).
