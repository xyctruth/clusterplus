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

### 本地运行控制器
```shell
make run
```

### 部署
```shell
make install
make deploy IMG=docker.io/xyctruth/clusterplus:v0.1.31
```

### samples yaml

[samples config file](./config/samples)
