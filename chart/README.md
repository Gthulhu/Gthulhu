# Helm Charts for Gthulhu deployment


## Usage

The Gthulhu requires prometheus and grafana to be installed in your kubernetes cluster.

```bash
$  helm install  kube-prometheus-stack kube-prometheus-stack
```

To deploy Gthulhu using Helm charts, follow these steps:
```bash
$ cd gthulhu
$ helm install gthulhu . -f ./values-production.yaml --set mtls.enabled=true --set-file mtls.ca.cert=certs/ca.crt --set-file mtls.dm.cert=certs/dm.crt --set-file mtls.dm.key=certs/dm.key --set-file mtls.manager.cert=certs/manager.crt --set-file mtls.manager.key=certs/manager.key
```

To uninstall Gthulhu, run the following command:
```bash
$ helm uninstall gthulhu
```

## Testing

### API Manager

To access the Gthulhu API, you can set up port forwarding using kubectl:
```bash
$ kubectl port-forward svc/gthulhu-manager 8080:8080
```

After deploying Gthulhu, you can test the API by sending a login request using curl:
```bash
$ curl -X POST http://localhost:8080/api/v1/auth/login   -H "Content-Type: application/json"   -d '{
    "username": "admin@example.com",
    "password": "your-password-here"
  }'
{"success":true,"data":{"token":"<TOKEN>"},"timestamp":"2025-12-30T13:09:10Z"}
```

Then, you can create a new strategy by sending another curl request with the obtained token:

```bash
$ curl -X POST http://localhost:8080/api/v1/strategies \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{            
    "strategyNamespace": "default",
    "labelSelectors": [
      {"key": "app.kubernetes.io/name", "value": "prometheus"}
    ],
    "k8sNamespace": ["default"],
    "priority": 10,
    "executionTime": 20000000
  }'
```

You can also retrieve your own strategies using the following curl command:

```bash
$ curl http://localhost:8080/api/v1/strategies/self \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>"
```

To delete specific strategy(s), use the following command:

```bash
$ curl -X DELETE http://localhost:8080/api/v1/strategies   -H "Content-Type: application/json"   -H "Authorization: Bearer <TOKEN>"   -d '{
    "strategyId": "696f3a16b12be8ecfe9a6dc6"
  }'
```

To delete specific intent(s), use the following command:

```bash
$ curl http://localhost:8080/api/v1/intents/self   -H "Content-Type: application/json"   -H "Authorization: Bearer <TOKEN>"
{"success":true,"data":{"intents":[{"ID":"696f3a16b12be8ecfe9a6dc7","StrategyID":"696f3a16b12be8ecfe9a6dc6","PodID":"31e4e721-a5a0-421a-ae1d-b7971ae30d6e","NodeID":"myvm","K8sNamespace":"default","CommandRegex":"","Priority":10,"ExecutionTime":20000000,"PodLabels":{"app.kubernetes.io/instance":"kube-prometheus-stack-prometheus","app.kubernetes.io/managed-by":"prometheus-operator","app.kubernetes.io/name":"prometheus","app.kubernetes.io/version":"3.8.1","apps.kubernetes.io/pod-index":"0","controller-revision-hash":"prometheus-kube-prometheus-stack-prometheus-77c9dd5f65","operator.prometheus.io/name":"kube-prometheus-stack-prometheus","operator.prometheus.io/shard":"0","prometheus":"kube-prometheus-stack-prometheus","statefulset.kubernetes.io/pod-name":"prometheus-kube-prometheus-stack-prometheus-0"},"State":2}]},"timestamp":"2026-01-20T08:25:52Z"}
```

You can delete an intent by its ID using the following command:
> Please note that, the deletion of intents is not recommended unless for testing purposes, as it may lead to inconsistencies in the system.
> The reason is that intents are generated based on the strategies defined by users. Deleting an intent does not remove the corresponding strategy, which may result in the system attempting to recreate the deleted intent based on the existing strategy. This can lead to confusion and potential conflicts within the system, as the state of intents may not accurately reflect the strategies in place. Therefore, it is advisable to manage strategies directly rather than deleting intents to maintain system integrity and consistency.

```bash
curl -X DELETE http://localhost:8080/api/v1/intents   -H "Content-Type: application/json"   -H "Authorization: Bearer <TOKEN>"   -d '{
    "intentIds": ["696f3a16b12be8ecfe9a6dc7"]
  }'
```

### List all intents from decision maker

```bash
$ curl -X POST http://127.0.0.1:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAny28YMC2/+yYj3T29lz6\n0uryNz8gNVrqD7lTJuHQ3DMTE6ADqnERy8VgHve0tWzhJc5ZBZ1Hduvj+z/kNqbc\nU81YGhmfOrQ3iFNYBlSAseIHdAw39HGyC6OKzTXI4HRpc8CwcF6hKExkyWlkALr5\ni+IQDfimvarjjZ6Nm368L0Rthv3KOkI5CqRZ6bsVwwBug7GcdkvFs3LiRSKlMBpH\n2tCkZ5ZZE8VyuK7VnlwV7n6EHzN5BqaHq8HVLw2KzvibSi+/5wIZV2Yx33tViLbh\nOsZqLt6qQCGGgKzNX4TGwRLGAiVV1NCpgQhimZ4YP2thqSsqbaISOuvFlYq+QGP1\nbcvcHB7UhT1ZnHSDYcbT2qiD3VoqytXVKLB1X5XCD99YLSP9B32f1lvZD4MhDtE4\nIhAuqn15MGB5ct4yj/uMldFScs9KhqnWcwS4K6Qx3IfdB+ZxT5hEOWJLEcGqe/CS\nXITNG7oS9mrSAJJvHSLz++4R/Sh1MnT2YWjyDk6qeeqAwut0w5iDKWt7qsGEcHFP\nIVVlos+xLfrPDtgHQk8upjslUcMyMDTf21Y3RdJ3k1gTR9KHEwzKeiNlLjen9ekF\nWupF8jik1aYRWL6h54ZyGxwKEyMYi9o18G2pXPzvVaPYtU+TGXdO4QwiES72TNCD\nbNaGj75Gj0sN+LfjjQ4A898CAwEAAQ==\n-----END PUBLIC KEY-----",
    "client_id": "gthulhu-scheduler",
    "expired_at": 1736899200
  }'

$ curl 127.0.0.1:8080/api/v1/scheduling/strategies -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>"
```

### View Scheduler Sidecar (decision maker) Logs

```bash
$ kubectl logs gthulhu-scheduler-hqflq -c scheduler-sidecar
```

## Licence

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.