package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var files embed.FS

func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock(762498103)`); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(762498103)`)
	var tracked bool
	if err = conn.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&tracked); err != nil {
		return err
	}
	if !tracked {
		var tables int
		if err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM pg_tables WHERE schemaname='public'`).Scan(&tables); err != nil {
			return err
		}
		if tables != 0 {
			return fmt.Errorf("automatic migrations require an empty database or an existing migration history")
		}
		if _, err = conn.Exec(ctx, `CREATE TABLE schema_migrations(name TEXT PRIMARY KEY,checksum TEXT NOT NULL,applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
			return err
		}
	}
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		data, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		checksum := hex.EncodeToString(hash[:])
		var previous string
		err = conn.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE name=$1`, name).Scan(&previous)
		if err == nil {
			if previous != checksum {
				return fmt.Errorf("migration %s changed after application", name)
			}
			continue
		}
		if err != pgx.ErrNoRows {
			return err
		}
		// A file that already BEGINs and COMMITs must run as that transaction.
		// Wrapping it again lets the inner COMMIT close the outer one before
		// schema_migrations is recorded. The insert goes in before COMMIT.
		if body, ok := scriptOwnsTransaction(string(data)); ok {
			sql := "BEGIN;\n" + body + "INSERT INTO schema_migrations(name,checksum) VALUES (" + quoteLiteral(name) + ", " + quoteLiteral(checksum) + ");\nCOMMIT;"
			if _, err = conn.Exec(ctx, sql); err != nil {
				// 'I' is idle. 'E' is a failed transaction left by the
				// script's BEGIN when a later statement raises.
				if conn.Conn().PgConn().TxStatus() != 'I' {
					_, _ = conn.Exec(ctx, `ROLLBACK`)
				}
				return fmt.Errorf("migration %s failed: %w", name, err)
			}
			continue
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(data)); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations(name,checksum) VALUES($1,$2)`, name, checksum)
		}
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s failed: %w", name, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

// scriptOwnsTransaction reports a script whose statements are one BEGIN and
// one COMMIT around the work. body is that work, each statement ending in
// a semicolon. psql -f autocommits every statement, so the guard and the
// ALTERs have to share this transaction or a failed guard still alters.
func scriptOwnsTransaction(script string) (body string, ok bool) {
	parts := splitSQLStatements(script)
	if len(parts) < 2 || sqlKeyword(parts[0]) != "BEGIN" || sqlKeyword(parts[len(parts)-1]) != "COMMIT" {
		return "", false
	}
	return strings.Join(parts[1:len(parts)-1], ";\n") + ";\n", true
}

func sqlKeyword(stmt string) string {
	fields := strings.Fields(stripSQLComments(stmt))
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func stripSQLComments(stmt string) string {
	var b strings.Builder
	inLine := false
	inBlock := false
	for i := 0; i < len(stmt); i++ {
		if inLine {
			if stmt[i] == '\n' {
				inLine = false
				b.WriteByte('\n')
			}
			continue
		}
		if inBlock {
			if stmt[i] == '*' && i+1 < len(stmt) && stmt[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if stmt[i] == '-' && i+1 < len(stmt) && stmt[i+1] == '-' {
			inLine = true
			i++
			continue
		}
		if stmt[i] == '/' && i+1 < len(stmt) && stmt[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		b.WriteByte(stmt[i])
	}
	return b.String()
}

// splitSQLStatements splits on semicolons outside quotes, comments, and
// dollar quotes.
func splitSQLStatements(script string) []string {
	var stmts []string
	var b strings.Builder
	dollar := ""
	inSingle := false
	inLineComment := false
	inBlockComment := false
	for i := 0; i < len(script); i++ {
		c := script[i]
		if inLineComment {
			b.WriteByte(c)
			if c == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			b.WriteByte(c)
			if c == '*' && i+1 < len(script) && script[i+1] == '/' {
				b.WriteByte('/')
				i++
				inBlockComment = false
			}
			continue
		}
		if dollar != "" {
			if strings.HasPrefix(script[i:], dollar) {
				b.WriteString(dollar)
				i += len(dollar) - 1
				dollar = ""
				continue
			}
			b.WriteByte(c)
			continue
		}
		if inSingle {
			b.WriteByte(c)
			if c == '\'' {
				if i+1 < len(script) && script[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if c == '-' && i+1 < len(script) && script[i+1] == '-' {
			b.WriteString("--")
			i++
			inLineComment = true
			continue
		}
		if c == '/' && i+1 < len(script) && script[i+1] == '*' {
			b.WriteString("/*")
			i++
			inBlockComment = true
			continue
		}
		if c == '\'' {
			b.WriteByte(c)
			inSingle = true
			continue
		}
		if c == '$' {
			if tag, ok := dollarTagAt(script, i); ok {
				dollar = tag
				b.WriteString(tag)
				i += len(tag) - 1
				continue
			}
		}
		if c == ';' {
			if stmt := strings.TrimSpace(b.String()); stmt != "" {
				stmts = append(stmts, stmt)
			}
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if stmt := strings.TrimSpace(b.String()); stmt != "" {
		stmts = append(stmts, stmt)
	}
	return stmts
}

func dollarTagAt(script string, i int) (string, bool) {
	if i >= len(script) || script[i] != '$' {
		return "", false
	}
	j := i + 1
	for j < len(script) {
		c := script[j]
		if c == '$' {
			return script[i : j+1], true
		}
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return "", false
		}
		j++
	}
	return "", false
}
