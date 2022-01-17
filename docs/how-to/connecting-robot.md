# Connecting a robot to the cloud

Estimated time: 10 min

This page describes how to connect a Kubernetes cluster on a robot running Ubuntu 20.04 to the cloud.

Once you've done this, you can:

* Run a private Docker container from a container registry of your choice
* Securely communicate with cloud services

## Setting up the BTP project

1. If you haven't already, complete the [deploy-from-sources](deploy-from-sources.md) steps.

1. On the computer you used to set up the project, find out the domain name of your Kyma Kubernetes cluster:

    ```shell
    export KUBECONFIG=<path-to-kubeconfig-of-kyma-cluster>
    kubectl get configmaps -n robot-config robot-setup -o=go-template --template='{{index .data "domain"}}'
    ```

## Installing the cluster on the robot

You'll need to install a Kubernetes cluster on the robot before you can connect it to the cloud. The cluster manages and supports the processes that communicate with the cloud.

The installation script installs and configures:

* Docker
* A single-node Kubernetes cluster (packages: kubectl, kubeadm, kubelet)

<!-- this comment is required to separate the lists -->

1. Download and run install\_k8s\_on\_robot.sh. This script will take a few minutes as it downloads and installs the dependencies of the Kubernetes cluster.

    ```shell
    curl https://setup-robot.<your-kyma-cloud-cluster-domain>/install_k8s_on_robot.sh | bash
    [...]
    The local Kubernetes cluster has been installed.
    ```

    After the script successfully finishes, the Kubernetes cluster is up and running.
    
    > **Note:**  At the end of the script output, you might notice instructions for creating `~/.kube/config`, deploying a pod network, and joining nodes to the cluster. You can ignore these instructions for now, as the script has already set up a single-node cluster.

    In case you installed docker for the first time on the robot, please log off and on again that the assignment of your user to docker group is loaded.

1. Set up the robot cluster to connect to the cloud. You may find it easiest if you SSH into the robot from the workstation you used to set up the project.

    ```shell
    curl https://setup-robot.<your-kyma-cloud-cluster-domain>/setup_robot.sh > setup_robot.sh
    bash setup_robot.sh my-robot --tenant default --robot-type my-robot-type --labels "model=my-robot-label"
    ```

    > **Note:** `my-robot-type` is a placeholder and you can ignore it for now.

## What's next

* tbd

## Uninstalling the local cluster

You can remove the local cluster with the following command:

```shell
sudo kubeadm reset
```

You may also want to remove the following APT packages and repositories, which `install_k8s_on_robot.sh` installs if they are not present:

```shell
sudo apt-get purge kubectl kubelet kubeadm
sudo rm /etc/apt/sources.list.d/kubernetes.list
sudo apt-get purge docker-ce
sudo add-apt-repository --remove \
  "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
```
