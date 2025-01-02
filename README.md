# clouditera-sdk

clouditera-sdk是clouditera内部封装的go SDK仓库。

## 使用方法

```bash
export GOPRIVATE=source.gitlab.clouditera.com
export GOINSECURE=source.gitlab.clouditera.com
go get source.gitlab.clouditera.com/clouditera/go-common-sdk@v0.0.5
```

## 更新日志

- v0.0.7 (2025年1月2日): 增加QueryOption的Select和Table选项
- v0.0.6 (2025年1月2日): 增加QueryConds的Sql方法
- v0.0.5 (2024年12月25日): 增加QueryConds对多参数的支持
