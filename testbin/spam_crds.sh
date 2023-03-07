#!/usr/bin/env bash

actions=("deletePod" "createPod")
profiles=("balance-performance" "performance" "balance-power")

podList=()
deletePod(){
  if [ ${#podList[@]} -eq 0 ]; then
    return
  fi

  local pod=${podList[$RANDOM % ${#podList[@]} ]}
  echo deletePod $pod
  kubectl delete pod $pod &
  podList=("${podList[@]/$pod}")
}

createPod(){
  local podName=pod$RANDOM
  local CPUs=$((RANDOM%3+1))
  local profile=${profiles[ $RANDOM % ${#profiles[@]} ]}

  podList+=("$podName")

  cat << EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $podName
spec:
  containers:
  - name: example-power-container
    image: ubuntu
    command: ["/bin/sh"]
    args: ["-c", "sleep infinity"]
    resources:
      requests:
        memory: "200Mi"
        cpu: "$CPUs"
        power.intel.com/$profile: "$CPUs"
      limits:
        memory: "200Mi"
        cpu: "$CPUs"
        power.intel.com/$profile: "$CPUs"
EOF
}

count=1
MAXCOUNT=100

while [ "$count" -le $MAXCOUNT ]; do
  ${actions[ $RANDOM % ${#actions[@]} ]}
  sleep 2
done
