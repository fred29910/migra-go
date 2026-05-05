  1. 性能优化

  1. Renderer 有状态累积，重复调用会不断增长 r.sql。
     问题位置：internal/render/render.go:34
     建议：RenderAll 开始时重置切片，避免长生命周期对象内存持续占用。

  // before
  for _, op := range ops { ... r.sql = append(r.sql, sql) }

  // after
  r.sql = r.sql[:0]
  for _, op := range ops { ... }

  2. 热路径重复创建 map，产生不必要分配。
     问题位置：internal/parser/parser.go:260, internal/normalize/normalize.go:119
     建议：将类型别名表提升为包级只读变量。
  3. Diff 对列每次构建临时 map，表大时有额外开销。
     问题位置：internal/diff/diff_tables.go:43
     建议：Table 增加 ColumnByName map[string]*Column（保留 Columns 顺序），构建时维护索引。

  2. 代码质量与可维护性

  1. “声明支持 normalize”但主流程未调用，导致误报 diff。
     问题位置：cmd/migra/diff.go:66
     建议：真正接入 normalize.CanonicalizeSchema，并在失败时返回错误。
  2. 关键逻辑存在注释与实现不一致。
     问题位置：internal/parser/parser.go:143
     说明：注释写“ALTER TABLE 前置时创建表”，但 schema 不存在时直接报错。
     建议：统一语义（要么明确 fail-fast，要么自动 GetOrCreateNamespace）。
  3. 多处字符串解析脆弱，遇表达式/函数索引会误解析。
     问题位置：internal/introspect/indexes.go:63, internal/introspect/constraints.go:81
     建议：尽量从 pg_catalog 获取结构化字段，少做 strings.Split 式 SQL 文本解析。

  3. Idiomatic Go

  1. 忽略错误返回，不符合 Go 错误处理习惯。
     问题位置：cmd/migra/main.go:25
     建议：检查 viper.BindPFlag 返回值并在启动时报错/panic。
  2. parseExpression 返回空串也被当作有效默认值指针，语义不清。
     问题位置：internal/parser/parser.go:273
     建议：改为 (string, bool)，仅 ok=true 时写入 DefaultExpr。
  3. isSQLFile 用手工下标判断后缀，可读性差。
     问题位置：cmd/migra/diff.go:196
     建议：strings.HasSuffix(strings.ToLower(s), ".sql")。

  4. 架构与结构

  1. 渲染层未 schema-qualified，和“支持多 schema”目标冲突。
     问题位置：internal/render/render.go:92, internal/render/render.go:108
     建议：统一 schema.table 输出。

  // before
  ALTER TABLE "users" ADD COLUMN ...

  // after
  ALTER TABLE "public"."users" ADD COLUMN ...

  2. SQL 引号处理不安全（标识符双引号、字符串单引号都未 escape）。
     问题位置：internal/render/render.go:181, internal/render/render.go:186
     建议：实现安全转义：

  func quoteIdentifier(id string) string { return `"` + strings.ReplaceAll(id, `"`, `""`) + `"` }
  func quoteString(s string) string { return `'` + strings.ReplaceAll(s, `'`, `''`) + `'` }

  3. 输出顺序依赖 map 遍历，结果非确定性（CI/回归比对不稳定）。
     问题位置：internal/diff/differ.go:35, internal/diff/diff_tables.go:18
     建议：先收集 key 后排序，再遍历。
  4. Differ/Parser/Renderer 都是可变状态对象，不利于并发复用与测试隔离。
     建议：逐步改为“函数式无状态 API”（输入->输出），或明确声明“不可并发复用”。