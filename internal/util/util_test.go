package util

import "testing"

func TestIsBuiltinType(t *testing.T) {
	builtins := []string{
		"integer", "int", "int4", "int8", "int2", "bigint", "smallint",
		"serial", "bigserial", "smallserial",
		"boolean", "bool", "text", "varchar", "character varying",
		"char", "character", "numeric", "real", "float4",
		"double precision", "float8",
		"json", "jsonb", "uuid", "inet", "cidr", "macaddr",
		"interval", "date", "time", "timetz", "timestamp", "timestamptz",
		"bytea", "money", "oid", "void", "name",
		"varchar(255)", "numeric(10,2)", "char(10)",
		"integer[]", "text[]", "varchar(100)[]",
		"timestamp without time zone", "timestamp with time zone",
		"INTEGER", "VarChar(50)",
	}
	for _, dt := range builtins {
		if !IsBuiltinType(dt) {
			t.Errorf("IsBuiltinType(%q) = false, want true", dt)
		}
	}

	customs := []string{
		"mood", "posint", "auth.mood", "my_type",
	}
	for _, dt := range customs {
		if IsBuiltinType(dt) {
			t.Errorf("IsBuiltinType(%q) = true, want false", dt)
		}
	}
}
