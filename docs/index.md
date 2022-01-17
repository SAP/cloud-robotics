# 🤖  Cloud Robotics 🤖 

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

### Documentation

* [Quickstart](): Set up Cloud Robotics from binaries (following soon)
* [Overview](overview.md): Develop a deeper understanding of Cloud Robotics.
* Concepts
    * Common: [Project configuration](concepts/config.md)
    * Layer 1: [Federation](concepts/federation.md), 
      [Device Identity](concepts/device_identity.md)
    * Layer 2: [Application Management](concepts/app-management.md)
* How-to guides
    * [Deploying Cloud Robotics from sources](how-to/deploy-from-sources.md)<br/>
      Build and deploy Cloud Robotics from the sources hosted on Github.
    * [Connecting a robot to the cloud](how-to/connecting-robot.md)<br/>
      Enable secure communication between a robot and the Cloud Kubernets Cluster.

### Are you looking for a Cloud Robotics use case?
Try executing SAP EWM warehouse orders on autonmous robots 🤖 with the apps of our [EWM Cloud Robotics repository](https://github.com/SAP/ewm-cloud-robotics/).
