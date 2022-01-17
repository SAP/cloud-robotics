# Kyma@Gardener

## Get Gardener cluster

## Deploy Kyma with default profile
https://kyma-project.io/docs/kyma/latest/04-operation-guides/operations/02-install-kyma/

Restrict kyma prometheus operator to kyma-system namespace, that cloud-robotics and kyma prometheus operators do not influence each other

```
kyma deploy --value monitoring.prometheusOperator.namespaces.releaseNamespace=true
```


## Deploy cloud-robotics

[Deploying Cloud Robotics from sources](../deploy-from-sources.md)
