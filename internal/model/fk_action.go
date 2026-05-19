package model

// FKActionCode maps PostgreSQL FK action codes to SQL keywords.
// Codes from pg_constraint.confupdtype/confdeltype and pg_query FK action codes:
//
//	"a" = NO ACTION, "r" = RESTRICT, "c" = CASCADE,
//	"n" = SET NULL, "d" = SET DEFAULT
//
// Both pg_query (parser) and pg_constraint (introspect) use the same single-character
// codes for ON DELETE/ON UPDATE actions, so this single implementation serves both.
func FKActionCode(code string) string {
	switch code {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return ""
	}
}
