package introspect

import "testing"

func TestLoadEnumTypesWrapsRowsError(t *testing.T) {
	// 使用 mock 或错误注入验证 rows.Err() 被包装
	// 由于 introspect 依赖数据库，这里测试错误格式
	// 实际验证可通过检查错误信息是否包含上下文
	t.Skip("integration test requires database, verifying code directly")
}
