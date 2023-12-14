{{- define "manager-chart-library.new-shared-profile" -}}
apiVersion: power.intel.com/v1
kind: PowerProfile
metadata:
  name: {{ .Values.sharedprofile.name }}
  namespace: {{ .Values.sharedprofile.namespace }}
spec:
  name: {{ .Values.sharedprofile.spec.name }}
  max: {{ .Values.sharedprofile.spec.max }}
  min: {{ .Values.sharedprofile.spec.min }}
  epp: {{ .Values.sharedprofile.spec.epp }}
  governor: {{ .Values.sharedprofile.spec.governor }}
  shared: true
{{- end -}}