package wga_option

// CheckResult 条件检查结果。
type CheckResult struct {
	Model          CheckModel          // 模型检查结果
	ToolCategories []CheckToolCategory // 工具类别检查结果
}

// CheckModel 模型检查结果。
type CheckModel struct {
	Meet bool // 是否满足条件
}

// CheckToolCategory 工具类别检查结果。
type CheckToolCategory struct {
	Category  string      // 工具类别类型
	Condition string      // 工具类别条件
	Meet      bool        // 是否满足条件
	Tools     []CheckTool // 工具检查结果
}

// CheckTool 工具检查结果。
type CheckTool struct {
	Title string // 工具标题
	Meet  bool   // 是否满足条件
}
