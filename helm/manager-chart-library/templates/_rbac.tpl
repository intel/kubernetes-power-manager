{{- define "manager-chart-library.operatorserviceaccount" -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Values.operatorserviceaccount.name }}
  namespace: {{ .Values.operatorserviceaccount.namespace }}

{{- end -}}

{{- define "manager-chart-library.agentserviceaccount" -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Values.agentserviceaccount.name }}
  namespace: {{ .Values.agentserviceaccount.namespace }}

{{- end -}}

{{- define "manager-chart-library.operatorrole" -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Values.operatorrole.name }}
  namespace: {{ .Values.operatorrole.namespace }}
rules:
- apiGroups: ["", "power.intel.com", "apps", "coordination.k8s.io"]
  resources: 
    {{ range .Values.operatorrole.resources }}
    - {{ . }}
    {{ end }}
  verbs: ["*"]
{{- if .Values.ocp }}
- apiGroups:
  - security.openshift.io
  resourceNames:
  - privileged
  resources:
  - securitycontextconstraints
  verbs:
  - use
{{- end -}}

{{- end -}}

{{- define "manager-chart-library.operatorrolebinding" -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Values.operatorrolebinding.name }}
  namespace: {{ .Values.operatorrolebinding.namespace }}
subjects:
- kind: ServiceAccount
  name: {{ .Values.operatorrolebinding.serviceaccount.name }}
  namespace: {{ .Values.operatorrolebinding.serviceaccount.namespace }}
roleRef:
  kind: Role
  name: {{ .Values.operatorrolebinding.rolename }}
  apiGroup: rbac.authorization.k8s.io

{{- end -}}

{{- define "manager-chart-library.operatorclusterrole" -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ .Values.operatorclusterrole.name }}
rules:
- apiGroups: ["", "power.intel.com", "apps"]
  resources: 
    {{ range .Values.operatorclusterrole.resources }}
    - {{ . }}
    {{ end }}
  verbs: ["*"]

{{- end -}}

{{- define "manager-chart-library.operatorclusterrolebinding" -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ .Values.operatorclusterrolebinding.name }}
subjects:
- kind: ServiceAccount
  name: {{ .Values.operatorclusterrolebinding.serviceaccount.name }}
  namespace: {{ .Values.operatorclusterrolebinding.serviceaccount.namespace }}
roleRef:
  kind: ClusterRole
  name: {{ .Values.operatorclusterrolebinding.clusterrolename }}
  apiGroup: rbac.authorization.k8s.io

{{- end -}}

{{- define "manager-chart-library.agentclusterrole" -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ .Values.agentclusterrole.name }}
rules:
- apiGroups: ["", "batch", "power.intel.com"]
  resources: 
    {{ range .Values.agentclusterrole.resources }}
    - {{ . }}
    {{ end }}
  verbs: ["*"]
{{- if .Values.ocp }}
- apiGroups:
  - security.openshift.io
  resourceNames:
  - privileged
  resources:
  - securitycontextconstraints
  verbs:
  - use
{{- end -}}

{{- end -}}

{{- define "manager-chart-library.agentclusterrolebinding" -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ .Values.agentclusterrolebinding.name }}
subjects:
- kind: ServiceAccount
  name: {{ .Values.agentclusterrolebinding.serviceaccount.name }}
  namespace: {{ .Values.agentclusterrolebinding.serviceaccount.namespace }}
roleRef:
  kind: ClusterRole
  name: {{ .Values.agentclusterrolebinding.clusterrolename }}
  apiGroup: rbac.authorization.k8s.io

{{- end -}}