{{- define "tenant-namespaces" -}}
{{- if .Values.tenant_namespaces }}
{{- range .Values.tenant_namespaces }}{{ . }},{{- end }}
{{- end }}
{{- end }}

{{- define "prometheus-domain" -}}
{{- if .Values.tenant_domain }}prom.{{ .Values.tenant_domain }}{{ else }}{{ .Values.tenant_main_namespace }}-prom.{{ .Values.domain }}{{ end }}
{{- end -}}
