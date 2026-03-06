# Kubernetes 监控和管理指南

## 概述

本指南提供了在 Kubernetes 上监控和管理 demo_cluster 的方法。

## 基础监控

### 1. 查看 Pod 状态

```bash
# 查看所有 Pod
kubectl get pods -n demo-cluster

# 查看 Pod 详细信息
kubectl get pods -n demo-cluster -o wide

# 查看 Pod 的 YAML 配置
kubectl get pod <pod-name> -n demo-cluster -o yaml

# 查看 Pod 的 JSON 配置
kubectl get pod <pod-name> -n demo-cluster -o json

# 实时监控 Pod 状态
kubectl get pods -n demo-cluster --watch
```

### 2. 查看资源使用

```bash
# 查看节点资源使用
kubectl top nodes

# 查看 Pod 资源使用
kubectl top pods -n demo-cluster

# 查看 Pod 详细资源使用
kubectl top pods -n demo-cluster --containers

# 查看所有命名空间的 Pod 资源使用
kubectl top pods --all-namespaces
```

### 3. 查看日志

```bash
# 查看 Pod 日志
kubectl logs <pod-name> -n demo-cluster

# 实时查看日志
kubectl logs -f <pod-name> -n demo-cluster

# 查看前一个容器的日志
kubectl logs <pod-name> -n demo-cluster --previous

# 查看最后 100 行日志
kubectl logs --tail=100 <pod-name> -n demo-cluster

# 查看最后 1 小时的日志
kubectl logs --since=1h <pod-name> -n demo-cluster

# 查看特定时间范围的日志
kubectl logs --since-time=2024-01-21T10:00:00Z <pod-name> -n demo-cluster

# 查看多个 Pod 的日志
kubectl logs -f -l app=web -n demo-cluster

# 查看所有容器的日志
kubectl logs <pod-name> -n demo-cluster --all-containers=true
```

### 4. 查看事件

```bash
# 查看所有事件
kubectl get events -n demo-cluster

# 查看特定 Pod 的事件
kubectl describe pod <pod-name> -n demo-cluster

# 实时查看事件
kubectl get events -n demo-cluster --watch

# 查看事件详情
kubectl describe event <event-name> -n demo-cluster
```

### 5. 查看服务和端点

```bash
# 查看服务
kubectl get svc -n demo-cluster

# 查看服务详情
kubectl describe svc <service-name> -n demo-cluster

# 查看端点
kubectl get endpoints -n demo-cluster

# 查看端点详情
kubectl describe endpoints <endpoint-name> -n demo-cluster
```

## 高级监控

### 1. 使用 Kubernetes Dashboard

#### 启动 Dashboard（Minikube）

```bash
# 启动 Dashboard
minikube dashboard

# 或手动启动
kubectl proxy
# 访问: http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/
```

#### 部署 Dashboard（其他集群）

```bash
# 部署 Dashboard
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml

# 创建代理用户
kubectl create serviceaccount admin-user -n kubernetes-dashboard
kubectl create clusterrolebinding admin-user --clusterrole=cluster-admin --serviceaccount=kubernetes-dashboard:admin-user

# 获取 Token
kubectl -n kubernetes-dashboard create token admin-user

# 启动代理
kubectl proxy

# 访问 Dashboard
# http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/
```

### 2. 使用 Prometheus 和 Grafana

#### 安装 Prometheus

```bash
# 添加 Prometheus Helm 仓库
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# 安装 Prometheus
helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring --create-namespace

# 查看 Prometheus 服务
kubectl get svc -n monitoring

# 端口转发
kubectl port-forward svc/prometheus-kube-prometheus-prometheus 9090:9090 -n monitoring

# 访问 Prometheus
# http://localhost:9090
```

#### 安装 Grafana

```bash
# Grafana 通常与 Prometheus 一起安装
# 获取 Grafana 密码
kubectl get secret -n monitoring prometheus-grafana -o jsonpath="{.data.admin-password}" | base64 --decode

# 端口转发
kubectl port-forward svc/prometheus-grafana 3000:80 -n monitoring

# 访问 Grafana
# http://localhost:3000
# 用户名: admin
# 密码: <上面获取的密码>
```

