#!/usr/bin/env bash

set -e

# Directory of this script
dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

package_chart() {
    chart_name=$1
    printf "\nPackaging: "$chart_name"\n"

    cd "$dir"
    helm lint ./subcharts/$chart_name
    helm package -u ./subcharts/$chart_name --destination "$dir/subcharts-packaged"

    printf "\n"
}

template_chart() {
    chart_name=$1
    chart_file=$2
    values_file=$3
    target_chart=$4
    namespace=$5
    printf "\nTemplating: "$chart_name"\n"

    cd "$dir"
    helm template --namespace $namespace $chart_name ./external-chart-templates/$chart_file \
      -f ./external-chart-templates/$values_file > ./subcharts/$target_chart/files/$chart_name.yaml

    printf "\n"
}

# Template charts
template_chart "prom" "kube-prometheus-stack-30.0.1.tgz" "prometheus-cloud.values.yaml" "prometheus-cloud" "\${HELM-NAMESPACE}"
template_chart "prom" "kube-prometheus-stack-30.0.1.tgz" "prometheus-robot.values.yaml" "prometheus-robot" "\${HELM-NAMESPACE}"

# Helm charts to be packaged
package_chart "k8s-relay-cloud"
package_chart "k8s-relay-robot"
package_chart "prometheus-cloud"
package_chart "prometheus-robot"