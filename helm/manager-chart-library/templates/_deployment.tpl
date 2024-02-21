{{- define "manager-chart-library.deployment" -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.operator.name }}
  namespace: {{ .Values.operator.namespace }}
  labels:
    control-plane: {{ .Values.operator.labels.controlplane }}
spec:
  selector:
    matchLabels:
      control-plane: {{ .Values.operator.labels.controlplane }}
  replicas: {{ .Values.operator.replicas }}
  template:
    metadata:
      labels:
        control-plane: {{ .Values.operator.labels.controlplane }}
    spec:
      serviceAccountName: {{ .Values.operator.container.serviceaccount.name }}
      containers:
      - command:
        - {{ .Values.operator.container.command }}
        args:
        - {{ .Values.operator.container.args }}
        imagePullPolicy: IfNotPresent
        image: {{ .Values.operator.container.image }}
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
        name: {{ .Values.operator.container.name }}
        resources:
          limits:
            cpu: {{ .Values.operator.container.cpu.limits }}
            memory: {{ .Values.operator.container.memory.limits }}
          requests:
            cpu: {{ .Values.operator.container.cpu.requests }}
            memory: {{ .Values.operator.container.memory.requests }}
        volumeMounts:
        - mountPath: /sys/fs
          name: cgroup
          mountPropagation: HostToContainer
          readOnly: true
      terminationGracePeriodSeconds: 10
      volumes:
      - name: cgroup
        hostPath:
          path: /sys/fs

{{- end -}}