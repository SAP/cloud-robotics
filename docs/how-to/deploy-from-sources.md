# Deploy Cloud Robotics Core from sources

Estimated time: 30 min

This page describes how to deploy Cloud Robotics Core from sources on your Kyma Cluster.

Please use a Linux shell like Ubuntu 20.04 LTS. You will need the following tools on your machine
- [git](https://git-scm.com/downloads)
- [docker](https://docs.docker.com/get-docker/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [helm](https://helm.sh/docs/intro/install/)

## Build and deploy the project

1. Get a Kubernetes cluster with Kyma or Istio

    Examples:
    - [SAP BTP Kyma](cluster-provisioning/Kyma@BTP.md)
    - [Kyma @ Gardener](cluster-provisioning/Kyma@Gardener.md)
    - [Istio @ Gardener](cluster-provisioning/Istio@Gardener.md)
    - [Istio @ GCP](cluster-provisioning/Kyma@GCP.md)

2. Clone the source repo.

    ```shell
    git clone https://github.com/SAP/cloud-robotics
    cd cloud-robotics
    ```

3. Create a Cloud Robotics config in your Kyma cluster:

   Before you deploy the images in your cluster for the first time you need to configure it using `make kubeconfig=<path of kubeconfig file of your cluster> set-deployment-config`.

   Please answer the configuration questions. Domain and Ingress IP of your cluster should be determined automatically and provided as default values.

   By default the images are pulled and pushed to the docker registry set in `.REGISTRY` file in the root directory of this repository on your computer. This file is not synchronized to github. 

   When you save the configuration and there is no `.REGISTRY` file on your computer, it will be created automatically using the registry you entered in the wizzard.

    Command summary:

    ```shell
    make kubeconfig=<path of kubeconfig file of your cluster> set-deployment-config
    ```

4. Build the project. Depending on your computer and internet connection, it may take around 15 minutes.

   Docker containers for core services are built in docker containers.

   If you have set a docker registry where the images are already existing in the previous step, you could skip this step.

   You can start building your docker images using `make docker-images`

   After a successfull build `make docker-push` pushes them to the registry.

   Command summary:

   ```shell
   make docker-images
   make docker-push
   ```

5. Deploy the cloud project.

   In order to deploy the core services use this command `make kubeconfig=<path of kubeconfig file of your cluster> create-deployment`.

   When your docker images are pushed and your cluster is configured correctly this deployment should finish automatically after a while.

   Command summary:

    ```shell
    make kubeconfig=<path of kubeconfig file of your cluster> create-deployment
    ```

After completing the deployment, you can list these components from the console on your workstation:

```shell
$ kubectl get pods
```

```shell
NAME                                          READY   STATUS    RESTARTS   AGE
cert-manager-57db9b7d88-gm6qv                 1/1     Running   0          64m
cert-manager-cainjector-55d7f64568-hctf2      1/1     Running   0          64m
cert-manager-webhook-7766b75f65-gj2w7         1/1     Running   0          64m
app-rollout-controller-64b7676d6-bdf6f        1/1     Running   0          59s
chart-assignment-controller-73a7f73d6-acf7e   1/1     Running   0          59s
```

More services are about to come 😀

With the project deployed, you're ready to [connect a robot to the cloud](connecting-robot.md).

## Update the project

To apply changes made in the source code, run:

```shell
make docker-images
make docker-push
make kubeconfig=<path of kubeconfig file of your cluster> update-deployment
```

## What's next

* [Connecting a robot to the cloud](connecting-robot.md).
* [Find out more about Logging](../concepts/logging-doc.md)
