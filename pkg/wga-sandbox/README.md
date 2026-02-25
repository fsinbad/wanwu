# WGA Sandbox

沙箱容器交互包，支持在隔离环境中执行智能体任务。

## 架构

```
api.go
  ├── Run()      执行任务，返回输出流
  └── Cleanup()  延迟清理

sandbox.Manager
  ├── Create()   创建沙箱
  ├── Get()      获取实例
  └── Cleanup()  清理沙箱

runner.Runner
  ├── BeforeRun() 准备环境
  ├── Run()       执行任务
  └── AfterRun()  复制输出
```

## 沙箱模式

| 模式 | 说明 | 状态 |
|------|------|------|
| reuse | 复用已启动容器 | 完整实现 |
| oneshot | 每次启动新容器 | 接口定义 |

## 使用

```go
ctx := context.Background()

outputCh, _ := wga_sandbox.Run(ctx,
    wga_sandbox_option.WithModelConfig(wga_sandbox_option.ModelConfig{
        Provider:     "yuanjing",
        ProviderName: "YuanJing",
        BaseURL:      "https://maas-api.ai-yuanjing.com/openapi/compatible-mode/v1",
        APIKey:       "sk-xxx",
        Model:        "glm-5",
        ModelName:    "GLM-5",
    }),
    wga_sandbox_option.WithCurrentTask("生成一个 HTTP 服务器"),
)

for line := range outputCh {
    fmt.Println(line)
}
```

## JSON 格式

```go
outputCh, _ := wga_sandbox.Run(ctx,
    wga_sandbox_option.WithModelConfig(modelConfig),
    wga_sandbox_option.WithCurrentTask("任务描述"),
    wga_sandbox_option.WithOutputFormat(wga_sandbox_option.OutputFormatJSON),
)

for line := range outputCh {
    event, _ := wga_sandbox.ParseOpencodeEvent([]byte(line))
    switch event.Type {
    case wga_sandbox.OpencodeEventTypeText:
        part, _ := wga_sandbox.ParseOpencodeTextPart(event.Part)
        fmt.Println(part.Text)
    }
}
```

## AG-UI 协议

```go
import ag_ui_util "github.com/UnicomAI/wanwu/pkg/ag-ui-util"

outputCh, _ := wga_sandbox.Run(ctx,
    wga_sandbox_option.WithModelConfig(modelConfig),
    wga_sandbox_option.WithCurrentTask("任务描述"),
    wga_sandbox_option.WithOutputFormat(wga_sandbox_option.OutputFormatJSON),
    wga_sandbox_option.WithRunSession(wga_sandbox_option.RunSession{
        ThreadID: "thread-123",
        RunID:    "run-456",
    }),
)

tr := ag_ui_util.NewOpencodeTranslator("run-456", "thread-123")
eventCh := tr.TranslateStream(ctx, outputCh)
```

## API

- `Run(ctx, opts...)` - 执行任务
- `Cleanup(ctx, runID)` - 清理沙箱

## 选项

| 选项 | 说明 |
|------|------|
| `WithModelConfig` | 模型配置（必须） |
| `WithCurrentTask` | 当前任务（必须） |
| `WithRunSession` | 会话标识 |
| `WithInstruction` | 系统提示词 |
| `WithMessages` | 历史消息 |
| `WithInputDir` | 输入目录 |
| `WithOutputDir` | 输出目录 |
| `WithTools` | 工具列表 |
| `WithSkills` | 技能列表 |
| `WithOutputFormat` | 输出格式 |
| `WithEnableThinking` | 思考模式 |
| `WithSkipCleanup` | 跳过清理 |

## 依赖

- Docker 环境
- `wga-sandbox-wanwu` 容器（reuse 模式）
