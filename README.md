# clusterplus

> 开放平台 K8s CRD

## 功能

- 部署应用
- 网络策略:重试，负载均衡，熔断，故障注入，限流等
- 蓝绿部署一键切换流量，进入指定蓝绿环境
- 全局接口级别限流，黑名单


## 使用

### 根据定义的 types 生成 K8s CRD
```shell
make manifests
```

### 将CRD安装到集群
```shell
make install
```

### 本地运行控制下
```shell
make run
```

### samples yaml
```yaml
apiVersion: apps.clusterplus.io/v1
kind: Plus
metadata:
  name: gateway
spec:
  gateway:
    cors:
      allowOrigins:
        - api-fat.tanjingmama.cn
    hosts:
      - api-fat.tanjingmama.cn
    route: # 指定路由进入环境
      blue:
        headersMatch:
          - MerchantId: "1"
            MerchantType: "type1"
      green:
        headersMatch:
          - MerchantId: "2"
            MerchantType: "type2"
    weights: # 默认路由权重
      blue: 50
      green: 50
  policy:
    timeout: 10s
    maxRequests: 10000
    retries:
      attempts: 3
      perTryTimeout: 3s
      retryOn: 5xx
    outlierDetection:
      consecutiveErrors: 5000
      ejectionPercent: 50
      ejectionTime: 30s
      interval: 10s
    fault:
      delay:
        percent: 0
        delay: 5s
      abort:
        percent: 0
        httpStatus: 500
  apps:
    - version: blue
      env:
        - name: VERSION
          value: blue_version
      image: xyctruth/plus-test:v0.0.3
      maxReplicas: 1
      minReplicas: 1
      port: 8080
      protocol: http
      resources:
        limits:
          cpu: 200m
          memory: 1Gi
        requests:
          cpu: 100m
          memory: 100Mi
    - version: green
      env:
        - name: VERSION
          value: green_version
      image: xyctruth/plus-test:v0.0.3
      maxReplicas: 1
      minReplicas: 1
      port: 8080
      protocol: http
      resources:
        limits:
          cpu: 200m
          memory: 1Gi
        requests:
          cpu: 100m
          memory: 100Mi
```