### 3. 使用 ELK Stack 进行日志收集

#### 安装 Elasticsearch

```bash
# 添加 Elastic Helm 仓库
helm repo add elastic https://helm.elastic.co
helm repo update

# 安装 Elasticsearch
helm install elasticsearch elastic/elasticsearch -n logging --create-namespace

# 查看 Elasticsearch 服务
kubectl get svc -n logging
```

#### 安装 Kibana

```bash
# 安装 Kibana
helm install kibana elastic/kibana -n logging

# 端口转发
kubectl port-forward svc/kibana-kibana 5601:5601 -n logging

# 访问 Kibana
# http://localhost:5601
```

#### 安装 Filebeat

```bash
# 安装 Filebeat
helm install filebeat elastic/filebeat -n logging

# 验证 Filebeat 运行
kubectl get pods -n logging
```

## 性能监控

### 1. CPU 和内存使用

```bash
# 查看 Pod CPU 和内存使用
kubectl top pods -n demo-cluster

# 查看节点 CPU 和内存使用
kubectl top nodes

# 查看详细的资源使用
kubectl describe node <node-name>
```

### 2. 网络监控

```bash
# 查看网络策略
kubectl get networkpolicies -n demo-cluster

# 查看网络流量（需要安装 Cilium 或其他 CNI）
# 这通常需要专门的网络监控工具
```

### 3. 存储监控

```bash
# 查看 PVC 使用
kubectl get pvc -n demo-cluster

# 查看 PV 使用
kubectl get pv

# 查看存储类
kubectl get storageclass

# 进入 Pod 检查磁盘使用
kubectl exec -it <pod-name> -n demo-cluster -- df -h
```

## 告警和通知

### 1. 配置 Prometheus 告警

创建 `prometheus-rules.yaml`：

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: demo-cluster-alerts
  namespace: monitoring
spec:
  groups:
  - name: demo-cluster
    interval: 30s
    rules:
    - alert: PodCrashLooping
      expr: rate(kube_pod_container_status_restarts_total{namespace="demo-cluster"}[15m]) > 0.1
      for: 5m
      annotations:
        summary: "Pod {{ $labels.pod }} is crash looping"
    
    - alert: HighMemoryUsage
      expr: container_memory_usage_bytes{namespace="demo-cluster"} / container_spec_memory_limit_bytes > 0.9
      for: 5m
      annotations:
        summary: "Pod {{ $labels.pod }} memory usage is high"
    
    - alert: HighCPUUsage
      expr: rate(container_cpu_usage_seconds_total{namespace="demo-cluster"}[5m]) > 0.8
      for: 5m
      annotations:
        summary: "Pod {{ $labels.pod }} CPU usage is high"
    
    - alert: PodNotReady
      expr: kube_pod_status_ready{namespace="demo-cluster",condition="false"} == 1
      for: 5m
      annotations:
        summary: "Pod {{ $labels.pod }} is not ready"
```

### 2. 配置告警通知

编辑 `alertmanager-config.yaml`：

```yaml
global:
  resolve_timeout: 5m

