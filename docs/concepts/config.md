# Project configuration

The project configuration that one has entered during the initial setup are
stored with the project in a Kubernetes configmap. One can look at the options with the following
command:

```shell
kubectl get configmaps -n default cloud-robotics-core-config -o yaml
```

The settings contained in the config file are used by the cloud and chart-assignment-controller services running
in kubernetes to configure apps.

To support configuring apps, we pass the settings to app-rollout-controller where they are
provided as additional variables for helm templating. The command below prints
the settings we pass to app-rollout-controller:

```shell
kubectl get deployment app-rollout-controller -o=jsonpath='{.spec.template.spec.containers[0].args[0]}'
```