route:
  receiver: 'default'
  group_by: ['alertname', 'cluster', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h

receivers:
- name: 'default'
  # 配置通知方式（邮件、Slack、钉钉等）
  email_configs:
  - to: 'admin@example.com'
    from: 'alertmanager@example.com'
    smarthost: 'smtp.example.com:587'
    auth_username: 'alertmanager@example.com'
    auth_password: 'password'
```

## 日志管理

### 1. 查看应用日志

```bash
# 查看 Web 应用日志
kubectl logs -f -l app=web -n demo-cluster

# 查看 Center 应用日志
kubectl logs -f -l app=center -n demo-cluster

# 查看 Gate 应用日志
kubectl logs -f -l app=gate -n demo-cluster

# 查看 Game 应用日志
kubectl logs -f -l app=game -n demo-cluster
```

### 2. 日志聚合

```bash
# 使用 stern 查看多个 Pod 的日志
# 安装 stern: brew install stern

# 查看所有 Pod 的日志
stern . -n demo-cluster

# 查看特定应用的日志
stern web -n demo-cluster

# 查看特定时间范围的日志
stern web -n demo-cluster --since 1h
```

### 3. 日志导出

```bash
# 导出 Pod 日志到文件
kubectl logs <pod-name> -n demo-cluster > pod.log

# 导出所有 Pod 日志
for pod in $(kubectl get pods -n demo-cluster -o name); do
  kubectl logs $pod -n demo-cluster > ${pod##*/}.log
done
```

## 性能优化

### 1. 资源限制

编辑部署配置，添加资源限制：

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

### 2. 自动扩展

创建 HPA（Horizontal Pod Autoscaler）：

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: web-hpa
  namespace: demo-cluster
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### 3. Pod 优先级

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: high-priority
value: 1000
globalDefault: false
description: "High priority for critical services"
```

## 备份和恢复

### 1. 备份数据库

```bash
# 备份 PostgreSQL
kubectl exec -it <postgres-pod> -n demo-cluster -- pg_dump -U postgres demo_cluster > backup.sql

# 备份所有数据库
kubectl exec -it <postgres-pod> -n demo-cluster -- pg_dumpall -U postgres > backup-all.sql
```

### 2. 恢复数据库

```bash
# 恢复数据库
kubectl exec -it <postgres-pod> -n demo-cluster -- psql -U postgres demo_cluster < backup.sql

# 恢复所有数据库
kubectl exec -it <postgres-pod> -n demo-cluster -- psql -U postgres < backup-all.sql
```

### 3. 备份 ETCD

```bash
# 备份 ETCD
kubectl exec -it <etcd-pod> -n demo-cluster -- etcdctl snapshot save /tmp/etcd-backup.db

# 从备份恢复
kubectl exec -it <etcd-pod> -n demo-cluster -- etcdctl snapshot restore /tmp/etcd-backup.db
```

## 常用命令速查

```bash
# 查看 Pod 状态
kubectl get pods -n demo-cluster

# 查看 Pod 详情
kubectl describe pod <pod-name> -n demo-cluster

# 查看 Pod 日志
kubectl logs -f <pod-name> -n demo-cluster

# 进入 Pod
kubectl exec -it <pod-name> -n demo-cluster -- sh

# 查看资源使用
kubectl top pods -n demo-cluster

# 查看事件
kubectl get events -n demo-cluster

# 查看服务
kubectl get svc -n demo-cluster

# 查看存储
kubectl get pvc -n demo-cluster

# 端口转发
kubectl port-forward svc/<service-name> <local-port>:<remote-port> -n demo-cluster

# 扩展副本
kubectl scale deployment <deployment-name> --replicas=3 -n demo-cluster

# 更新镜像
kubectl set image deployment/<deployment-name> <container-name>=<image>:<tag> -n demo-cluster

# 查看部署历史
kubectl rollout history deployment/<deployment-name> -n demo-cluster

# 回滚部署
kubectl rollout undo deployment/<deployment-name> -n demo-cluster
```

## 相关工具

- **Kubernetes Dashboard** - Web UI 管理工具
- **kubectl** - 命令行工具
- **Prometheus** - 监控和告警
- **Grafana** - 可视化仪表板
- **ELK Stack** - 日志收集和分析
- **Jaeger** - 分布式追踪
- **Kube-state-metrics** - Kubernetes 状态指标
- **Metrics Server** - 资源指标 API

## 参考资源

- [Kubernetes 官方文档](https://kubernetes.io/docs/)
- [Prometheus 文档](https://prometheus.io/docs/)
- [Grafana 文档](https://grafana.com/docs/)
- [ELK Stack 文档](https://www.elastic.co/guide/index.html)
